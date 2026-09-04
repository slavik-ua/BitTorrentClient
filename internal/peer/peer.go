package peer

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type Bitfield []byte

func (b Bitfield) HasPiece(pieceIndex int) bool {
	byteIndex := pieceIndex / 8
	bitIndex := pieceIndex % 8

	if int(byteIndex) >= len(b) {
		return false
	}

	mask := byte(1 << (7 - bitIndex))
	return b[byteIndex]&mask != 0
}

func (b Bitfield) SetPiece(pieceIndex int) error {
	if pieceIndex < 0 {
		return fmt.Errorf("negative piece index: %d", pieceIndex)
	}

	byteIndex := pieceIndex / 8
	bitIndex := pieceIndex % 8

	if byteIndex >= len(b) {
		return fmt.Errorf("piece index %d outside bitfield", pieceIndex)
	}

	b[byteIndex] |= byte(1 << (7 - bitIndex))
	return nil
}

type Peer struct {
	Address    net.TCPAddr
	conn       net.Conn
	ID         [20]byte
	bitfield   Bitfield
	choked     bool
	interested bool
}

type PeerMessage struct {
	ID      byte
	Payload []byte
}

type MsgID byte

const (
	MsgChoke MsgID = iota
	MsgUnchoke
	MsgInterested
	MsgNotInterested
	MsgHave
	MsgBitfield
	MsgRequest
	MsgPiece
	MsgCancel
)

func Dial(ctx context.Context, address net.TCPAddr, infoHash [20]byte, localPeerID [20]byte, pieceCount int) (*Peer, error) {
	handshake := make([]byte, 68)

	handshake[0] = 19
	copy(handshake[1:20], "BitTorrent protocol")

	copy(handshake[28:48], infoHash[:])
	copy(handshake[48:68], localPeerID[:])

	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address.String())
	if err != nil {
		return nil, fmt.Errorf("connect to peer %s: %w", address.String(), err)
	}

	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, err
	}

	success := true
	defer func() {
		if !success {
			_ = conn.Close()
		}
	}()

	if _, err := conn.Write(handshake); err != nil {
		success = false
		return nil, err
	}

	response := make([]byte, 68)
	if _, err := io.ReadFull(conn, response); err != nil {
		return nil, err
	}

	if response[0] != 19 {
		return nil, fmt.Errorf("invalid protocol length: %d", response[0])
	}

	if string(response[1:20]) != "BitTorrent protocol" {
		return nil, fmt.Errorf("unexpected protocol: %q", response[1:20])
	}

	if !bytes.Equal(response[28:48], infoHash[:]) {
		return nil, fmt.Errorf("peer returned different info hash")
	}

	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("crear peer deadline: %w", err)
	}

	bitfieldLength := (pieceCount + 7) / 8

	return &Peer{
		Address:    address,
		conn:       conn,
		ID:         [20]byte(response[48:68]),
		bitfield:   make(Bitfield, bitfieldLength),
		choked:     true,
		interested: false,
	}, nil
}

func (p *Peer) ReadPeerMessage() (*PeerMessage, error) {
	var lengthBuf [4]byte

	if _, err := io.ReadFull(p.conn, lengthBuf[:]); err != nil {
		return nil, fmt.Errorf("read message length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])

	if length == 0 {
		return nil, nil
	}

	if length > 2*1024*1024 {
		return nil, fmt.Errorf("peer message too large: %d", length)
	}

	message := make([]byte, length)
	if _, err := io.ReadFull(p.conn, message); err != nil {
		return nil, fmt.Errorf("read peer message: %w", err)
	}

	return &PeerMessage{ID: message[0], Payload: message[1:]}, nil
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrUnexpectedEOF
		}

		data = data[n:]
	}

	return nil
}

func (p *Peer) SendInterested() error {
	message := []byte{
		0, 0, 0, 1,
		byte(MsgInterested),
	}

	if err := writeFull(p.conn, message); err != nil {
		return fmt.Errorf("send interested: %w", err)
	}
	p.interested = true
	return nil
}

func (p *Peer) WaitUnchoke() error {
	if err := p.conn.SetReadDeadline(time.Now().Add(20 * time.Second)); err != nil {
		return fmt.Errorf("set unchoke deadline: %w", err)
	}
	defer p.conn.SetReadDeadline(time.Time{})

	for p.choked {
		msg, err := p.ReadPeerMessage()
		if err != nil {
			return err
		}

		if msg == nil {
			continue
		}

		if err := p.ApplyMessage(msg); err != nil {
			return err
		}
	}

	return nil
}

func (p *Peer) HasPiece(pieceIndex int) bool {
	return p.bitfield.HasPiece(pieceIndex)
}

func (p *Peer) IsChoked() bool {
	return p.choked
}

func (p *Peer) ApplyMessage(msg *PeerMessage) error {
	switch MsgID(msg.ID) {
	case MsgChoke:
		p.choked = true
	case MsgUnchoke:
		p.choked = false
	case MsgBitfield:
		if len(msg.Payload) != len(p.bitfield) {
			return fmt.Errorf("invalid bitfield length: %d", len(msg.Payload))
		}
	case MsgHave:
		if len(msg.Payload) != 4 {
			return fmt.Errorf("invalid have payload length: %d", len(msg.Payload))
		}

		pieceIndex := binary.BigEndian.Uint32(msg.Payload)
		if err := p.bitfield.SetPiece(int(pieceIndex)); err != nil {
			return fmt.Errorf("process have message: %w", err)
		}
	}

	return nil
}

func (p *Peer) SendRequest(pieceIndex uint32, begin uint32, blockLength uint32) error {
	if blockLength == 0 || blockLength > 16*1024 {
		return fmt.Errorf("invalid block length: %d", blockLength)
	}

	message := make([]byte, 17)

	binary.BigEndian.PutUint32(message[0:4], 13)
	message[4] = byte(MsgRequest)
	binary.BigEndian.PutUint32(message[5:9], pieceIndex)
	binary.BigEndian.PutUint32(message[9:13], begin)
	binary.BigEndian.PutUint32(message[13:17], blockLength)

	if err := writeFull(p.conn, message); err != nil {
		return fmt.Errorf("send block request: %w", err)
	}
	return nil
}

func (p *Peer) ParsePieceMessage(msg *PeerMessage) (uint32, uint32, []byte, error) {
	if msg.ID != byte(MsgPiece) {
		return 0, 0, nil, fmt.Errorf("not a piece message")
	}

	if len(msg.Payload) < 8 {
		return 0, 0, nil, fmt.Errorf("piece payload too short")
	}

	pieceIndex := binary.BigEndian.Uint32(msg.Payload[0:4])
	begin := binary.BigEndian.Uint32(msg.Payload[4:8])
	block := msg.Payload[8:]

	return pieceIndex, begin, block, nil
}

func (p *Peer) Close() error {
	return p.conn.Close()
}
