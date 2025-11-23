package pkg

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/pierrec/lz4/v4"
)

// ChunkProcessor handles chunk compression and decompression
type ChunkProcessor interface {
	// Compress compresses a chunk of data
	Compress(data []byte) ([]byte, error)

	// Decompress decompresses a chunk of data
	Decompress(compressedData []byte, expectedSize int) ([]byte, error)

	// WriteChunk writes a chunk to the writer (with compression header)
	WriteChunk(w io.Writer, data []byte) error

	// ReadChunk reads a chunk from the reader (with compression header)
	ReadChunk(r io.Reader) ([]byte, error)
}

// UncompressedProcessor handles uncompressed chunks
type UncompressedProcessor struct{}

func (p *UncompressedProcessor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (p *UncompressedProcessor) Decompress(compressedData []byte, expectedSize int) ([]byte, error) {
	// Pad with zeros if needed
	if len(compressedData) < expectedSize {
		result := make([]byte, expectedSize)
		copy(result, compressedData)
		return result, nil
	}
	return compressedData, nil
}

func (p *UncompressedProcessor) WriteChunk(w io.Writer, data []byte) error {
	// Write uncompressed flag (0x00)
	if err := binary.Write(w, binary.BigEndian, byte(0x00)); err != nil {
		return err
	}

	// Write data directly
	_, err := w.Write(data)
	return err
}

func (p *UncompressedProcessor) ReadChunk(r io.Reader) ([]byte, error) {
	// The compression flag has already been read, just read the data
	data := make([]byte, ChunkSize)
	n, err := io.ReadFull(r, data)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return data[:n], nil
}

// LZ4Processor handles LZ4 compressed chunks
type LZ4Processor struct{}

func (p *LZ4Processor) Compress(data []byte) ([]byte, error) {
	// Use raw block compression (not framed)
	compressed := make([]byte, lz4.CompressBlockBound(len(data)))
	n, err := lz4.CompressBlock(data, compressed, nil)
	if err != nil {
		return nil, err
	}
	return compressed[:n], nil
}

func (p *LZ4Processor) Decompress(compressedData []byte, expectedSize int) ([]byte, error) {
	// Use raw block decompression (not framed)
	result := make([]byte, expectedSize)
	n, err := lz4.UncompressBlock(compressedData, result)
	if err != nil {
		return nil, err
	}

	// Pad with zeros if needed
	if n < expectedSize {
		for i := n; i < expectedSize; i++ {
			result[i] = 0
		}
	}

	return result, nil
}

func (p *LZ4Processor) WriteChunk(w io.Writer, data []byte) error {
	// Compress the data
	compressed, err := p.Compress(data)
	if err != nil {
		return err
	}

	// Write compressed flag (0x01)
	if err := binary.Write(w, binary.BigEndian, byte(0x01)); err != nil {
		return err
	}

	// Write compressed size (4 bytes, big-endian)
	if err := binary.Write(w, binary.BigEndian, int32(len(compressed))); err != nil {
		return err
	}

	// Write compressed data
	_, err = w.Write(compressed)
	return err
}

func (p *LZ4Processor) ReadChunk(r io.Reader) ([]byte, error) {
	// The compression flag has already been read
	// Read compressed size
	var compSize int32
	if err := binary.Read(r, binary.BigEndian, &compSize); err != nil {
		return nil, err
	}

	if compSize <= 0 {
		return nil, errors.New("invalid compressed size")
	}

	// Read compressed data
	compData := make([]byte, compSize)
	if _, err := io.ReadFull(r, compData); err != nil {
		return nil, err
	}

	// Decompress
	return p.Decompress(compData, ChunkSize)
}

// NewProcessor creates a chunk processor based on compression type
func NewProcessor(compressionType byte) (ChunkProcessor, error) {
	switch compressionType {
	case CompressionUncompressed:
		return &UncompressedProcessor{}, nil
	case CompressionLZ4:
		return &LZ4Processor{}, nil
	case CompressionLZF:
		return nil, errors.New("LZF compression not implemented")
	case CompressionLZX:
		return nil, errors.New("LZX compression not implemented")
	default:
		return nil, errors.New("unknown compression type")
	}
}
