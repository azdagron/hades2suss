package entries

import (
	"bytes"
	"github.com/azdagron/hades2suss/internal/ioutils"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

const (
	EntryTypeAtlas   = 0xDE
	AtlasMagic       = 0x7FB1776B
	ReferenceMarker  = 0xDD
)

// AtlasEntry represents a texture atlas entry with sprite mappings
type AtlasEntry struct {
	EntryName             string
	Version               int32
	SubAtlases            []SubAtlas
	IsReference           bool
	ReferencedTextureName string
}

// SubAtlas represents a sprite within a texture atlas
type SubAtlas struct {
	Name         string  `json:"name"`
	Rect         Rect    `json:"rect"`
	TopLeft      Point   `json:"topLeft"`
	OriginalSize Point   `json:"originalSize"`
	ScaleRatio   PointF  `json:"scaleRatio"`
	IsMulti      bool    `json:"isMulti,omitempty"`
	IsMip        bool    `json:"isMip,omitempty"`
	IsAlpha8     bool    `json:"isAlpha8,omitempty"`
	Hull         []Point `json:"hull,omitempty"`
}

// TypeCode returns the entry type code
func (e *AtlasEntry) TypeCode() byte {
	return EntryTypeAtlas
}

// Name returns the entry name
func (e *AtlasEntry) Name() string {
	return e.EntryName
}

// ReadFrom reads the atlas entry from the reader
func (e *AtlasEntry) ReadFrom(r io.Reader) error {
	// Atlas entries don't have a name field - the name comes from the referencedTextureName inside

	// Read content size (but ignore it - it's unreliable per Python code comment)
	// "This is the size, but we don't care about this and it gets ignored"
	var ignoredSize int32
	if err := binary.Read(r, binary.BigEndian, &ignoredSize); err != nil {
		return fmt.Errorf("reading content size: %w", err)
	}

	// Parse content directly from reader (no separate buffer needed)
	return e.parseContent(r)
}

// parseContent parses the atlas content data
func (e *AtlasEntry) parseContent(r io.Reader) error {
	// Read magic number
	magic, err := ioutils.ReadInt32BE(r)
	if err != nil {
		return fmt.Errorf("reading magic: %w", err)
	}

	if magic != AtlasMagic {
		return fmt.Errorf("invalid atlas magic: 0x%X", magic)
	}

	// Read version
	version, err := ioutils.ReadInt32BE(r)
	if err != nil {
		return fmt.Errorf("reading version: %w", err)
	}
	e.Version = version

	// Read sub-atlas count
	count, err := ioutils.ReadInt32BE(r)
	if err != nil {
		return fmt.Errorf("reading sub-atlas count: %w", err)
	}

	// Read each sub-atlas
	e.SubAtlases = make([]SubAtlas, count)
	for i := int32(0); i < count; i++ {
		if err := e.readSubAtlas(r, &e.SubAtlases[i]); err != nil {
			return fmt.Errorf("reading sub-atlas %d: %w", i, err)
		}
	}

	// Read reference marker
	marker, err := ioutils.ReadByte(r)
	if err != nil {
		return fmt.Errorf("reading reference marker: %w", err)
	}

	if marker == ReferenceMarker {
		e.IsReference = true
		refName, err := ioutils.ReadString(r)
		if err != nil {
			return fmt.Errorf("reading referenced texture name: %w", err)
		}
		e.ReferencedTextureName = refName
		e.EntryName = refName // Set entry name from referenced texture
	}

	return nil
}

// readSubAtlas reads a single sub-atlas
func (e *AtlasEntry) readSubAtlas(r io.Reader, sub *SubAtlas) error {
	var err error

	// Read name
	sub.Name, err = ioutils.ReadString(r)
	if err != nil {
		return fmt.Errorf("reading name: %w", err)
	}

	// Read rect
	sub.Rect.X, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}
	sub.Rect.Y, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}
	sub.Rect.Width, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}
	sub.Rect.Height, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}

	// Read top-left
	sub.TopLeft.X, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}
	sub.TopLeft.Y, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}

	// Read original size
	sub.OriginalSize.X, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}
	sub.OriginalSize.Y, err = ioutils.ReadInt32BE(r)
	if err != nil {
		return err
	}

	// Read scale ratio
	sub.ScaleRatio.X, err = ioutils.ReadFloat32BE(r)
	if err != nil {
		return err
	}
	sub.ScaleRatio.Y, err = ioutils.ReadFloat32BE(r)
	if err != nil {
		return err
	}

	// Read flags if version > 0
	if e.Version > 0 {
		flags, err := ioutils.ReadByte(r)
		if err != nil {
			return err
		}
		sub.IsMulti = (flags & 0x01) != 0
		sub.IsMip = (flags & 0x02) != 0
		sub.IsAlpha8 = (flags & 0x04) != 0
	}

	// Read hull if version > 2
	if e.Version > 2 {
		hullCount, err := ioutils.ReadInt32BE(r)
		if err != nil {
			return err
		}

		sub.Hull = make([]Point, hullCount)
		for i := int32(0); i < hullCount; i++ {
			sub.Hull[i].X, err = ioutils.ReadInt32BE(r)
			if err != nil {
				return err
			}
			sub.Hull[i].Y, err = ioutils.ReadInt32BE(r)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteTo writes the atlas entry to the writer
func (e *AtlasEntry) WriteTo(w io.Writer) error {
	// Build content buffer
	var contentBuf bytes.Buffer

	// Write magic
	if err := ioutils.WriteInt32BE(&contentBuf, AtlasMagic); err != nil {
		return err
	}

	// Write version
	if err := ioutils.WriteInt32BE(&contentBuf, e.Version); err != nil {
		return err
	}

	// Write sub-atlas count
	if err := ioutils.WriteInt32BE(&contentBuf, int32(len(e.SubAtlases))); err != nil {
		return err
	}

	// Write each sub-atlas
	for _, sub := range e.SubAtlases {
		if err := e.writeSubAtlas(&contentBuf, &sub); err != nil {
			return err
		}
	}

	// Write reference marker and texture name
	if e.IsReference {
		if err := ioutils.WriteByte(&contentBuf, ReferenceMarker); err != nil {
			return err
		}
		if err := ioutils.WriteString(&contentBuf, e.ReferencedTextureName); err != nil {
			return err
		}
	}

	// Write name
	if err := ioutils.WriteString(w, e.EntryName); err != nil {
		return err
	}

	// Write content size
	if err := ioutils.WriteInt32BE(w, int32(contentBuf.Len())); err != nil {
		return err
	}

	// Write content
	_, err := w.Write(contentBuf.Bytes())
	return err
}

// writeSubAtlas writes a single sub-atlas
func (e *AtlasEntry) writeSubAtlas(w io.Writer, sub *SubAtlas) error {
	// Write name
	if err := ioutils.WriteString(w, sub.Name); err != nil {
		return err
	}

	// Write rect
	if err := ioutils.WriteInt32BE(w, sub.Rect.X); err != nil {
		return err
	}
	if err := ioutils.WriteInt32BE(w, sub.Rect.Y); err != nil {
		return err
	}
	if err := ioutils.WriteInt32BE(w, sub.Rect.Width); err != nil {
		return err
	}
	if err := ioutils.WriteInt32BE(w, sub.Rect.Height); err != nil {
		return err
	}

	// Write top-left
	if err := ioutils.WriteInt32BE(w, sub.TopLeft.X); err != nil {
		return err
	}
	if err := ioutils.WriteInt32BE(w, sub.TopLeft.Y); err != nil {
		return err
	}

	// Write original size
	if err := ioutils.WriteInt32BE(w, sub.OriginalSize.X); err != nil {
		return err
	}
	if err := ioutils.WriteInt32BE(w, sub.OriginalSize.Y); err != nil {
		return err
	}

	// Write scale ratio
	if err := ioutils.WriteFloat32BE(w, sub.ScaleRatio.X); err != nil {
		return err
	}
	if err := ioutils.WriteFloat32BE(w, sub.ScaleRatio.Y); err != nil {
		return err
	}

	// Write flags if version > 0
	if e.Version > 0 {
		var flags byte
		if sub.IsMulti {
			flags |= 0x01
		}
		if sub.IsMip {
			flags |= 0x02
		}
		if sub.IsAlpha8 {
			flags |= 0x04
		}
		if err := ioutils.WriteByte(w, flags); err != nil {
			return err
		}
	}

	// Write hull if version > 2
	if e.Version > 2 {
		if err := ioutils.WriteInt32BE(w, int32(len(sub.Hull))); err != nil {
			return err
		}
		for _, point := range sub.Hull {
			if err := ioutils.WriteInt32BE(w, point.X); err != nil {
				return err
			}
			if err := ioutils.WriteInt32BE(w, point.Y); err != nil {
				return err
			}
		}
	}

	return nil
}

// Extract extracts the atlas to a JSON file and optionally extracts subtextures
func (e *AtlasEntry) Extract(targetDir string, subtextures bool) error {
	// Create manifest directory
	manifestDir := filepath.Join(targetDir, "manifest")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	// Write atlas JSON
	outputPath := filepath.Join(manifestDir, e.EntryName+".atlas.json")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling atlas JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("writing atlas JSON: %w", err)
	}

	// Extract subtextures if requested
	if subtextures {
		if err := e.extractSubtextures(targetDir); err != nil {
			return fmt.Errorf("extracting subtextures: %w", err)
		}
	}

	return nil
}

// extractSubtextures extracts individual sprites from the atlas texture
func (e *AtlasEntry) extractSubtextures(targetDir string) error {
	if e.ReferencedTextureName == "" {
		return nil
	}

	// Load the atlas texture
	texturePath := filepath.Join(targetDir, "textures", e.ReferencedTextureName+".png")
	atlasImg, err := loadPNG(texturePath)
	if err != nil {
		return fmt.Errorf("loading atlas texture: %w", err)
	}

	// Create subtextures directory
	subtexturesDir := filepath.Join(targetDir, "subtextures", e.EntryName)
	if err := os.MkdirAll(subtexturesDir, 0755); err != nil {
		return fmt.Errorf("creating subtextures directory: %w", err)
	}

	// Extract each sub-atlas
	for _, sub := range e.SubAtlases {
		// Create sub-image from atlas
		rect := image.Rect(
			int(sub.Rect.X),
			int(sub.Rect.Y),
			int(sub.Rect.X+sub.Rect.Width),
			int(sub.Rect.Y+sub.Rect.Height),
		)

		subImg := image.NewRGBA(rect)
		draw.Draw(subImg, rect, atlasImg, image.Point{int(sub.Rect.X), int(sub.Rect.Y)}, draw.Src)

		// Save sub-image
		outputPath := filepath.Join(subtexturesDir, sub.Name+".png")
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("creating subtexture directory: %w", err)
		}

		if err := savePNG(subImg, outputPath); err != nil {
			return fmt.Errorf("saving subtexture %s: %w", sub.Name, err)
		}
	}

	return nil
}

// Import imports an atlas from a JSON file
func (e *AtlasEntry) Import(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading atlas file: %w", err)
	}

	if err := json.Unmarshal(data, e); err != nil {
		return fmt.Errorf("unmarshaling atlas JSON: %w", err)
	}

	// Extract name from path
	basename := filepath.Base(path)
	ext := filepath.Ext(basename)
	e.EntryName = basename[:len(basename)-len(ext)]

	// Remove .atlas suffix if present
	if len(e.EntryName) > 6 && e.EntryName[len(e.EntryName)-6:] == ".atlas" {
		e.EntryName = e.EntryName[:len(e.EntryName)-6]
	}

	return nil
}

// Helper functions for image I/O
func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func savePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
