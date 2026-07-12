package parser

import (
	"bufio"
	"fmt"
	"os"
)

type TorrentFile struct {
	Length int
	Path   []string
}

type TorrentInfo struct {
	Name        string
	PieceLength int
	Pieces      []byte
	Length      int
	Files       []TorrentFile
}

func ParseTorrent(path string) (TorrentInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return TorrentInfo{}, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	value, err := DecodeValue(reader)
	if err != nil {
		return TorrentInfo{}, err
	}

	if value.Dtype != "dict" {
		return TorrentInfo{}, fmt.Errorf("torrent must be a dictionary")
	}

	infoValue, ok := value.Dict["info"]
	if !ok || infoValue.Dtype != "dict" {
		return TorrentInfo{}, fmt.Errorf("torrent does not contain an info dictionary")
	}

	info := infoValue.Dict

	var torrent TorrentInfo

	name, ok := info["name"]
	if !ok || name.Dtype != "string" {
		return TorrentInfo{}, fmt.Errorf("info dictionary has no valid name")
	}
	torrent.Name = name.Str

	pieceLength, ok := info["piece length"]
	if !ok || pieceLength.Dtype != "integer" {
		return TorrentInfo{}, fmt.Errorf("info dictionary has no valid piece length")
	}
	torrent.PieceLength = pieceLength.Number

	pieces, ok := info["pieces"]
	if !ok || pieces.Dtype != "string" {
		return TorrentInfo{}, fmt.Errorf("info dictionary has not valid pieces")
	}
	torrent.Pieces = []byte(pieces.Str)

	if length, ok := info["length"]; ok {
		torrent.Length = length.Number
	}

	if files, ok := info["files"]; ok {
		for _, fileValue := range files.List {
			lengthValue, ok := fileValue.Dict["length"]
			if !ok {
				return TorrentInfo{}, fmt.Errorf("file entry has no length")
			}

			pathValue, ok := fileValue.Dict["path"]
			if !ok || pathValue.Dtype != "list" {
				return TorrentInfo{}, fmt.Errorf("file entry has no valid path")
			}

			filePath := make([]string, len(pathValue.List))
			for i, part := range pathValue.List {
				filePath[i] = part.Str
			}

			torrent.Files = append(torrent.Files, TorrentFile{
				Length: lengthValue.Number,
				Path:   filePath,
			})
		}
	}

	return torrent, nil
}
