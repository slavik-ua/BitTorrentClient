package tracker

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"net/url"
	"time"
)

type Client struct {
	conn       *net.UDPConn
	connID     uint64
	downloaded uint64
	left       uint64
	uploaded   uint64
}

func NewClient(serverAddress string) (*Client, error) {
	parsed, err := url.Parse(serverAddress)
	if err != nil {
		return nil, err
	}

	address, err := net.ResolveUDPAddr("udp4", parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("resolve UDP tracker: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, address)
	if err != nil {
		return nil, fmt.Errorf("dial UDP tracker: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))

	return &Client{
		conn:   conn,
		connID: 0,
	}, nil
}

func (c *Client) Connect() error {
	txnID := rand.Uint32()

	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[0:], 0x41727101980)
	binary.BigEndian.PutUint32(buf[8:], 0)
	binary.BigEndian.PutUint32(buf[12:], txnID)

	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set connect write deadline: %w", err)
	}

	fmt.Println("local address:", c.conn.LocalAddr())
	fmt.Println("remote address", c.conn.RemoteAddr())

	n, err := c.conn.Write(buf)
	if err != nil {
		return fmt.Errorf("send connect request: %w", err)
	}

	fmt.Printf("Wrote %d of %d bytes\n", n, len(buf))

	if err := c.conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
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

func (c *Client) SetLeft(left uint64) {
	c.left = left
}

func (c *Client) SendAnnounce(infoHash [20]byte) error {
	const announceRequestSize = 98

	peerID, err := GeneratePeerID()
	if err != nil {
		return err
	}

	var randomKey [4]byte
	if _, err := crand.Read(randomKey[:]); err != nil {
		return err
	}

	txnID := rand.Uint32()

	buf := make([]byte, announceRequestSize)
	binary.BigEndian.PutUint64(buf[0:8], c.connID)
	binary.BigEndian.PutUint32(buf[8:12], 1)
	binary.BigEndian.PutUint32(buf[12:16], txnID)
	copy(buf[16:36], infoHash[:])
	copy(buf[36:56], peerID[:])
	binary.BigEndian.PutUint64(buf[56:64], c.downloaded)
	binary.BigEndian.PutUint64(buf[64:72], c.left)
	binary.BigEndian.PutUint64(buf[72:80], c.uploaded)
	binary.BigEndian.PutUint32(buf[80:84], 2)
	binary.BigEndian.PutUint32(buf[84:88], 0)
	copy(buf[88:92], randomKey[:])
	binary.BigEndian.PutUint32(buf[92:96], ^uint32(0))

	port := uint16(6881)
	binary.BigEndian.PutUint16(buf[96:98], port)

	// move
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("set announce write deadline: %w", err)
	}

	if _, err = c.conn.Write(buf); err != nil {
		return err
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return fmt.Errorf("set announce read deadline: %w", err)
	}

	resp := make([]byte, 64*1024)

	n, err := c.conn.Read(resp)
	if err != nil {
		return err
	}

	response := resp[:n]
	if len(response) < 20 {
		return fmt.Errorf("announce response is too short: %d bytes", len(response))
	}

	action := binary.BigEndian.Uint32(response[0:4])
	receivedTxnID := binary.BigEndian.Uint32(response[4:8])

	if receivedTxnID != txnID {
		return fmt.Errorf("transaction ID is different: %d != %d", txnID, receivedTxnID)
	}

	if action == 3 {
		return fmt.Errorf("tracker announce error: %s", string(response[8:]))
	}

	if action != 1 {
		return fmt.Errorf("unexpected action: %d", action)
	}

	interval := binary.BigEndian.Uint32(response[8:12])
	leechers := binary.BigEndian.Uint32(response[12:16])
	seeders := binary.BigEndian.Uint32(response[16:20])

	fmt.Printf("Interval: %d\n", interval)
	fmt.Printf("Leechers: %d\n", leechers)
	fmt.Printf("Seeders: %d\n", seeders)

	peerData := response[20:]
	if len(peerData)%6 != 0 {
		return fmt.Errorf("invalid peer data length: %d", len(peerData))
	}

	for index := 0; index < len(peerData); index += 6 {
		ip := net.IP(peerData[index : index+4])
		port := binary.BigEndian.Uint16(peerData[index+4 : index+6])

		fmt.Printf("peer %s:%d\n", ip, port)
	}

	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
