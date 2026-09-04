package downloader

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
	"torrentClient/internal/parser"
	"torrentClient/internal/peer"
	"torrentClient/internal/tracker"
)

type Downloader struct {
	torrent     parser.TorrentInfo
	peerId      [20]byte
	outputPath  []string
	totalLength uint64
	uploaded    uint64
	downloaded  uint64
	left        uint64
	Pieces      []bool
	Peers       []*net.TCPAddr
	mu          sync.Mutex

	trackerClient tracker.Tracker
}

func NewDownloader(torrentPath, outputPath []string) (*Downloader, error) {
	torrent, err := parser.ParseTorrent(torrentPath)
	if err != nil {
		return nil, fmt.Errorf("cannot parse torrent: %w", err)
	}

	peerId, err := tracker.GeneratePeerID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate peer Id: %w", err)
	}

	totalLength := torrent.Length
	if len(torrent.Files) > 0 {
		for _, file := range torrent.Files {
			err := ensureFile(outputPath, file.Path, int64(file.Length))
			if err != nil {
				return nil, fmt.Errorf("cannot truncate file: %s %w", file.Path, err)
			}
			totalLength += file.Length
		}
	}

	return &Downloader{
		torrent:     torrent,
		peerId:      peerId,
		outputPath:  outputPath,
		totalLength: totalLength,
		left:        totalLength,
	}, nil
}

func (d *Downloader) SendAnnounce() error {
	announceRequest := tracker.AnnounceRequest{
		InfoHash:   d.torrent.Hash,
		PeerID:     d.peerId,
		Port:       6861,
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

	any := false
Out:
	for _, innerList := range d.torrent.AnnounceList {
		for _, url := range innerList {
			fmt.Println("HERE:", url)
			trackerClient, err := tracker.New(url)
			if err != nil {
				fmt.Printf("cannot create tracker: %s, %v\n", url, err)
				continue
			}
			announceCtx, announceCancel = context.WithTimeout(context.Background(), 20*time.Second)

			announceResponse, err = trackerClient.Announce(announceCtx, announceRequest)
			if err != nil {
				announceCancel()
				fmt.Printf("could not send announce to %s, %v\n", url, err)
				continue
			}
			any = true
			break Out
		}
	}
	announceCancel()
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

func (d *Downloader) ConnectToPeer(peerClient *net.TCPAddr) error {
	peerCtx, peerCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer peerCancel()

	pieceCount := len(d.torrent.Pieces) / 20

	connectedPeer, err := peer.Dial(peerCtx, *peerClient, d.torrent.Hash, d.peerId, pieceCount)
	ip := peerClient.IP.String()
	if err != nil {
		return fmt.Errorf("cannot connect to peer: %s, %w", ip, err)
	}
	fmt.Printf("CONNECTED: %v\n", ip)

	for d.left > 0 {
		if err := connectedPeer.SendInterested(); err != nil {
			connectedPeer.Close()
			return fmt.Errorf("cannot send interested: %w", err)
		}

		if err := connectedPeer.WaitUnchoke(); err != nil {
			connectedPeer.Close()
			return fmt.Errorf("cannot wait for unchoke: %w", err)
		}

		fmt.Println("Peer unchocked us")

		pieceIndex := -1
		d.mu.Lock()
		for index := range pieceCount {
			if !d.Pieces[index] && connectedPeer.HasPiece(index) {
				pieceIndex = index
				d.Pieces[index] = true
				break
			}
		}
		d.mu.Unlock()

		if pieceIndex == -1 {
			connectedPeer.Close()
			return fmt.Errorf("peer does not have any piece")
		}

		fmt.Printf("peer has piece: %d\n", pieceIndex)

		pieceSize := d.torrent.PieceLength
		if pieceIndex == pieceCount-1 {
			pieceSize = int64(d.totalLength) - int64(pieceIndex)*d.torrent.PieceLength
		}

		buf := make([]byte, pieceSize)
		for begin := 0; begin < int(pieceSize); {
			if connectedPeer.IsChoked() {
				if err := connectedPeer.SendInterested(); err != nil {
					d.mu.Lock()
					d.Pieces[pieceIndex] = false
					d.mu.Unlock()

					connectedPeer.Close()
					return fmt.Errorf("cannot send interested: %w", err)
				}

				if err := connectedPeer.WaitUnchoke(); err != nil {
					d.mu.Lock()
					d.Pieces[pieceIndex] = false
					d.mu.Unlock()

					connectedPeer.Close()
					return fmt.Errorf("cannot wait for unchoke: %w", err)
				}
			}

			err := connectedPeer.SendRequest(uint32(pieceIndex), uint32(begin), uint32(min(16384, pieceSize-int64(begin))))
			if err != nil {
				d.mu.Lock()
				d.Pieces[pieceIndex] = false
				d.mu.Unlock()

				connectedPeer.Close()
				return fmt.Errorf("cannot send request: %w", err)
			}

			msg, err := connectedPeer.ReadPeerMessage()
			if err != nil {
				d.mu.Lock()
				d.Pieces[pieceIndex] = false
				d.mu.Unlock()

				connectedPeer.Close()
				return fmt.Errorf("cannot read peer message: %w", err)
			}
			if msg == nil {
				continue
			}

			if msg.ID == byte(peer.MsgPiece) {
				pi, off, blk, err := connectedPeer.ParsePieceMessage(msg)
				if err != nil {
					fmt.Printf("cannot parse peer message: %v\n", err)
					continue
				}

				fmt.Println("GOT", len(blk))

				if pi == uint32(pieceIndex) && int(off)+len(blk) <= len(buf) {
					copy(buf[off:], blk)
					begin += len(blk)
				}
			} else {
				err := connectedPeer.ApplyMessage(msg)
				if err != nil {
					fmt.Printf("cannot apply peer message: %v\n", err)
				}
			}
		}

		sum := sha1.Sum(buf)
		fmt.Println("ASDASD:", len(buf))

		actual := [20]byte(d.torrent.Pieces[pieceIndex*20 : (pieceIndex+1)*20])
		if sum != actual {
			d.mu.Lock()
			d.Pieces[pieceIndex] = false
			d.mu.Unlock()

			continue
		}

		fileExtends := d.torrent.MapRange(uint64(pieceIndex)*uint64(d.torrent.PieceLength), uint64(pieceSize))
		for _, ext := range fileExtends {
			path := filepath.Join(append(d.outputPath, ext.Path)...)
			f, err := os.OpenFile(path, os.O_WRONLY, 0644)
			if err != nil {
				panic(err)
			}

			n, err := f.WriteAt(buf[ext.BufOffset:ext.BufOffset+ext.Length], int64(ext.Offset))
			f.Close()
			if err != nil {
				panic(err)
			}
			fmt.Println("WROTE", ext.Path, ext.Offset)

			if uint64(n) != ext.Length {
				return fmt.Errorf("short write: got %d, want %d", n, ext.Length)
			}
		}

		size := uint64(len(buf))

		d.mu.Lock()
		d.downloaded += size
		d.left -= size
		d.mu.Unlock()
	}

	return nil
}

func ensureFile(outputPath, filePath []string, size int64) error {
	path := filepath.Join(append(outputPath, filePath...)...)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	_, err = os.Stat(path)
	if err == nil {
		return nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return f.Truncate(size)
}
