package tracker

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	parser "torrentClient/internal/parser"
)

// Move struct to another package
func BuildURL(info parser.TorrentInfo) (string, error) {
	peerID, err := GeneratePeerID()
	if err != nil {
		return "", err
	}

	left := info.Length
	if len(info.Files) > 0 {
		left = 0
		for _, file := range info.Files {
			left += file.Length
		}
	}

	trackerURL, err := url.Parse(info.Announce)
	if err != nil {
		return "", fmt.Errorf("parse tracker URL: %w", err)
	}

	query := trackerURL.Query()

	query.Set("port", "6881")
	query.Set("uploaded", "0")
	query.Set("downloaded", "0")
	query.Set("left", strconv.FormatInt(left, 10))
	query.Set("compact", "1")

	normalQuery := query.Encode()

	binaryQuery := fmt.Sprintf(
		"info_hash=%s&peer_id=%s",
		escapeBinary(info.Hash[:]),
		escapeBinary(peerID[:]),
	)

	if normalQuery == "" {
		trackerURL.RawQuery = binaryQuery
	} else {
		trackerURL.RawQuery = normalQuery + "&" + binaryQuery
	}

	return trackerURL.String(), nil
}

func SendRequest(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tracker returned %d", resp.StatusCode)
	}

	return body, nil
}

func GeneratePeerID() ([20]byte, error) {
	var peerID [20]byte

	copy(peerID[:], "-GT001-")
	if _, err := rand.Read(peerID[8:]); err != nil {
		return [20]byte{}, fmt.Errorf("generate peerID %w", err)
	}

	return peerID, nil
}

func escapeBinary(data []byte) string {
	var b strings.Builder

	for _, v := range data {
		fmt.Fprintf(&b, "%%%02X", v)
	}

	return b.String()
}
