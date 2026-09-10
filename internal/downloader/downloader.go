package downloader

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"torrentClient/internal/parser"
	"torrentClient/internal/peer"
	"torrentClient/internal/store"
	"torrentClient/internal/tracker"
)

type Downloader struct {
	torrent     parser.TorrentInfo
	peerId      [20]byte
	store       *store.Store
	totalLength uint64
	uploaded    uint64
	downloaded  uint64
	left        uint64
	Pieces      []bool
	Peers       []*net.TCPAddr
	mu          sync.Mutex

	trackerClient tracker.Tracker
}

func NewDownloader(torrentPath, outputPath string) (*Downloader, error) {
	torrent, err := parser.ParseTorrent(torrentPath)
	if err != nil {
		return nil, fmt.Errorf("cannot parse torrent: %w", err)
	}

	peerId, err := tracker.GeneratePeerID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate peer Id: %w", err)
	}

	store, err := store.New(outputPath, torrent)
	if err != nil {
		return nil, fmt.Errorf("cannot create store: %w", err)
	}

	return &Downloader{
		torrent:     torrent,
		peerId:      peerId,
		totalLength: store.TotalLength(),
		left:        store.TotalLength(),
		store:       store,
	}, nil
}

func (d *Downloader) SendAnnounce() error {
	announceRequest := tracker.AnnounceRequest{
		InfoHash:   d.torrent.Hash,
		PeerID:     d.peerId,
		Port:       6881,
		Uploaded:   0,
		Downloaded: 0,
		Left:       d.totalLength,
		Event:      tracker.EventStarted,
		NumWant:    4,
	}

	var trackerClient tracker.Tracker
	var announceResponse tracker.AnnounceResponse
	var announceCtx context.Context
	var announceCancel context.CancelFunc
	var err error

	any := false
Out:
	for _, innerList := range d.torrent.AnnounceList {
		for _, url := range innerList {
			fmt.Println("HERE:", url)
			trackerClient, err = tracker.New(url)
			if err != nil {
				fmt.Printf("cannot create tracker: %s, %v\n", url, err)
				continue
			}
			announceCtx, announceCancel = context.WithTimeout(context.Background(), 20*time.Second)

			announceResponse, err = trackerClient.Announce(announceCtx, announceRequest)
			announceCancel()
			if closer, ok := trackerClient.(io.Closer); ok {
				closer.Close()
			}

			if err != nil {
				fmt.Printf("could not send announce to %s, %v\n", url, err)
				continue
			}

			any = true
			break Out
		}
	}

	if !any {
		return fmt.Errorf("cannot send announce to tracker")
	}

	if len(announceResponse.Peers) == 0 {
		return fmt.Errorf("tracker returned no piece hashes")
	}

	pieceCount := len(d.torrent.Pieces) / 20
	if pieceCount == 0 {
		return fmt.Errorf("torrent contains no piece hashes")
	}

	d.Pieces = make([]bool, pieceCount)
	d.trackerClient = trackerClient
	d.Peers = announceResponse.Peers

	return nil
}

func (d *Downloader) unmark(pieceIndex int) {
	d.mu.Lock()
	d.Pieces[pieceIndex] = false
	d.mu.Unlock()
}

func (d *Downloader) pickPiece(hasPiece func(int) bool) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	for index, done := range d.Pieces {
		if !done && hasPiece(index) {
			d.Pieces[index] = true
			return index
		}
	}

	return -1
}

func (d *Downloader) pieceSize(pieceIndex, pieceCount int) int64 {
	if pieceIndex == pieceCount-1 {
		return int64(d.totalLength) - int64(pieceIndex)*d.torrent.PieceLength
	}
	return d.torrent.PieceLength
}

func (d *Downloader) fetchPiece(p peer.Conn, pieceIndex int, size int64) ([]byte, error) {
	buf := make([]byte, size)
	for begin := 0; begin < int(size); {
		if p.IsChoked() {
			if err := p.SendInterested(); err != nil {
				return nil, fmt.Errorf("cannot send interested: %w", err)
			}

			if err := p.WaitUnchoke(); err != nil {
				return nil, fmt.Errorf("cannot wait for unchoke: %w", err)
			}
		}

		err := p.SendRequest(uint32(pieceIndex), uint32(begin), uint32(min(16384, size-int64(begin))))
		if err != nil {
			return nil, fmt.Errorf("cannot send request: %w", err)
		}

		msg, err := p.ReadPeerMessage()
		if err != nil {
			return nil, fmt.Errorf("cannot read peer message: %w", err)
		}
		if msg == nil {
			continue
		}

		if msg.ID == byte(peer.MsgPiece) {
			pi, off, blk, err := p.ParsePieceMessage(msg)
			if err != nil {
				continue
			}

			if pi == uint32(pieceIndex) && int(off)+len(blk) <= len(buf) {
				copy(buf[off:], blk)
				begin += len(blk)
			}
		} else {
			_ = p.ApplyMessage(msg)
		}
	}

	return buf, nil
}

var ErrPieceMismatch = errors.New("piece hash mismatch")

func (d *Downloader) verifyAndStore(pieceIndex int, buf []byte) error {
	sum := sha1.Sum(buf)
	actual := [20]byte(d.torrent.Pieces[pieceIndex*20 : (pieceIndex+1)*20])
	if sum != actual {
		d.unmark(pieceIndex)
		return ErrPieceMismatch
	}

	if err := d.store.WritePiece(pieceIndex, buf); err != nil {
		return fmt.Errorf("cannot write piece: %w", err)
	}

	size := uint64(len(buf))

	d.mu.Lock()
	d.downloaded += size
	d.left -= size
	d.mu.Unlock()

	return nil
}

func (d *Downloader) run(conn peer.Conn, pieceCount int) error {
	for d.left > 0 {
		if err := conn.SendInterested(); err != nil {
			return fmt.Errorf("cannot send interested: %w", err)
		}

		if err := conn.WaitUnchoke(); err != nil {
			return fmt.Errorf("cannot wait for unchoke: %w", err)
		}

		fmt.Println("Peer unchocked us")

		pieceIndex := d.pickPiece(conn.HasPiece)
		if pieceIndex == -1 {
			return fmt.Errorf("peer does not have any piece")
		}

		fmt.Printf("peer has piece: %d\n", pieceIndex)

		pieceSize := d.pieceSize(pieceIndex, pieceCount)

		buf, err := d.fetchPiece(conn, pieceIndex, pieceSize)
		if err != nil {
			d.unmark(pieceIndex)
			return fmt.Errorf("cannot fetch piece: %w", err)
		}

		if err := d.verifyAndStore(pieceIndex, buf); err != nil {
			if errors.Is(err, ErrPieceMismatch) {
				continue
			}
			return err
		}
	}

	return nil
}

func (d *Downloader) ConnectToPeer(peerClient *net.TCPAddr) error {
	peerCtx, peerCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer peerCancel()

	pieceCount := len(d.torrent.Pieces) / 20

	connectedPeer, err := peer.Dial(peerCtx, *peerClient, d.torrent.Hash, d.peerId, pieceCount)
	ip := peerClient.IP.String()
	if err != nil {
		return fmt.Errorf("cannot connect to peer: %s, %w", ip, err)
	}
	defer connectedPeer.Close()

	fmt.Printf("CONNECTED: %v\n", ip)

	return d.run(connectedPeer, pieceCount)
}
