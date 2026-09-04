package main

import (
	"fmt"
	"torrentClient/internal/downloader"
)

func main() {
	downloader, err := downloader.NewDownloader([]string{"movie.torrent"}, []string{"files"})
	if err != nil {
		panic(err)
	}
	err = downloader.SendAnnounce()
	if err != nil {
		panic(err)
	}

	for _, p := range downloader.Peers {
		fmt.Printf("Trying peer %s ...\n", p.String())
		err = downloader.ConnectToPeer(p)
		if err != nil {
			fmt.Printf("peer %s failed: %v (trying next)\n", p.String(), err)
			continue
		}
		fmt.Printf("Peer %s succeeded\n", p.String())
		break
	}
	if err != nil {
		panic(fmt.Errorf("all peers failed: %w", err))
	}

	fmt.Println("DONE")
	// 	torrent, err := parser.ParseTorrent("local.torrent")
	// 	if err != nil {
	// 		panic(fmt.Errorf("parse torrent: %w", err))
	// 	}

	// 	peerID, err := tracker.GeneratePeerID()
	// 	if err != nil {
	// 		panic(fmt.Errorf("generate peerID: %w", err))
	// 	}

	// 	outputPath := []string{"files"}

	// 	totalLength := torrent.Length
	// 	if len(torrent.Files) > 0 {
	// 		for _, file := range torrent.Files {
	// 			err := ensureFile(outputPath, file.Path, int64(file.Length))
	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 			totalLength += file.Length
	// 		}
	// 	}

	// 	announceRequest := tracker.AnnounceRequest{
	// 		InfoHash:   torrent.Hash,
	// 		PeerID:     peerID,
	// 		Port:       6881,
	// 		Uploaded:   0,
	// 		Downloaded: 0,
	// 		Left:       totalLength,
	// 		Event:      tracker.EventStarted,
	// 		NumWant:    4,
	// 	}

	// 	var trackerClient tracker.Tracker
	// 	var announceResponse tracker.AnnounceResponse
	// 	var announceCtx context.Context
	// 	var announceCancel context.CancelFunc
	// 	any := false
	// Out:
	// 	for _, innerList := range torrent.AnnounceList {
	// 		for _, url := range innerList {
	// 			trackerClient, err = tracker.New(url)
	// 			if err != nil {
	// 				fmt.Printf("cannot create tracker: %s, %v", url, err)
	// 				continue
	// 			}
	// 			announceCtx, announceCancel = context.WithTimeout(context.Background(), 20*time.Second)

	// 			announceResponse, err = trackerClient.Announce(
	// 				announceCtx,
	// 				announceRequest)
	// 			if err != nil {
	// 				announceCancel()
	// 				fmt.Printf("could not send announce to %s, %v", url, err)
	// 				continue
	// 			}
	// 			any = true
	// 			break Out
	// 		}
	// 	}
	// 	announceCancel()
	// 	if !any {
	// 		panic(fmt.Errorf("announce to tracker: %w", err))
	// 	}

	// 	if len(announceResponse.Peers) == 0 {
	// 		panic("tracker returned no peers")
	// 	}

	// 	pieceCount := len(torrent.Pieces) / 20
	// 	if pieceCount == 0 {
	// 		panic("torrent contains no piece hashes")
	// 	}

	// 	peerCtx, peerCancel := context.WithTimeout(context.Background(), 30*time.Second)
	// 	defer peerCancel()

	// 	for _, peerClient := range announceResponse.Peers {
	// 		connectedPeer, err := peer.Dial(peerCtx, *peerClient, torrent.Hash, peerID, pieceCount)
	// 		if err != nil {
	// 			fmt.Printf("cannot connect to peer, %v, %v\n", peerID[:], err)
	// 			continue
	// 		}
	// 		fmt.Printf("CONNECTED: %v\n", peerID)

	// 		if err := connectedPeer.SendInterested(); err != nil {
	// 			fmt.Printf("send interested: %v\n", err)
	// 			connectedPeer.Close()
	// 			continue
	// 		}

	// 		if err := connectedPeer.WaitUnchoke(); err != nil {
	// 			fmt.Printf("wait for unchoke: %v\n", err)
	// 			connectedPeer.Close()
	// 			continue
	// 		}

	// 		fmt.Println("Peer unchoked us")

	// 		pieceIndex := -1

	// 		for index := range pieceCount {
	// 			if connectedPeer.HasPiece(index) {
	// 				pieceIndex = index
	// 				break
	// 			}
	// 		}

	// 		if pieceIndex == -1 {
	// 			panic("peer does not have any piece")
	// 		}

	// 		fmt.Printf("peer has piece: %d\n", pieceIndex)

	// 		pieceSize := torrent.PieceLength
	// 		if pieceIndex == pieceCount-1 {
	// 			pieceSize = int64(totalLength) - int64(pieceIndex)*torrent.PieceLength
	// 		}

	// 		buf := make([]byte, pieceSize)
	// 		for begin := 0; begin < int(pieceSize); {
	// 			err := connectedPeer.SendRequest(uint32(pieceIndex), uint32(begin), uint32(min(16384, pieceSize-int64(begin))))
	// 			if err != nil {
	// 				fmt.Printf("cannot send request: %v", err)
	// 				break
	// 			}

	// 			msg, err := connectedPeer.ReadPeerMessage()
	// 			if err != nil {
	// 				fmt.Printf("cannot read peer message: %v", err)
	// 				break
	// 			}
	// 			if msg == nil {
	// 				continue
	// 			}

	// 			if msg.ID == byte(peer.MsgPiece) {
	// 				pi, off, blk, err := connectedPeer.ParsePieceMessage(msg)
	// 				if err != nil {
	// 					fmt.Printf("cannot parse peer message: %v", err)
	// 					continue
	// 				}

	// 				fmt.Println("GOT", len(blk))

	// 				if pi == uint32(pieceIndex) && int(off)+len(blk) <= len(buf) {
	// 					copy(buf[off:], blk)
	// 					begin += len(blk)
	// 				}
	// 			} else {
	// 				err := connectedPeer.ApplyMessage(msg)
	// 				if err != nil {
	// 					fmt.Printf("cannot apply peer message: %v", err)
	// 				}
	// 			}
	// 		}
	// 		sum := sha1.Sum(buf)
	// 		fmt.Println("ASDASD:", len(buf))
	// 		actual := [20]byte(torrent.Pieces[pieceIndex*20 : (pieceIndex+1)*20])
	// 		if sum != actual {
	// 			panic("sum does not match")
	// 		}

	// 		globalOff := pieceIndex * int(torrent.PieceLength)
	// 		fileExtends := torrent.MapRange(uint64(pieceIndex)*uint64(torrent.PieceLength), uint64(pieceSize))
	// 		for _, ext := range fileExtends {
	// 			path := filepath.Join(append(outputPath, ext.Path)...)
	// 			f, err := os.OpenFile(path, os.O_WRONLY, 0644)
	// 			if err != nil {
	// 				panic(err)
	// 			}

	// 			relOff := ext.Offset - uint64(globalOff)

	// 			_, err = f.WriteAt(buf, int64(relOff))
	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 			fmt.Println("WROTE", ext.Path, ext.Offset)
	// 			f.Close()
	// 		}

	//		connectedPeer.Close()
	//	}
	//
	// selectedPeer := announceResponse.Peers[0]
}
