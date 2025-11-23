package pkg

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	// Chunk size is 32MB (0x2000000 bytes)
	ChunkSize = 0x2000000

	// Compression type codes
	CompressionUncompressed = 0x00
	CompressionLZ4          = 0x20 // Hades/Hades 2
	CompressionLZF          = 0x40 // Transistor/Pyre
	CompressionLZX          = 0x60 // Not implemented

	// Package versions
	VersionTransistor = 5 // Transistor/Pyre
	VersionHades      = 7 // Hades/Hades 2

	// Entry type codes
	EntryTypeTexture2D  = 0xAD
	EntryTypeAtlas      = 0xDE
	EntryTypeTexture3D  = 0xAA
	EntryTypeBink       = 0xBB
	EntryTypeBinkAtlas  = 0xEE
	EntryTypeInclude    = 0xCC
	EntryTypeSpine      = 0xF0
	EntryTypeEndChunk   = 0xBE
	EntryTypeEndFile    = 0xFF
)

// Header represents the 4-byte package header
type Header struct {
	CompressionType byte
	Reserved1       byte
	Reserved2       byte
	Version         byte
}

// ReadHeader reads a package header from the reader
func ReadHeader(r io.Reader) (*Header, error) {
	h := &Header{}
	if err := binary.Read(r, binary.BigEndian, h); err != nil {
		return nil, err
	}

	// Validate header
	if h.Reserved1 != 0 || h.Reserved2 != 0 {
		return nil, errors.New("invalid header: reserved bytes must be zero")
	}

	if h.Version != VersionTransistor && h.Version != VersionHades {
		return nil, errors.New("unsupported package version")
	}

	return h, nil
}

// WriteHeader writes a package header to the writer
func WriteHeader(w io.Writer, h *Header) error {
	return binary.Write(w, binary.BigEndian, h)
}

// NewHeader creates a new package header
func NewHeader(compressionType, version byte) *Header {
	return &Header{
		CompressionType: compressionType,
		Reserved1:       0,
		Reserved2:       0,
		Version:         version,
	}
}
