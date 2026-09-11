package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"torrentClient/internal/downloader"
)

func main() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	if err := run(); err != nil {
		slog.Error("download failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	torrentPath := flag.String("torrent", "file.torrent", "path .torrent file")
	outDir := flag.String("out", "files", "output directory")
	port := flag.Uint("port", 6881, "listen port to advertise")
	flag.Parse()

	log := slog.Default()

	cfg := downloader.Config{
		ListenPort:      uint16(*port),
		NumWant:         4,
		BlockSize:       16384,
		AnnounceTimeout: 20 * time.Second,
		PeerTimeout:     30 * time.Second,
		Logger:          log,
	}
	d, err := downloader.NewDownloader(*torrentPath, *outDir, cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := d.SendAnnounce(ctx); err != nil {
		return err
	}

	for _, p := range d.Peers {
		log.Info("trying peer", "addr", p.String())
		if err := d.ConnectToPeer(ctx, p); err != nil {
			log.Warn("peer failed, trying next", "addr", p.String(), "err", err)
			continue
		}
		log.Info("peer succeeded", "addr", p.String())
		return nil
	}
	if len(d.Peers) == 0 {
		return fmt.Errorf("tracker returned no peers")
	}

	return fmt.Errorf("all peers failed")
}
