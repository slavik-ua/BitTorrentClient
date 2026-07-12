package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Data struct {
	Dtype  string
	Number int
	Str    string
	List   []Data
	Dict   map[string]Data
}

func DecodeValue(reader *bufio.Reader) (Data, error) {
	char, err := reader.ReadByte()
	if err != nil {
		return Data{}, err
	}

	switch char {
	case 'i':
		n, err := ParseInteger(reader)
		if err != nil {
			return Data{}, err
		}
		return Data{
			Dtype:  "integer",
			Number: n,
		}, nil
	case 'l':
		list, err := ParseList(reader)
		if err != nil {
			return Data{}, err
		}
		return Data{
			Dtype: "list",
			List:  list,
		}, nil
	case 'd':
		dict, err := ParseDict(reader)
		if err != nil {
			return Data{}, err
		}
		return Data{
			Dtype: "dict",
			Dict:  dict,
		}, nil
	default:
		if char < '0' || char > '9' {
			return Data{}, fmt.Errorf("invalid value start: %q", char)
		}
		if err := reader.UnreadByte(); err != nil {
			return Data{}, err
		}

		str, err := ParseString(reader)
		if err != nil {
			return Data{}, err
		}
		return Data{
			Dtype: "string",
			Str:   str,
		}, nil
	}
}

func ParseInteger(reader *bufio.Reader) (int, error) {
	num := 0

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		if b == 'e' {
			break
		}

		if b < '0' || b > '9' {
			return 0, fmt.Errorf("invalid integer character: %q", b)
		}

		num = num*10 + int(b-'0')
	}

	return num, nil
}

func ParseLength(reader *bufio.Reader) (int, error) {
	length := 0

	for {
		b, err := reader.ReadByte()
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

func ParseString(reader *bufio.Reader) (string, error) {
	length, err := ParseLength(reader)
	if err != nil {
		return "", err
	}

	result := make([]byte, length)

	if _, err := io.ReadFull(reader, result); err != nil {
		return "", err
	}

	return string(result), nil
}

func ParseList(reader *bufio.Reader) ([]Data, error) {
	var result []Data

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == 'e' {
			return result, nil
		}

		if err := reader.UnreadByte(); err != nil {
			return nil, err
		}

		value, err := DecodeValue(reader)
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}
}

func ParseDict(reader *bufio.Reader) (map[string]Data, error) {
	result := make(map[string]Data)

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == 'e' {
			return result, nil
		}

		if b < '0' || b > '9' {
			return nil, fmt.Errorf("dictionary key must be a string, got %q", b)
		}

		if err := reader.UnreadByte(); err != nil {
			return nil, err
		}

		key, err := ParseString(reader)
		if err != nil {
			return nil, err
		}

		val, err := DecodeValue(reader)
		if err != nil {
			return nil, err
		}

		result[key] = val
	}
}

func ShowValue(value Data, indent int) {
	depth := strings.Repeat(" ", indent)

	switch value.Dtype {
	case "integer":
		fmt.Printf("%s%d\n", depth, value.Number)

	case "string":
		fmt.Printf("%s%q\n", depth, value.Str)

	case "list":
		fmt.Printf("%s[\n", depth)
		for _, val := range value.List {
			ShowValue(val, indent+4)
		}
		fmt.Printf("%s  ]\n", depth)

	case "dict":
		fmt.Printf("%s{\n", depth)
		for key, val := range value.Dict {
			fmt.Printf("%s  %s: ", depth, key)
			ShowValue(val, indent)
		}
		fmt.Printf("%s}\n", depth)
	}
}
