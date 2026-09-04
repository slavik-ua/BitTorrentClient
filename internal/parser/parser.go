package parser

import (
	"crypto/sha1"
	"fmt"
	"os"
)

func ParseTorrent(path string) (TorrentInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TorrentInfo{}, err
	}

	// reader := bufio.NewReader(file)

	parser := NewBencodeParser(data)

	value, err := parser.ParseValue()
	if err != nil {
		return TorrentInfo{}, err
	}

	root, ok := value.(Dict)
	if !ok {
		return TorrentInfo{}, fmt.Errorf("torrent root must be a dictionary")
	}

	info, ok := root["info"].(Dict)
	if !ok {
		return TorrentInfo{}, fmt.Errorf("torrent does not contain an info dictionary")
	}

	var torrent TorrentInfo

	announce, ok := root["announce"].(String)
	if !ok {
		return TorrentInfo{}, fmt.Errorf("value dictionary has not annouce")
	}
	torrent.Announce = string(announce)

	if rawList, ok := root["announce-list"].(List); ok {
		announceList := make([][]string, 0, len(rawList))
		for _, list := range rawList {
			innerList, ok := list.(List)
			if !ok {
				return TorrentInfo{}, fmt.Errorf("outer list does not have inner list")
			}

			innerUrls := make([]string, 0, len(innerList))
			for _, val := range innerList {
				url, ok := val.(String)
				if !ok {
					return TorrentInfo{}, fmt.Errorf("inner list url is not a string")
				}
				innerUrls = append(innerUrls, string(url))
			}
			announceList = append(announceList, innerUrls)
		}
		torrent.AnnounceList = announceList
	} else {
		torrent.AnnounceList = [][]string{{torrent.Announce}}
	}

	name, ok := info["name"].(String)
	if !ok {
		return TorrentInfo{}, fmt.Errorf("info dictionary has no valid name")
	}
	torrent.Name = string(name)

	pieceLength, ok := info["piece length"].(Integer)
	if !ok {
		return TorrentInfo{}, fmt.Errorf("info dictionary has no valid piece length")
	}
	torrent.PieceLength = int64(pieceLength)

	pieces, ok := info["pieces"].(String)
	if !ok {
		return TorrentInfo{}, fmt.Errorf("info dictionary has no valid pieces")
	}
	torrent.Pieces = []byte(pieces)

	if length, ok := info["length"].(Integer); ok {
		torrent.Length = uint64(length)
	}

	if files, ok := info["files"].(List); ok {
		for _, item := range files {
			fileDict, ok := item.(Dict)
			if !ok {
				return TorrentInfo{}, fmt.Errorf("file dictionary has no valid pieces")
			}

			lengthValue, ok := fileDict["length"].(Integer)
			if !ok {
				return TorrentInfo{}, fmt.Errorf("file entry has no valid length")
			}

			pathValue, ok := fileDict["path"].(List)
			if !ok {
				return TorrentInfo{}, fmt.Errorf("file entry has no valid path")
			}

			filePath := make([]string, len(pathValue))
			for i, part := range pathValue {
				pathPart, ok := part.(String)
				if !ok {
					return TorrentInfo{}, fmt.Errorf("file path element %d must be a string", i)
				}
				filePath[i] = string(pathPart)
			}

			torrent.Files = append(torrent.Files, TorrentFile{
				Length: uint64(lengthValue),
				Path:   filePath,
			})
		}
	}

	infoRaw := parser.GetInfo()
	torrent.InfoRaw = infoRaw

	torrent.Hash = sha1.Sum(infoRaw)

	return torrent, nil
}
