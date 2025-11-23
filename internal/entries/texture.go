package entries

import (
	"github.com/azdagron/hades2suss/internal/ioutils"
	"github.com/azdagron/hades2suss/internal/xnb"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	EntryTypeTexture2D = 0xAD
)

// TextureEntry represents a 2D texture entry
type TextureEntry struct {
	EntryName string
	Size      int32
	Data      []byte // XNB data
}

// TypeCode returns the entry type code
func (e *TextureEntry) TypeCode() byte {
	return EntryTypeTexture2D
}

// Name returns the entry name
func (e *TextureEntry) Name() string {
	return e.EntryName
}

// ReadFrom reads the texture entry from the reader
func (e *TextureEntry) ReadFrom(r io.Reader) error {
	// Read name
	name, err := ioutils.ReadString(r)
	if err != nil {
		return fmt.Errorf("reading texture name: %w", err)
	}
	e.EntryName = name

	// Read size
	size, err := ioutils.ReadInt32BE(r)
	if err != nil {
		return fmt.Errorf("reading texture size: %w", err)
	}
	e.Size = size

	// Read XNB data
	e.Data = make([]byte, size)
	if _, err := io.ReadFull(r, e.Data); err != nil {
		return fmt.Errorf("reading texture data: %w", err)
	}

	return nil
}

// WriteTo writes the texture entry to the writer
func (e *TextureEntry) WriteTo(w io.Writer) error {
	// Write name
	if err := ioutils.WriteString(w, e.EntryName); err != nil {
		return fmt.Errorf("writing texture name: %w", err)
	}

	// Write size
	if err := ioutils.WriteInt32BE(w, e.Size); err != nil {
		return fmt.Errorf("writing texture size: %w", err)
	}

	// Write XNB data
	if _, err := w.Write(e.Data); err != nil {
		return fmt.Errorf("writing texture data: %w", err)
	}

	return nil
}

// Extract extracts the texture to a PNG file
func (e *TextureEntry) Extract(targetDir string, subtextures bool) error {
	// Create textures directory
	texturesDir := filepath.Join(targetDir, "textures")
	if err := os.MkdirAll(texturesDir, 0755); err != nil {
		return fmt.Errorf("creating textures directory: %w", err)
	}

	// Use the original name to preserve path information
	// Just replace backslashes with forward slashes for cross-platform compatibility
	cleanName := filepath.ToSlash(e.EntryName)

	// Determine output path
	outputPath := filepath.Join(texturesDir, cleanName+".png")

	// Create subdirectories if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Convert XNB to PNG
	if err := xnb.ConvertXNBToPNG(e.Data, outputPath); err != nil {
		return fmt.Errorf("converting XNB to PNG: %w", err)
	}

	return nil
}

// Import imports a texture from a PNG or DDS file
func (e *TextureEntry) Import(path string) error {
	ext := filepath.Ext(path)
	var err error

	switch ext {
	case ".png":
		e.Data, err = xnb.ConvertPNGToXNB(path)
	case ".dds":
		e.Data, err = xnb.ConvertDDSToXNB(path)
	default:
		return fmt.Errorf("unsupported texture format: %s", ext)
	}

	if err != nil {
		return err
	}

	e.Size = int32(len(e.Data))

	// Extract name from path (without extension)
	basename := filepath.Base(path)
	e.EntryName = basename[:len(basename)-len(ext)]

	return nil
}
