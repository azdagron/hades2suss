package pkg

import (
	"bytes"
	"errors"
	"io"
	"os"
)

// Writer provides buffered writing to package files with automatic chunk compression
type Writer struct {
	file      *os.File
	processor ChunkProcessor
	header    *Header

	// Write buffer for current chunk
	buffer   *bytes.Buffer
	chunkNum int
}

// NewWriter creates a new package writer
func NewWriter(path string, compressionType, version byte) (*Writer, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	// Create header
	header := NewHeader(compressionType, version)

	// Write header
	if err := WriteHeader(file, header); err != nil {
		file.Close()
		return nil, err
	}

	// Create processor
	processor, err := NewProcessor(compressionType)
	if err != nil {
		file.Close()
		return nil, err
	}

	w := &Writer{
		file:      file,
		processor: processor,
		header:    header,
		buffer:    new(bytes.Buffer),
		chunkNum:  0,
	}

	return w, nil
}

// Write implements io.Writer interface
func (w *Writer) Write(p []byte) (n int, err error) {
	totalWritten := 0

	for len(p) > 0 {
		// Calculate available space in current chunk
		// First chunk has 4 bytes less due to header
		maxChunkSize := ChunkSize
		if w.chunkNum == 0 {
			maxChunkSize = ChunkSize - 4
		}

		// Reserve 1 byte for end marker
		availSpace := maxChunkSize - w.buffer.Len() - 1

		if availSpace <= 0 {
			// Flush current chunk
			if err := w.flushChunk(false); err != nil {
				return totalWritten, err
			}
			continue
		}

		// Write what fits
		toWrite := len(p)
		if toWrite > availSpace {
			toWrite = availSpace
		}

		written, err := w.buffer.Write(p[:toWrite])
		if err != nil {
			return totalWritten, err
		}

		totalWritten += written
		p = p[written:]
	}

	return totalWritten, nil
}

// flushChunk writes the current buffer as a chunk
func (w *Writer) flushChunk(closing bool) error {
	if w.buffer.Len() == 0 && !closing {
		return nil
	}

	// Write end marker
	endMarker := byte(EntryTypeEndChunk)
	if closing {
		endMarker = byte(EntryTypeEndFile)
	}

	if err := w.buffer.WriteByte(endMarker); err != nil {
		return err
	}

	// Get chunk data
	chunkData := w.buffer.Bytes()

	// Write chunk using processor
	if err := w.processor.WriteChunk(w.file, chunkData); err != nil {
		return err
	}

	// Reset buffer
	w.buffer.Reset()
	w.chunkNum++

	return nil
}

// Close flushes the final chunk and closes the writer
func (w *Writer) Close() error {
	// Flush final chunk with end-of-file marker
	if err := w.flushChunk(true); err != nil {
		return err
	}

	if w.file != nil {
		return w.file.Close()
	}

	return nil
}

// WriteByte writes a single byte
func (w *Writer) WriteByte(b byte) error {
	_, err := w.Write([]byte{b})
	return err
}

// Header returns the package header
func (w *Writer) Header() *Header {
	return w.header
}

// BytesWritten returns the number of bytes written to the current chunk buffer
func (w *Writer) BytesWritten() int {
	return w.buffer.Len()
}

// AvailableSpace returns available space in current chunk (excluding end marker)
func (w *Writer) AvailableSpace() int {
	maxChunkSize := ChunkSize
	if w.chunkNum == 0 {
		maxChunkSize = ChunkSize - 4
	}
	return maxChunkSize - w.buffer.Len() - 1
}

// FlushChunk manually flushes the current chunk (for testing or forcing chunk boundaries)
func (w *Writer) FlushChunk() error {
	return w.flushChunk(false)
}

// CopyFrom copies data from a reader to the writer
func (w *Writer) CopyFrom(r io.Reader) (int64, error) {
	return io.Copy(w, r)
}

// WriteTo writes all buffered data to the provided writer (for nested writing)
func (w *Writer) WriteTo(dest io.Writer) (int64, error) {
	if w.buffer.Len() == 0 {
		return 0, nil
	}
	return io.Copy(dest, w.buffer)
}

// BufferSize returns the current buffer size
func (w *Writer) BufferSize() int {
	return w.buffer.Len()
}

// EnsureSpace ensures there is enough space in the buffer, flushing if necessary
func (w *Writer) EnsureSpace(needed int) error {
	if w.AvailableSpace() < needed {
		return w.flushChunk(false)
	}
	return nil
}

// WriteEntryTypeCode writes an entry type code and ensures the entry fits in the chunk
func (w *Writer) WriteEntryTypeCode(typeCode byte, estimatedSize int) error {
	// Ensure we have space for the type code + estimated entry size
	if err := w.EnsureSpace(1 + estimatedSize); err != nil {
		return err
	}

	return w.WriteByte(typeCode)
}

var (
	ErrEntryTooLarge = errors.New("entry too large for single chunk")
	ErrBufferFull    = errors.New("chunk buffer is full")
)
