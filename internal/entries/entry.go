package entries

import (
	"io"
)

// Entry represents a package entry that can be read from and written to a package
type Entry interface {
	// TypeCode returns the entry type code
	TypeCode() byte

	// Name returns the entry name
	Name() string

	// ReadFrom reads the entry data from the reader
	ReadFrom(r io.Reader) error

	// WriteTo writes the entry data to the writer
	WriteTo(w io.Writer) error

	// Extract extracts the entry to a directory
	Extract(targetDir string, subtextures bool) error

	// Import imports the entry from a file
	Import(path string) error
}

// Point represents a 2D integer point
type Point struct {
	X int32
	Y int32
}

// PointF represents a 2D float point
type PointF struct {
	X float32
	Y float32
}

// Rect represents a rectangle
type Rect struct {
	X      int32
	Y      int32
	Width  int32
	Height int32
}
