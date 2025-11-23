package xnb

import (
	"bytes"
	"github.com/azdagron/hades2suss/internal/bc7"
	"github.com/azdagron/hades2suss/internal/ioutils"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
)

// Image format codes
const (
	FormatBGRA    = 0  // Color format (BGRA)
	FormatBC3     = 6  // DXT5
	FormatAlpha8  = 12 // Alpha only
	FormatLA      = 27 // Luminance-Alpha
	FormatBC7     = 28 // BC7 compression
)

// XNBHeader represents the XNB file header
type XNBHeader struct {
	Magic    [4]byte // 'XNBw'
	Version  byte    // 5 or 6
	Flags    byte    // 0x00 = uncompressed
	FileSize uint32  // Little-endian
}

// XNBImage represents the image data within an XNB file
type XNBImage struct {
	Format    int32
	Width     int32
	Height    int32
	MipLevels int32
	DataSize  int32
	PixelData []byte
}

// DecodeXNBToImage decodes XNB data to an image.Image
func DecodeXNBToImage(xnbData []byte) (image.Image, error) {
	r := bytes.NewReader(xnbData)

	// Read XNB header
	header, err := readXNBHeader(r)
	if err != nil {
		return nil, fmt.Errorf("reading XNB header: %w", err)
	}

	if header.Flags != 0 {
		return nil, errors.New("compressed XNB not supported")
	}

	// Skip type readers for version 5
	if header.Version == 5 {
		if err := skipTypeReaders(r); err != nil {
			return nil, fmt.Errorf("skipping type readers: %w", err)
		}
	}

	// Read image data
	imgData, err := readXNBImage(r)
	if err != nil {
		return nil, fmt.Errorf("reading image data: %w", err)
	}

	// Decode image based on format
	var img image.Image
	switch imgData.Format {
	case FormatBGRA:
		img = decodeBGRA(imgData.PixelData, imgData.Width, imgData.Height)
	case FormatBC7:
		img, err = bc7.DecodeBC7(imgData.PixelData, imgData.Width, imgData.Height)
		if err != nil {
			return nil, fmt.Errorf("decoding BC7: %w", err)
		}
	case FormatBC3:
		return nil, errors.New("BC3/DXT5 format not yet implemented")
	case FormatAlpha8:
		img = decodeAlpha8(imgData.PixelData, imgData.Width, imgData.Height)
	case FormatLA:
		img = decodeLA(imgData.PixelData, imgData.Width, imgData.Height)
	default:
		return nil, fmt.Errorf("unsupported image format: %d", imgData.Format)
	}

	return img, nil
}

// EncodeImageToXNB encodes an image.Image to XNB data (BGRA format)
func EncodeImageToXNB(img image.Image) ([]byte, error) {
	// Convert to RGBA if needed
	bounds := img.Bounds()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	// Build XNB
	var buf bytes.Buffer

	// Header
	buf.Write([]byte("XNBw"))
	buf.WriteByte(6)  // Version 6
	buf.WriteByte(0)  // Uncompressed

	// Reserve space for file size (will update later)
	sizePos := buf.Len()
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Image data (BGRA format)
	ioutils.WriteInt32LE(&buf, FormatBGRA)
	ioutils.WriteInt32LE(&buf, int32(bounds.Dx()))
	ioutils.WriteInt32LE(&buf, int32(bounds.Dy()))
	ioutils.WriteInt32LE(&buf, int32(1)) // Mip levels

	// Convert RGBA to BGRA
	pixelData := encodeBGRA(rgba)
	ioutils.WriteInt32LE(&buf, int32(len(pixelData)))
	buf.Write(pixelData)

	// Update file size
	result := buf.Bytes()
	binary.LittleEndian.PutUint32(result[sizePos:], uint32(len(result)))

	return result, nil
}

