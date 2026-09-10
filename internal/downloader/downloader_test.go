package downloader

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"torrentClient/internal/parser"
	"torrentClient/internal/peer"
	"torrentClient/internal/store"

	"github.com/stretchr/testify/require"
)

type fakeConn struct {
	data    [][]byte
	corrupt map[int]int
	has     func(int) bool

	reqIdx   uint32
	reqBegin uint32
	reqLen   uint32
}

var _ peer.Conn = (*fakeConn)(nil)

func (f *fakeConn) SendInterested() error                  { return nil }
func (f *fakeConn) WaitUnchoke() error                     { return nil }
func (f *fakeConn) HasPiece(i int) bool                    { return f.has(i) }
func (f *fakeConn) IsChoked() bool                         { return false }
func (f *fakeConn) ApplyMessage(_ *peer.PeerMessage) error { return nil }
func (f *fakeConn) Close() error                           { return nil }

func (f *fakeConn) SendRequest(idx, begin, length uint32) error {
	f.reqIdx, f.reqBegin, f.reqLen = idx, begin, length
	return nil
}

func (f *fakeConn) ParsePieceMessage(m *peer.PeerMessage) (uint32, uint32, []byte, error) {
	return binary.BigEndian.Uint32(m.Payload[0:4]),
		binary.BigEndian.Uint32(m.Payload[4:8]),
		m.Payload[8:], nil
}

func (f *fakeConn) ReadPeerMessage() (*peer.PeerMessage, error) {
	block := f.data[f.reqIdx][f.reqBegin : f.reqBegin+f.reqLen]
	if n := f.corrupt[int(f.reqIdx)]; n > 0 {
		f.corrupt[int(f.reqIdx)] = n - 1
		bad := make([]byte, len(block))
		copy(bad, block)
		bad[0] ^= 0xff
		block = bad
	}
	raw := make([]byte, 8+len(block))
	binary.BigEndian.PutUint32(raw[0:4], f.reqIdx)
	binary.BigEndian.PutUint32(raw[4:8], f.reqBegin)
	copy(raw[8:], block)
	return &peer.PeerMessage{ID: byte(peer.MsgPiece), Payload: raw}, nil
}

func buildDownloader(t *testing.T, rng *rand.Rand, sizes []int) (*Downloader, [][]byte, string) {
	t.Helper()

	data := make([][]byte, len(sizes))
	pieces := make([]byte, 0, len(sizes)*20)
	var total uint64
	for i, n := range sizes {
		data[i] = make([]byte, n)
		_, err := rng.Read(data[i])
		require.NoError(t, err)
		h := sha1.Sum(data[i])
		pieces = append(pieces, h[:]...)
		total += uint64(n)
	}

	tr := parser.TorrentInfo{
		Name:        "f.bin",
		PieceLength: 10,
		Pieces:      pieces,
		Length:      total,
	}
	dir := t.TempDir()
	s, err := store.New(dir, tr)
	require.NoError(t, err)

	d := &Downloader{
		torrent:     tr,
		store:       s,
		totalLength: total,
		left:        total,
		Pieces:      make([]bool, len(sizes)),
	}
	return d, data, dir
}

func TestRun_DownloadsAllPieces(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	d, data, dir := buildDownloader(t, rng, []int{10, 10, 4})

	fake := &fakeConn{data: data, corrupt: map[int]int{}, has: func(int) bool { return true }}
	require.NoError(t, d.run(context.TODO(), fake, len(data)))

	require.Equal(t, uint64(0), d.left)
	require.Equal(t, uint64(24), d.downloaded)
	for i := range data {
		require.True(t, d.Pieces[i], "piece %d not marked done", i)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "f.bin"))
	require.NoError(t, err)
	require.Equal(t, bytes.Join(data, nil), raw)
}

func TestRun_RetriesCorruptPieces(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	d, data, _ := buildDownloader(t, rng, []int{10, 10, 4})

	fake := &fakeConn{
		data:    data,
		corrupt: map[int]int{0: 2, 2: 1},
		has:     func(int) bool { return true },
	}
	require.NoError(t, d.run(context.TODO(), fake, len(data)))

	require.Equal(t, uint64(0), d.left)
	require.Equal(t, 0, fake.corrupt[0], "corruption budget not consumed")
	require.Equal(t, 0, fake.corrupt[2], "corruption budget not consumed")
}

func TestRun_PeerWithoutPiecesErrors(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	d, data, _ := buildDownloader(t, rng, []int{10, 10, 4})

	fake := &fakeConn{data: data, has: func(int) bool { return false }}
	require.Error(t, d.run(context.TODO(), fake, len(data)))
}

func TestRun_CanceledContextStops(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	d, data, _ := buildDownloader(t, rng, []int{10, 10, 4})
	fake := &fakeConn{data: data, has: func(i int) bool { return false }}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.run(ctx, fake, len(data))
	require.ErrorIs(t, err, context.Canceled)
}
