package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"torrentClient/internal/downloader"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	torrentPath := flag.String("torrent", "file.torrent", "path .torrent file")
	outDir := flag.String("out", "files", "output directory")
	port := flag.Uint("port", 6881, "listen port to advertise")
	flag.Parse()

	cfg := downloader.Config{
		ListenPort:      uint16(*port),
		NumWant:         4,
		BlockSize:       16384,
		AnnounceTimeout: 20 * time.Second,
		PeerTimeout:     30 * time.Second,
	}
	d, err := downloader.NewDownloader(*torrentPath, *outDir, cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = d.SendAnnounce(ctx)
	if err != nil {
		return err
	}

	for _, p := range d.Peers {
		fmt.Printf("Trying peer %s ...\n", p.String())
		err = d.ConnectToPeer(ctx, p)
		if err != nil {
			fmt.Printf("peer %s failed: %v (trying next)\n", p.String(), err)
			continue
		}
		fmt.Printf("Peer %s succeeded\n", p.String())
		break
	}
	if err != nil {
		return fmt.Errorf("all peers failed: %w", err)
	}

	fmt.Println("DONE")

	return nil
}