// ConvertXNBToPNG converts XNB data to a PNG file
func ConvertXNBToPNG(xnbData []byte, outputPath string) error {
	r := bytes.NewReader(xnbData)

	// Read XNB header
	header, err := readXNBHeader(r)
	if err != nil {
		return fmt.Errorf("reading XNB header: %w", err)
	}

	if header.Flags != 0 {
		return errors.New("compressed XNB not supported")
	}

	// Skip type readers for version 5
	if header.Version == 5 {
		if err := skipTypeReaders(r); err != nil {
			return fmt.Errorf("skipping type readers: %w", err)
		}
	}

	// Read image data
	imgData, err := readXNBImage(r)
	if err != nil {
		return fmt.Errorf("reading image data: %w", err)
	}

	// Decode image based on format
	var img image.Image
	switch imgData.Format {
	case FormatBGRA:
		img = decodeBGRA(imgData.PixelData, imgData.Width, imgData.Height)
	case FormatBC7:
		img, err = bc7.DecodeBC7(imgData.PixelData, imgData.Width, imgData.Height)
		if err != nil {
			return fmt.Errorf("decoding BC7: %w", err)
		}
	case FormatBC3:
		return errors.New("BC3/DXT5 format not yet implemented")
	case FormatAlpha8:
		img = decodeAlpha8(imgData.PixelData, imgData.Width, imgData.Height)
	case FormatLA:
		img = decodeLA(imgData.PixelData, imgData.Width, imgData.Height)
	default:
		return fmt.Errorf("unsupported image format: %d", imgData.Format)
	}

	// Save as PNG
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}

	return nil
}

// ConvertPNGToXNB converts a PNG file to XNB data
func ConvertPNGToXNB(pngPath string) ([]byte, error) {
	// Read PNG
	f, err := os.Open(pngPath)
	if err != nil {
		return nil, fmt.Errorf("opening PNG file: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding PNG: %w", err)
	}

	// Convert to RGBA if needed
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Build XNB
	var buf bytes.Buffer

	// Write header
	buf.Write([]byte("XNBw"))
	buf.WriteByte(6)  // Version 6
	buf.WriteByte(0)  // Uncompressed

	// Reserve space for file size (will update later)
	sizePos := buf.Len()
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Write image data (BGRA format)
	ioutils.WriteInt32LE(&buf, FormatBGRA)
	ioutils.WriteInt32LE(&buf, int32(bounds.Dx()))
	ioutils.WriteInt32LE(&buf, int32(bounds.Dy()))
	ioutils.WriteInt32LE(&buf, int32(1)) // Mip levels

	// Convert RGBA to BGRA
	pixelData := encodeBGRA(rgba)
	ioutils.WriteInt32LE(&buf, int32(len(pixelData)))
	buf.Write(pixelData)

	// Update file size
	result := buf.Bytes()
	binary.LittleEndian.PutUint32(result[sizePos:], uint32(len(result)))

	return result, nil
}

// ConvertDDSToXNB converts a DDS file (with BC7) to XNB data
func ConvertDDSToXNB(ddsPath string) ([]byte, error) {
	// Read DDS file
	data, err := os.ReadFile(ddsPath)
	if err != nil {
		return nil, fmt.Errorf("reading DDS file: %w", err)
	}

	// Validate DDS header
	if len(data) < 148 || string(data[0:4]) != "DDS " {
		return nil, errors.New("invalid DDS file")
	}

	// Read dimensions from DDS_HEADER
	height := binary.LittleEndian.Uint32(data[12:16])
	width := binary.LittleEndian.Uint32(data[16:20])

	// Skip DDS header (128 bytes) + DX10 header (20 bytes) = 148 bytes
	pixelData := data[148:]

	// Build XNB
	var buf bytes.Buffer

	// Write header
	buf.Write([]byte("XNBw"))
	buf.WriteByte(6)  // Version 6
	buf.WriteByte(0)  // Uncompressed

	// Reserve space for file size
	sizePos := buf.Len()
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Write image data (BC7 format)
	ioutils.WriteInt32LE(&buf, FormatBC7)
	ioutils.WriteInt32LE(&buf, int32(width))
	ioutils.WriteInt32LE(&buf, int32(height))
	ioutils.WriteInt32LE(&buf, int32(1)) // Mip levels
	ioutils.WriteInt32LE(&buf, int32(len(pixelData)))
	buf.Write(pixelData)

	// Update file size
	result := buf.Bytes()
	binary.LittleEndian.PutUint32(result[sizePos:], uint32(len(result)))

	return result, nil
}

