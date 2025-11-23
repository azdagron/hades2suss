package ioutils

import (
	"encoding/binary"
	"errors"
	"io"
)

// ReadString reads a length-prefixed string (1 byte length, then ASCII bytes)
func ReadString(r io.Reader) (string, error) {
	var length uint8
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}

	strBytes := make([]byte, length)
	if _, err := io.ReadFull(r, strBytes); err != nil {
		return "", err
	}

	return string(strBytes), nil
}

// WriteString writes a length-prefixed string (1 byte length, then ASCII bytes)
func WriteString(w io.Writer, s string) error {
	if len(s) > 255 {
		return errors.New("string too long (max 255 bytes)")
	}

	if err := binary.Write(w, binary.BigEndian, uint8(len(s))); err != nil {
		return err
	}

	_, err := w.Write([]byte(s))
	return err
}

// ReadInt32BE reads a 32-bit integer in big-endian format
func ReadInt32BE(r io.Reader) (int32, error) {
	var val int32
	err := binary.Read(r, binary.BigEndian, &val)
	return val, err
}

// WriteInt32BE writes a 32-bit integer in big-endian format
func WriteInt32BE(w io.Writer, val int32) error {
	return binary.Write(w, binary.BigEndian, val)
}

// ReadFloat32BE reads a 32-bit float in big-endian format
func ReadFloat32BE(r io.Reader) (float32, error) {
	var val float32
	err := binary.Read(r, binary.BigEndian, &val)
	return val, err
}

// WriteFloat32BE writes a 32-bit float in big-endian format
func WriteFloat32BE(w io.Writer, val float32) error {
	return binary.Write(w, binary.BigEndian, val)
}

// ReadInt32LE reads a 32-bit integer in little-endian format
func ReadInt32LE(r io.Reader) (int32, error) {
	var val int32
	err := binary.Read(r, binary.LittleEndian, &val)
	return val, err
}

// WriteInt32LE writes a 32-bit integer in little-endian format
func WriteInt32LE(w io.Writer, val int32) error {
	return binary.Write(w, binary.LittleEndian, val)
}

// ReadUint32LE reads a 32-bit unsigned integer in little-endian format
func ReadUint32LE(r io.Reader) (uint32, error) {
	var val uint32
	err := binary.Read(r, binary.LittleEndian, &val)
	return val, err
}

// WriteUint32LE writes a 32-bit unsigned integer in little-endian format
func WriteUint32LE(w io.Writer, val uint32) error {
	return binary.Write(w, binary.LittleEndian, val)
}

// Read7BitEncodedInt reads a 7-bit encoded integer (used in XNB version 5)
func Read7BitEncodedInt(r io.Reader) (int, error) {
	result := 0
	shift := 0

	for {
		var b uint8
		if err := binary.Read(r, binary.BigEndian, &b); err != nil {
			return 0, err
		}

		result |= int(b&0x7F) << shift

		if b&0x80 == 0 {
			break
		}

		shift += 7
		if shift >= 35 {
			return 0, errors.New("invalid 7-bit encoded int")
		}
	}

	return result, nil
}

// ReadByte reads a single byte
func ReadByte(r io.Reader) (byte, error) {
	var b byte
	err := binary.Read(r, binary.BigEndian, &b)
	return b, err
}

// WriteByte writes a single byte
func WriteByte(w io.Writer, b byte) error {
	return binary.Write(w, binary.BigEndian, b)
}
