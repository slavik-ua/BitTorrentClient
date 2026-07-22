package parser

type TorrentFile struct {
	Length int64
	Path   []string
}

type TorrentInfo struct {
	Announce    string
	Name        string
	PieceLength int64
	Pieces      []byte
	Length      int64
	Files       []TorrentFile
	InfoRaw     []byte
	Hash        [20]byte
}