// readXNBHeader reads the XNB header
func readXNBHeader(r io.Reader) (*XNBHeader, error) {
	header := &XNBHeader{}

	if _, err := io.ReadFull(r, header.Magic[:]); err != nil {
		return nil, err
	}

	if string(header.Magic[:]) != "XNBw" {
		return nil, errors.New("invalid XNB magic")
	}

	if err := binary.Read(r, binary.BigEndian, &header.Version); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &header.Flags); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &header.FileSize); err != nil {
		return nil, err
	}

	return header, nil
}

// skipTypeReaders skips type reader data in XNB version 5
func skipTypeReaders(r io.Reader) error {
	// Read count
	count, err := ioutils.Read7BitEncodedInt(r)
	if err != nil {
		return err
	}

	// Skip each type reader
	for i := 0; i < count; i++ {
		// Skip type string
		typeLen, err := ioutils.Read7BitEncodedInt(r)
		if err != nil {
			return err
		}
		typeBytes := make([]byte, typeLen)
		if _, err := io.ReadFull(r, typeBytes); err != nil {
			return err
		}

		// Skip version (int32)
		var version int32
		if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
			return err
		}
	}

	// Skip two more 7-bit encoded ints
	if _, err := ioutils.Read7BitEncodedInt(r); err != nil {
		return err
	}
	if _, err := ioutils.Read7BitEncodedInt(r); err != nil {
		return err
	}

	return nil
}

// readXNBImage reads the image data from XNB
func readXNBImage(r io.Reader) (*XNBImage, error) {
	img := &XNBImage{}

	if err := binary.Read(r, binary.LittleEndian, &img.Format); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &img.Width); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &img.Height); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &img.MipLevels); err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.LittleEndian, &img.DataSize); err != nil {
		return nil, err
	}

	img.PixelData = make([]byte, img.DataSize)
	if _, err := io.ReadFull(r, img.PixelData); err != nil {
		return nil, err
	}

	return img, nil
}

// decodeBGRA converts BGRA pixel data to an RGBA image
func decodeBGRA(data []byte, width, height int32) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			offset := (y*int(width) + x) * 4
			if offset+3 < len(data) {
				// BGRA -> RGBA
				img.Set(x, y, color.RGBA{
					R: data[offset+2],
					G: data[offset+1],
					B: data[offset+0],
					A: data[offset+3],
				})
			}
		}
	}

	return img
}

// encodeBGRA converts an RGBA image to BGRA pixel data
func encodeBGRA(img *image.RGBA) []byte {
	bounds := img.Bounds()
	data := make([]byte, bounds.Dx()*bounds.Dy()*4)

	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.RGBAAt(x, y)
			// RGBA -> BGRA
			data[i+0] = c.B
			data[i+1] = c.G
			data[i+2] = c.R
			data[i+3] = c.A
			i += 4
		}
	}

	return data
}

// decodeAlpha8 converts Alpha8 pixel data to an RGBA image
func decodeAlpha8(data []byte, width, height int32) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			offset := y*int(width) + x
			if offset < len(data) {
				alpha := data[offset]
				img.Set(x, y, color.RGBA{
					R: 255,
					G: 255,
					B: 255,
					A: alpha,
				})
			}
		}
	}

	return img
}

// decodeLA converts Luminance-Alpha pixel data to an RGBA image
func decodeLA(data []byte, width, height int32) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			offset := (y*int(width) + x) * 2
			if offset+1 < len(data) {
				lum := data[offset]
				alpha := data[offset+1]
				img.Set(x, y, color.RGBA{
					R: lum,
					G: lum,
					B: lum,
					A: alpha,
				})
			}
		}
	}

	return img
}
