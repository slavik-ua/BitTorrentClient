package store

import (
	"os"
	"path/filepath"
	"testing"
	"torrentClient/internal/parser"

	"github.com/stretchr/testify/require"
)

func MultiFileTorrent() parser.TorrentInfo {
	return parser.TorrentInfo{
		Name:        "proj",
		PieceLength: 10,
		Files: []parser.TorrentFile{
			{Length: 12, Path: []string{"a.txt"}},
			{Length: 8, Path: []string{"b.txt"}},
		},
	}
}

func TestWritePiece_MultiFileBoundary(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, MultiFileTorrent())
	require.NoError(t, err)
	require.Equal(t, uint64(20), s.TotalLength())

	require.NoError(t, s.WritePiece(0, []byte("0123456789")))
	require.NoError(t, s.WritePiece(1, []byte("0123456789")))

	a, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "012345678901", string(a))

	b, err := os.ReadFile(filepath.Join(dir, "b.txt"))
	require.NoError(t, err)
	require.Equal(t, "23456789", string(b))
}

func TestWritePiece_SingleFile(t *testing.T) {
	dir := t.TempDir()
	torrent := parser.TorrentInfo{Name: "single.bin", PieceLength: 10, Length: 15}
	s, err := New(dir, torrent)
	require.NoError(t, err)
	require.Equal(t, uint64(15), s.TotalLength())

	require.NoError(t, s.WritePiece(0, []byte("0123456789")))
	require.NoError(t, s.WritePiece(1, []byte("01234")))

	raw, err := os.ReadFile(filepath.Join(dir, "single.bin"))
	require.NoError(t, err)
	require.Equal(t, "012345678901234", string(raw))
}

func TestWritePiece_PartialBufferWritesWhatItGets(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, MultiFileTorrent())
	require.NoError(t, err)

	require.NoError(t, s.WritePiece(1, []byte("ABC")))

	a, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "AB", string(a[10:12]))
}

func TestReadPiece_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, MultiFileTorrent())
	require.NoError(t, err)

	raw := []byte("ABC")
	require.NoError(t, s.WritePiece(0, raw))
	data, err := s.ReadPiece(0, int64(len(raw)))
	require.NoError(t, err)

	require.Equal(t, raw, data)
}
