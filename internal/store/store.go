package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"torrentClient/internal/parser"
)

type Store struct {
	dir         string
	torrent     parser.TorrentInfo
	totalLength uint64
}

func ensureFile(outputFile, filePath string, size int64) error {
	path := filepath.Join(outputFile, filePath)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	_, err = os.Stat(path)
	if err == nil {
		return nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return f.Truncate(size)
}

func New(dir string, torrent parser.TorrentInfo) (*Store, error) {
	totalLength := torrent.Length
	if len(torrent.Files) > 0 {
		for _, file := range torrent.Files {
			path := filepath.Join(file.Path...)
			err := ensureFile(dir, path, int64(file.Length))
			if err != nil {
				return nil, fmt.Errorf("cannot truncate file: %s %w", file.Path, err)
			}
			totalLength += file.Length
		}
	} else {
		if err := ensureFile(dir, torrent.Name, int64(totalLength)); err != nil {
			return nil, fmt.Errorf("cannot truncate file: %s %w", torrent.Name, err)
		}
	}

	return &Store{
		dir:         dir,
		torrent:     torrent,
		totalLength: totalLength,
	}, nil
}

func (s *Store) TotalLength() uint64 {
	return s.totalLength
}

func (s *Store) WritePiece(pieceIndex int, buf []byte) error {
	globalOff := uint64(pieceIndex) * uint64(s.torrent.PieceLength)

	fileExtends := s.torrent.MapRange(globalOff, uint64(len(buf)))
	for _, ext := range fileExtends {
		if ext.BufOffset+ext.Length > uint64(len(buf)) {
			return fmt.Errorf("wrong maprange: %v > %v", ext.BufOffset+ext.Length, len(buf))
		}

		path := filepath.Join(s.dir, ext.Path)
		f, err := os.OpenFile(path, os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open output file: %s %w", path, err)
		}

		n, err := f.WriteAt(buf[ext.BufOffset:ext.BufOffset+ext.Length], int64(ext.Offset))
		cerr := f.Close()
		if err != nil {
			return fmt.Errorf("write output file: %s %w", path, err)
		}
		if cerr != nil {
			return fmt.Errorf("close file: %s %w", path, cerr)
		}

		if uint64(n) != ext.Length {
			return fmt.Errorf("short write: got %d, want %d", uint64(n), ext.Length)
		}
	}

	return nil
}

func (s *Store) ReadPiece(pieceIndex int, size int64) ([]byte, error) {
	buf := make([]byte, size)
	globalOff := uint64(pieceIndex) * uint64(s.torrent.PieceLength)

	for _, ext := range s.torrent.MapRange(globalOff, uint64(size)) {
		path := filepath.Join(s.dir, ext.Path)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open output file: %s %w", path, err)
		}
		n, rerr := f.ReadAt(buf[ext.BufOffset:ext.BufOffset+ext.Length], int64(ext.Offset))
		cerr := f.Close()
		if rerr != nil && !errors.Is(rerr, io.EOF) {
			return nil, fmt.Errorf("read output file: %s %w", path, rerr)
		}
		if uint64(n) != ext.Length {
			return nil, fmt.Errorf("short read: got %d, want %d", uint64(n), ext.Length)
		}
		if cerr != nil {
			return nil, fmt.Errorf("close file: %s %w", path, cerr)
		}
	}

	return buf, nil
}
