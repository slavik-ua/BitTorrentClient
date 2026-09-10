package parser

import (
	"path/filepath"
	"sort"
)

type TorrentFile struct {
	Length uint64
	Path   []string
}

type TorrentInfo struct {
	Announce     string
	AnnounceList [][]string
	Name         string
	PieceLength  int64
	Pieces       []byte
	Length       uint64
	Files        []TorrentFile
	InfoRaw      []byte
	Hash         [20]byte
}

type FileExtent struct {
	Path      string
	Offset    uint64
	Length    uint64
	BufOffset uint64
}

func (t *TorrentInfo) MapRange(globalOff, size uint64) []FileExtent {
	if len(t.Files) == 0 {
		if size == 0 || globalOff+size > t.Length {
			return nil
		}
		return []FileExtent{
			{
				Path:      t.Name,
				Offset:    globalOff,
				Length:    size,
				BufOffset: 0,
			},
		}
	}

	sizes := make([]uint64, len(t.Files)+1)
	for i := range len(t.Files) {
		sizes[i+1] = sizes[i] + t.Files[i].Length
	}

	files := make([]FileExtent, 0, 2)

	cur := globalOff
	end := globalOff + size
	for cur < end {
		i := sort.Search(len(sizes), func(i int) bool {
			return sizes[i] > cur
		})

		fileStart := sizes[i-1]
		fileEnd := sizes[i]

		length := min(fileEnd-cur, end-cur)

		filePath := filepath.Join(t.Files[i-1].Path...)

		files = append(files, FileExtent{
			Path:      filePath,
			Offset:    cur - fileStart,
			Length:    length,
			BufOffset: cur - globalOff,
		})

		cur += length
	}

	return files
}
