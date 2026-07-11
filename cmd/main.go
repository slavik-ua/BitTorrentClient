package main

import (
	parser "torrentClient/internal/parser"
)

func main() {
	value, err := parser.ParseFile("file.torrent")
	if err != nil {
		panic(err)
	}

	parser.ShowValue(value, 0)
}
