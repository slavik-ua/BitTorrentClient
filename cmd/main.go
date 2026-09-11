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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	d, err := downloader.NewDownloader(*torrentPath, *outDir, cfg)
	if err != nil {
		return err
	}

	d.Resume(ctx)
	if d.IsComplete() {
		log.Info("already completed, nothing to download")
		return nil
	}

	if err := d.SendAnnounce(ctx); err != nil {
		return err
	}

	start := time.Now()
	tickDone := make(chan struct{})
	defer close(tickDone)

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tickDone:
				return
			case <-t.C:
				p := d.Progress()
				elapsed := time.Since(start).Seconds()
				rate := 0.0
				if elapsed > 0 {
					rate = float64(p.Downloaded) / elapsed
				}
				eta := "?"
				if rate > 0 && p.Left > 0 {
					eta = (time.Duration(float64(p.Left)/rate) * time.Second).Round(time.Second).String()
				}
				pct := 0.0
				if p.Total > 0 {
					pct = 100 * float64(p.Downloaded) / float64(p.Total)
				}
				log.Info("progress",
					"pieces", fmt.Sprintf("%d/%d", p.DonePieces, p.TotalPieces),
					"done", fmt.Sprintf("%.1f%%", pct),
					"rate", fmt.Sprintf("%.1f MB/s", rate/1e6),
					"eta", eta,
				)
			}
		}
	}()

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
