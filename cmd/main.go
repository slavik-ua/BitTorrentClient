package main

import (
	"fmt"
	parser "torrentClient/internal/parser"
)

func main() {
	value, err := parser.ParseTorrent("file.torrent")
	if err != nil {
		panic(err)
	}

	// parser.ShowValue(value, 0)
	fmt.Println(value.Files[0].Path)
}
