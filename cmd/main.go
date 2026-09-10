package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"torrentClient/internal/downloader"
)

func main() {
	downloader, err := downloader.NewDownloader("file.torrent", "files")
	if err != nil {
		panic(err)
	}
	err = downloader.SendAnnounce()
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for _, p := range downloader.Peers {
		fmt.Printf("Trying peer %s ...\n", p.String())
		err = downloader.ConnectToPeer(ctx, p)
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
}
