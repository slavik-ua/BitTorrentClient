package parser

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
