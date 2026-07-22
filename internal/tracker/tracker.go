package tracker

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

func GeneratePeerID() ([20]byte, error) {
	var peerID [20]byte

	copy(peerID[:], "-GT0001-")
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

type Event uint32

const (
	EventNone Event = iota
	EventCompleted
	EventStarted
	EventStopped
)

func (e Event) HTTPValue() string {
	switch e {
	case EventStarted:
		return "started"
	case EventCompleted:
		return "completed"
	case EventStopped:
		return "stopped"
	case EventNone:
		return ""
	default:
		return ""
	}
}

type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerID     [20]byte
	Port       uint16
	Uploaded   int64
	Downloaded int64
	Left       int64
	Event      Event
	NumWant    int
}

type AnnounceResponse struct {
	Interval   time.Duration
	Peers      []*net.TCPAddr
	Complete   int
	Incomplete int
}

type Tracker interface {
	Announce(ctx context.Context, request AnnounceRequest) (AnnounceResponse, error)
}

func New(rawURL string) (Tracker, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	switch parsed.Scheme {
	case "http", "https":
		return NewHTTPTracker(parsed), nil
	case "udp":
		udpTracker, err := NewUDPTracker(parsed)
		if err != nil {
			return nil, err
		}
		return udpTracker, nil
	default:
		return nil, fmt.Errorf("unsupported tracker scheme: %q", parsed.Scheme)
	}
}
