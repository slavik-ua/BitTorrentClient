package tracker

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"net/url"
	"time"
)

type UDPTracker struct {
	Address *net.UDPAddr
	conn    *net.UDPConn
	connID  uint64
}

func NewUDPTracker(url *url.URL) (*UDPTracker, error) {
	address, err := net.ResolveUDPAddr("udp", url.Host)
	if err != nil {
		return nil, fmt.Errorf("resolve UDP tracker: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, address)
	if err != nil {
		return nil, fmt.Errorf("dial UDP tracker: %w", err)
	}

	// conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	return &UDPTracker{
		Address: address,
		conn:    conn,
		connID:  0,
	}, nil
}

func (c *UDPTracker) Announce(ctx context.Context, request AnnounceRequest) (AnnounceResponse, error) {
	const announceRequestSize = 98

	err := c.connect(ctx)
	if err != nil {
		return AnnounceResponse{}, fmt.Errorf("Could not connect: %w", err)
	}

	var keyBuf [4]byte
	if _, err := crand.Read(keyBuf[:]); err != nil {
		return AnnounceResponse{}, err
	}
	randomKey := binary.BigEndian.Uint32(keyBuf[:])
	// randomKey := rand.Uint32()

	txnID := rand.Uint32()

	buf := make([]byte, announceRequestSize)
	binary.BigEndian.PutUint64(buf[0:8], c.connID)
	binary.BigEndian.PutUint32(buf[8:12], 1)
	binary.BigEndian.PutUint32(buf[12:16], txnID)
	copy(buf[16:36], request.InfoHash[:])
	copy(buf[36:56], request.PeerID[:])
	binary.BigEndian.PutUint64(buf[56:64], request.Downloaded)
	binary.BigEndian.PutUint64(buf[64:72], request.Left)
	binary.BigEndian.PutUint64(buf[72:80], request.Uploaded)
	binary.BigEndian.PutUint32(buf[80:84], uint32(request.Event))
	binary.BigEndian.PutUint32(buf[84:88], 0)
	binary.BigEndian.PutUint32(buf[88:92], randomKey)
	binary.BigEndian.PutUint32(buf[92:96], request.NumWant)
	binary.BigEndian.PutUint16(buf[96:98], request.Port)

	// move
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return AnnounceResponse{}, fmt.Errorf("set announce write deadline: %w", err)
	}

	if _, err := c.conn.Write(buf); err != nil {
		return AnnounceResponse{}, err
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return AnnounceResponse{}, fmt.Errorf("set announce read deadline: %w", err)
	}

	resp := make([]byte, 64*1024)

	n, err := c.conn.Read(resp)
	if err != nil {
		return AnnounceResponse{}, err
	}

	response := resp[:n]
	if len(response) < 20 {
		return AnnounceResponse{}, fmt.Errorf("announce response is too short: %d bytes", len(response))
	}

	action := binary.BigEndian.Uint32(response[0:4])
	receivedTxnID := binary.BigEndian.Uint32(response[4:8])

	if receivedTxnID != txnID {
		return AnnounceResponse{}, fmt.Errorf("transaction ID is different: %d != %d", txnID, receivedTxnID)
	}

	if action == 3 {
		return AnnounceResponse{}, fmt.Errorf("tracker announce error: %s", string(response[8:]))
	}

	if action != 1 {
		return AnnounceResponse{}, fmt.Errorf("unexpected action: %d", action)
	}

	interval := binary.BigEndian.Uint32(response[8:12])
	seeders := binary.BigEndian.Uint32(response[12:16])
	leechers := binary.BigEndian.Uint32(response[16:20])

	fmt.Printf("Interval: %d\n", interval)
	fmt.Printf("Leechers: %d\n", leechers)
	fmt.Printf("Seeders: %d\n", seeders)

	peerData := response[20:]
	if len(peerData)%6 != 0 {
		return AnnounceResponse{}, fmt.Errorf("invalid peer data length: %d", len(peerData))
	}

	peers := make([]*net.TCPAddr, 0, len(peerData)/6)
	for index := 0; index < len(peerData); index += 6 {
		ip := net.IP(peerData[index : index+4])
		port := binary.BigEndian.Uint16(peerData[index+4 : index+6])

		peers = append(peers, &net.TCPAddr{
			IP:   ip,
			Port: int(port),
		})
	}

	return AnnounceResponse{
		Interval: time.Duration(interval) * time.Second,
		Peers:    peers,
		Seeders:  int(seeders),
		Leechers: int(leechers),
	}, nil
}

func (c *UDPTracker) connect(ctx context.Context) error {
	txnID := rand.Uint32()

	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[0:], 0x41727101980)
	binary.BigEndian.PutUint32(buf[8:], 0)
	binary.BigEndian.PutUint32(buf[12:], txnID)

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set connect write deadline: %w", err)
	}

	fmt.Println("local address:", c.conn.LocalAddr())
	fmt.Println("remote address", c.conn.RemoteAddr())

	n, err := c.conn.Write(buf)
	if err != nil {
		return fmt.Errorf("send connect request: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	resp := make([]byte, 2048)
	n, err = c.conn.Read(resp)
	if err != nil {
		return err
	}

	response := resp[:n]
	if len(response) < 8 {
		return fmt.Errorf("connection response too short: %d bytes", len(response))
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	respTxnID := binary.BigEndian.Uint32(resp[4:8])

	if txnID != respTxnID {
		return fmt.Errorf("txnID does not match. %d != %d", txnID, respTxnID)
	}

	if action == 3 {
		return fmt.Errorf("tracker connect error: %s", string(response[:8]))
	}

	if action != 0 {
		return fmt.Errorf("action must be 0, got %d", action)
	}

	if len(response) < 16 {
		return fmt.Errorf("connect response too short: %d bytes", len(response))
	}

	c.connID = binary.BigEndian.Uint64(resp[8:16])

	return nil
}

func (c *UDPTracker) Close() error {
	return c.conn.Close()
}
