package parser

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTorrent_MultiFile(t *testing.T) {
	content := "d" +
		"8:announce31:http://tracker.example/announce" +
		"4:infod5:filesl" +
		"d6:lengthi100e4:pathl4:docs10:README.txtee" +
		"d6:lengthi200e4:pathl3:src7:main.goee" +
		"e" +
		"4:name7:project" +
		"12:piece lengthi16384e" +
		"6:pieces20:12345678901234567890" +
		"ee"

	file, err := os.CreateTemp(t.TempDir(), "*.torrent")
	require.NoError(t, err)

	_, err = file.WriteString(content)
	require.NoError(t, err)

	require.NoError(t, file.Close())

	actual, err := ParseTorrent(file.Name())
	require.NoError(t, err)

	expected := TorrentInfo{
		Announce:    "http://tracker.example/announce",
		Name:        "project",
		PieceLength: 16384,
		Pieces:      []byte("12345678901234567890"),
		Files: []TorrentFile{
			{
				Length: 100,
				Path:   []string{"docs", "README.txt"},
			},
			{
				Length: 200,
				Path:   []string{"src", "main.go"},
			},
		},
	}

	require.Equal(t, expected, actual)
}
