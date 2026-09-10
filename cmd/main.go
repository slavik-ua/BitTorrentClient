package main

import (
	"fmt"
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
}
