package tracker

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"torrentClient/internal/parser"
)

type HTTPTracker struct {
	URL    *url.URL
	Client http.Client
}

func NewHTTPTracker(url *url.URL) *HTTPTracker {
	client := http.Client{
		Timeout: 20 * time.Second,
	}

	return &HTTPTracker{
		URL:    url,
		Client: client,
	}
}

func ParseCompactPeers(data []byte) ([]*net.TCPAddr, error) {
	if len(data)%6 != 0 {
		return nil, fmt.Errorf("invalid compact peers length: %d", len(data))
	}

	peers := make([]*net.TCPAddr, 0, len(data)/6)

	for index := 0; index < len(data); index += 6 {
		ip := net.IPv4(data[index], data[index+1], data[index+2], data[index+3])
		port := binary.BigEndian.Uint16(data[index+4 : index+6])

		peers = append(peers, &net.TCPAddr{IP: ip, Port: int(port)})
	}

	return peers, nil
}

// Move struct to another package
func (h *HTTPTracker) buildAnnounceURL(request AnnounceRequest) *url.URL {
	trackerURL := *h.URL

	query := trackerURL.Query()
	query.Set("port", strconv.FormatUint(uint64(request.Port), 10))
	query.Set("uploaded", strconv.FormatUint(request.Uploaded, 10))
	query.Set("downloaded", strconv.FormatUint(request.Downloaded, 10))
	query.Set("left", strconv.FormatUint(request.Left, 10))
	query.Set("compact", "1")

	if request.NumWant != 0 {
		query.Set("numwant", strconv.Itoa(int(request.NumWant)))
	}

	if event := request.Event.HTTPValue(); event != "" {
		query.Set("event", event)
	}

	normalQuery := query.Encode()

	binaryQuery := fmt.Sprintf(
		"info_hash=%s&peer_id=%s",
		escapeBinary(request.InfoHash[:]),
		escapeBinary(request.PeerID[:]),
	)

	if normalQuery == "" {
		trackerURL.RawQuery = binaryQuery
	} else {
		trackerURL.RawQuery = normalQuery + "&" + binaryQuery
	}

	return &trackerURL
}

func (h *HTTPTracker) Announce(ctx context.Context, request AnnounceRequest) (AnnounceResponse, error) {
	announceURL := h.buildAnnounceURL(request)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, announceURL.String(), nil)
	if err != nil {
		return AnnounceResponse{}, fmt.Errorf("create tracker request: %w", err)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return AnnounceResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AnnounceResponse{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AnnounceResponse{}, fmt.Errorf("tracker returned %d", resp.StatusCode)
	}

	bencodeParser := parser.NewBencodeParser(body)
	value, err := bencodeParser.ParseValue()
	if err != nil {
		return AnnounceResponse{}, fmt.Errorf("parser error: %w", err)
	}
	root := value.(parser.Dict)

	if reason, ok := root["failure reason"].(parser.String); ok {
		return AnnounceResponse{}, fmt.Errorf("tracker failure: %s", string(reason))
	}

	// Interval
	duration, ok := root["interval"].(parser.Integer)
	if !ok {
		return AnnounceResponse{}, fmt.Errorf("cannot convert duration integer")
	}

	// Peers
	// peersString, ok := root["peers"].(parser.String)
	// if !ok {
	// 	return AnnounceResponse{}, fmt.Errorf("cannot convert peers string")
	// }

	peersVal, ok := root["peers"]
	if !ok {
		return AnnounceResponse{}, fmt.Errorf("peers do not exist")
	}

	var peers []*net.TCPAddr

	switch v := peersVal.(type) {
	case parser.String:
		peers, err = ParseCompactPeers([]byte(v))
		if err != nil {
			return AnnounceResponse{}, fmt.Errorf("peer parse error: %w", err)
		}
	case parser.List:
		for _, item := range v {
			item, ok := item.(parser.Dict)
			if !ok {
				return AnnounceResponse{}, fmt.Errorf("cannot convert to dict: %v", item)
			}
			ip, ok := item["ip"].(parser.String)
			if !ok {
				return AnnounceResponse{}, fmt.Errorf("cannot convert ip to string: %v", item["ip"])
			}
			port, ok := item["port"].(parser.Integer)
			if !ok {
				return AnnounceResponse{}, fmt.Errorf("cannot convert port to integer: %v", item["port"])
			}
			if port < 0 || port > 65535 {
				return AnnounceResponse{}, fmt.Errorf("port is not in between 0 and 65535: %v", port)
			}
			peers = append(peers, &net.TCPAddr{IP: net.ParseIP(string(ip)), Port: int(port)})
		}
	default:
		return AnnounceResponse{}, fmt.Errorf("cannot convert peersVal: %v", peersVal)
	}

	// Seeders
	seeders := 0
	if value, ok := root["complete"].(parser.Integer); ok {
		seeders = int(value)
	}

	// Leechers
	leechers := 0
	if value, ok := root["incomplete"].(parser.Integer); ok {
		leechers = int(value)
	}

	return AnnounceResponse{
		Interval: time.Duration(duration) * time.Second,
		Peers:    peers,
		Seeders:  seeders,
		Leechers: leechers,
	}, nil
}
