package pkg

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// Reader provides streaming access to package data with automatic chunk decompression
type Reader struct {
	file      *os.File
	processor ChunkProcessor
	header    *Header

	// Current chunk state
	currentChunk []byte
	chunkPos     int
	chunkNum     int
	eof          bool
}

// NewReader creates a new package reader
func NewReader(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Read header
	header, err := ReadHeader(file)
	if err != nil {
		file.Close()
		return nil, err
	}

	// Create processor
	processor, err := NewProcessor(header.CompressionType)
	if err != nil {
		file.Close()
		return nil, err
	}

	r := &Reader{
		file:      file,
		processor: processor,
		header:    header,
		chunkNum:  -1, // Will be incremented to 0 on first read
	}

	return r, nil
}

// Read implements io.Reader interface
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.eof {
		return 0, io.EOF
	}

	bytesRead := 0

	for bytesRead < len(p) {
		// Load next chunk if needed
		if r.currentChunk == nil || r.chunkPos >= len(r.currentChunk) {
			if err := r.loadNextChunk(); err != nil {
				if err == io.EOF {
					r.eof = true
					if bytesRead > 0 {
						return bytesRead, nil
					}
					return 0, io.EOF
				}
				return bytesRead, err
			}
		}

		// Copy from current chunk
		copied := copy(p[bytesRead:], r.currentChunk[r.chunkPos:])
		r.chunkPos += copied
		bytesRead += copied
	}

	return bytesRead, nil
}

// loadNextChunk loads the next chunk from the file
func (r *Reader) loadNextChunk() error {
	var chunk []byte

	// For uncompressed packages, there are no per-chunk compression flags
	if r.header.CompressionType == CompressionUncompressed {
		// Read all remaining data as one chunk
		chunk, err := io.ReadAll(r.file)
		if err != nil {
			return err
		}
		if len(chunk) == 0 {
			return io.EOF
		}
		r.currentChunk = chunk
		r.chunkPos = 0
		r.chunkNum++
		return nil
	}

	// For compressed packages, read compression flag for each chunk
	var compressed byte
	if err := binary.Read(r.file, binary.BigEndian, &compressed); err != nil {
		return err
	}

	// Check for end markers
	if compressed == EntryTypeEndChunk {
		// End of chunk marker, read next chunk
		r.chunkNum++
		return r.loadNextChunk()
	}

	if compressed == EntryTypeEndFile {
		// End of file marker
		return io.EOF
	}

	if compressed == 0x00 {
		// Uncompressed chunk (shouldn't happen if header says compressed)
		// Adjust size for first chunk (header takes 4 bytes)
		chunkSize := ChunkSize
		if r.chunkNum == -1 {
			chunkSize = ChunkSize - 4
		}

		chunk = make([]byte, chunkSize)
		n, readErr := io.ReadFull(r.file, chunk)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return readErr
		}
		chunk = chunk[:n]
	} else if compressed == 0x01 {
		// Compressed chunk
		var compSize int32
		if err := binary.Read(r.file, binary.BigEndian, &compSize); err != nil {
			return err
		}

		if compSize <= 0 {
			return errors.New("invalid compressed size")
		}

		compData := make([]byte, compSize)
		if _, err := io.ReadFull(r.file, compData); err != nil {
			return err
		}

		// Decompress
		expectedSize := ChunkSize
		if r.chunkNum == -1 {
			expectedSize = ChunkSize - 4
		}

		var err error
		chunk, err = r.processor.Decompress(compData, expectedSize)
		if err != nil {
			return err
		}
	} else {
		// Put the byte back for entry reading
		r.currentChunk = []byte{compressed}
		r.chunkPos = 0
		r.chunkNum++
		return nil
	}

	r.currentChunk = chunk
	r.chunkPos = 0
	r.chunkNum++

	return nil
}

// Header returns the package header
func (r *Reader) Header() *Header {
	return r.header
}

// Close closes the package reader
func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// ReadByte reads a single byte (implements io.ByteReader)
func (r *Reader) ReadByte() (byte, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	return b[0], err
}
