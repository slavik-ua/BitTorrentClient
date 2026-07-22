package main

import (
	"context"
	"fmt"
	"time"
	"torrentClient/internal/parser"
	"torrentClient/internal/peer"
	"torrentClient/internal/tracker"
)

func main() {
	torrent, err := parser.ParseTorrent("movie.torrent")
	if err != nil {
		panic(fmt.Errorf("parse torrent: %w", err))
	}

	peerID, err := tracker.GeneratePeerID()
	if err != nil {
		panic(fmt.Errorf("generate peerID: %w", err))
	}

	totalLength := torrent.Length
	if len(torrent.Files) > 0 {
		for _, file := range torrent.Files {
			totalLength += file.Length
		}
	}

	trackerClient, err := tracker.New(torrent.Announce)
	if err != nil {
		panic(fmt.Errorf("create tracker client: %w", err))
	}

	announceCtx, announceCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer announceCancel()

	announceResponse, err := trackerClient.Announce(
		announceCtx,
		tracker.AnnounceRequest{
			InfoHash:   torrent.Hash,
			PeerID:     peerID,
			Port:       6881,
			Uploaded:   0,
			Downloaded: 0,
			Left:       totalLength,
			Event:      tracker.EventStarted,
			NumWant:    50,
		})
	if err != nil {
		panic(fmt.Errorf("announce to tracker: %w", err))
	}

	if len(announceResponse.Peers) == 0 {
		panic("tracker returned no peers")
	}

	pieceCount := len(torrent.Pieces) / 20
	if pieceCount == 0 {
		panic("torrent contains no piece hashes")
	}

	peerCtx, peerCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer peerCancel()

	selectedPeer := announceResponse.Peers[0]

	connectedPeer, err := peer.Dial(peerCtx, *selectedPeer, torrent.Hash, peerID, pieceCount)
	if err != nil {
		panic(err)
	}
	defer connectedPeer.Close()

	if err := connectedPeer.SendInterested(); err != nil {
		panic(fmt.Errorf("send interested: %w", err))
	}

	if err := connectedPeer.WaitUnchoke(); err != nil {
		panic(fmt.Errorf("wait for unchoke: %w", err))
	}

	fmt.Println("Peer unchoked us")

	pieceIndex := -1

	for index := 0; index < pieceCount; index++ {
		if connectedPeer.HasPiece(index) {
			pieceIndex = index
			break
		}
	}

	if pieceIndex == -1 {
		panic("peer does not have any piece")
	}

	fmt.Printf("peer has piece: %d\n", pieceIndex)
}
