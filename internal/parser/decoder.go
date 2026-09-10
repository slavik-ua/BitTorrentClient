package parser

import (
	"fmt"
	"io"
	"strings"
)

type Value interface {
	isBencodeValue()
}

type Integer int

func (Integer) isBencodeValue() {}

type String string

func (String) isBencodeValue() {}

type List []Value

func (List) isBencodeValue() {}

type Dict map[string]Value

func (Dict) isBencodeValue() {}

type BencodeParser struct {
	data      []byte
	pos       int
	infoStart int
	infoEnd   int
}

func NewBencodeParser(data []byte) *BencodeParser {
	return &BencodeParser{
		data: data,
	}
}

func (p *BencodeParser) readByte() (byte, error) {
	if p.pos > len(p.data) {
		return 0, io.ErrUnexpectedEOF
	}
	b := p.data[p.pos]
	p.pos++
	return b, nil
}

func (p *BencodeParser) ParseValue() (Value, error) {
	b, err := p.readByte()
	if err != nil {
		return nil, err
	}

	switch b {
	case 'i':
		n, err := p.ParseInteger()
		return Integer(n), err
	case 'l':
		return p.ParseList()
	case 'd':
		return p.ParseDict()
	default:
		if b < '0' || b > '9' {
			return nil, fmt.Errorf("invalid value start: %q", b)
		}

		p.pos--
		str, err := p.ParseString()
		return String(str), err
	}
}

func (p *BencodeParser) ParseInteger() (int, error) {
	num := 0
	negative := false

	for {
		b, err := p.readByte()
		if err != nil {
			return 0, err
		}

		if b == 'e' {
			break
		}
		if b == '-' {
			negative = true
			continue
		}

		if b < '0' || b > '9' {
			return 0, fmt.Errorf("invalid integer character: %q", b)
		}

		num = num*10 + int(b-'0')
	}

	if negative {
		return -num, nil
	}

	return num, nil
}

func (p *BencodeParser) ParseLength() (int, error) {
	length := 0

	for {
		b, err := p.readByte()
		if err != nil {
			return 0, err
		}

		if b == ':' {
			return length, nil
		}

		if b < '0' || b > '9' {
			return 0, fmt.Errorf("invalid integer character: %q", b)
		}

		length = length*10 + int(b-'0')
	}
}

func (p *BencodeParser) ParseString() (string, error) {
	length, err := p.ParseLength()
	if err != nil {
		return "", err
	}

	if length < 0 || p.pos+length > len(p.data) {
		return "", io.ErrUnexpectedEOF
	}

	result := make([]byte, length)
	for i := range length {
		b, err := p.readByte()
		if err != nil {
			return "", err
		}
		result[i] = b
	}

	return string(result), nil
}

func (p *BencodeParser) ParseList() (List, error) {
	var result List

	for {
		b, err := p.readByte()
		if err != nil {
			return nil, err
		}

		if b == 'e' {
			return result, nil
		}

		p.pos--
		value, err := p.ParseValue()
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}
}

func (p *BencodeParser) ParseDict() (Dict, error) {
	result := make(Dict)

	for {
		b, err := p.readByte()
		if err != nil {
			return nil, err
		}

		if b == 'e' {
			return result, nil
		}

		p.pos--
		if b < '0' || b > '9' {
			return nil, fmt.Errorf("dictionary key must be a string, got %q", b)
		}

		key, err := p.ParseString()
		if err != nil {
			return nil, err
		}

		if key == "info" {
			p.infoStart = p.pos
		}

		val, err := p.ParseValue()
		if err != nil {
			return nil, err
		}

		if key == "info" {
			p.infoEnd = p.pos
		}

		result[key] = val
	}
}

func (p *BencodeParser) GetInfo() []byte {
	return p.data[p.infoStart:p.infoEnd]
}

func ShowValue(value Value, indent int) {
	depth := strings.Repeat(" ", indent)

	switch v := value.(type) {
	case Integer:
		fmt.Printf("%s%d\n", depth, v)

	case String:
		fmt.Printf("%s%q\n", depth, v)

	case List:
		fmt.Printf("%s[\n", depth)
		for _, val := range v {
			ShowValue(val, indent+4)
		}
		fmt.Printf("%s  ]\n", depth)

	case Dict:
		fmt.Printf("%s{\n", depth)
		for key, val := range v {
			fmt.Printf("%s  %s: ", depth, key)
			ShowValue(val, indent)
		}
		fmt.Printf("%s}\n", depth)
	}
}
