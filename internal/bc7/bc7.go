package bc7

/*
#include "bcdec.h"
*/
import "C"
import (
	"errors"
	"image"
	"image/color"
	"unsafe"
)

// DecodeBC7 decodes BC7 compressed texture data to an RGBA image using bcdec library
// BC7 uses 4x4 pixel blocks, each block is 16 bytes (128 bits)
func DecodeBC7(data []byte, width, height int32) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	// Calculate number of blocks
	blocksX := (width + 3) / 4
	blocksY := (height + 3) / 4

	// Each block is 16 bytes
	blockSize := 16
	if len(data) < int(blocksX*blocksY)*blockSize {
		return nil, errors.New("BC7 data too short")
	}

	blockIndex := 0
	for by := int32(0); by < blocksY; by++ {
		for bx := int32(0); bx < blocksX; bx++ {
			// Get compressed block (16 bytes)
			blockStart := blockIndex * blockSize
			if blockStart+blockSize > len(data) {
				break
			}
			compressedBlock := data[blockStart : blockStart+blockSize]

			// Decompress block to 4x4 RGBA pixels (64 bytes)
			decompressedBlock := make([]byte, 64)

			// Call bcdec_bc7
			C.bcdec_bc7(
				unsafe.Pointer(&compressedBlock[0]),
				unsafe.Pointer(&decompressedBlock[0]),
				16, // destinationPitch: 4 pixels * 4 bytes per pixel
			)

			// Copy decompressed pixels to image
			for py := 0; py < 4; py++ {
				for px := 0; px < 4; px++ {
					x := int(bx)*4 + px
					y := int(by)*4 + py

					if x < int(width) && y < int(height) {
						offset := (py*4 + px) * 4
						img.Set(x, y, color.RGBA{
							R: decompressedBlock[offset+0],
							G: decompressedBlock[offset+1],
							B: decompressedBlock[offset+2],
							A: decompressedBlock[offset+3],
						})
					}
				}
			}

			blockIndex++
		}
	}

	return img, nil
}

// EncodeBC7 encodes an RGBA image to BC7 compressed data
func EncodeBC7(img image.Image) ([]byte, error) {
	// BC7 encoding is very complex and computationally expensive
	// For now, return an error indicating it's not implemented
	return nil, errors.New("BC7 encoding not yet implemented - use DDS files instead")
}
