package cmd

import (
	"crypto/sha256"
	"github.com/azdagron/hades2suss/internal/entries"
	"github.com/azdagron/hades2suss/internal/pkg"
	"github.com/azdagron/hades2suss/internal/xnb"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang/freetype/truetype"
	"github.com/spf13/cobra"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// Turbo Pascal colors
var (
	turboPascalBlue   = color.RGBA{0, 0, 170, 255}
	turboPascalYellow = color.RGBA{255, 255, 85, 255}
	turboPascalWhite  = color.RGBA{255, 255, 255, 255}
	dropShadowColor   = color.RGBA{0, 0, 0, 128} // Semi-transparent black
)

var patchCmd = &cobra.Command{
	Use:   "patch [manifest-path]",
	Short: "Patch a package with SUSSY BAKA overlay on portraits",
	Long: `Patches a package in-place by overlaying portrait textures with "SUSSY BAKA!" text.

Creates a backup (.pkg.orig) on first run and tracks changes with checksums (.pkg.patchsum.txt).
Skips patching if the package already matches the expected patched checksum.

If no manifest path is provided, automatically detects and patches all Aphrodite.pkg_manifest files
in the default Hades II Steam installation directory.

Examples:
  hades2suss patch Aphrodite.pkg_manifest
  hades2suss patch  # Auto-detects Steam installation

This is mainly for demonstration/meme purposes.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPatch,
}

func init() {
	rootCmd.AddCommand(patchCmd)
}

func runPatch(cmd *cobra.Command, args []string) error {
	var manifestPaths []string

	if len(args) == 0 {
		// Auto-detect Aphrodite.pkg_manifest files
		detected, err := findAphroditeManifests()
		if err != nil {
			return fmt.Errorf("auto-detecting manifests: %w", err)
		}
		if len(detected) == 0 {
			return fmt.Errorf("no Aphrodite.pkg_manifest files found in default Steam locations")
		}
		manifestPaths = detected
		fmt.Printf("Auto-detected %d Aphrodite manifest(s)\n", len(manifestPaths))
	} else {
		manifestPaths = []string{args[0]}
	}

	// Patch each manifest
	for i, manifestPath := range manifestPaths {
		if len(manifestPaths) > 1 {
			fmt.Printf("\n[%d/%d] Processing: %s\n", i+1, len(manifestPaths), manifestPath)
		}
		if err := patchManifest(manifestPath); err != nil {
			return fmt.Errorf("patching %s: %w", manifestPath, err)
		}
	}

	return nil
}

func patchManifest(manifestPath string) error {
	// Derive base package path by removing _manifest suffix
	if !strings.HasSuffix(manifestPath, "_manifest") {
		return fmt.Errorf("manifest path must end with _manifest")
	}
	packagePath := strings.TrimSuffix(manifestPath, "_manifest")

	fmt.Printf("Patching package: %s\n", packagePath)

	// Step 1: Check if package exists
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package file does not exist: %s", packagePath)
	}

	origPath := packagePath + ".orig"
	checksumPath := packagePath + ".patchsum.txt"

	// Step 2: Create backup if it doesn't exist
	if _, err := os.Stat(origPath); os.IsNotExist(err) {
		fmt.Printf("Creating backup: %s\n", origPath)
		data, err := os.ReadFile(packagePath)
		if err != nil {
			return fmt.Errorf("reading package for backup: %w", err)
		}
		if err := os.WriteFile(origPath, data, 0644); err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
	} else {
		fmt.Printf("Backup exists: %s\n", origPath)
	}

	// Step 3: Check if already patched
	if checksumData, err := os.ReadFile(checksumPath); err == nil {
		expectedChecksum := strings.TrimSpace(string(checksumData))

		// Calculate current package checksum
		currentData, err := os.ReadFile(packagePath)
		if err != nil {
			return fmt.Errorf("reading package for checksum: %w", err)
		}
		currentChecksum := calculateSHA256(currentData)

		if currentChecksum == expectedChecksum {
			fmt.Printf("Package already patched (checksum matches), skipping.\n")
			return nil
		}
		fmt.Printf("Package checksum mismatch, re-patching...\n")
	}

	// Step 4: Read manifest to find portrait locations
	portraitLocations, err := findPortraitLocations(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	if len(portraitLocations) == 0 {
		return fmt.Errorf("no portraits found in manifest")
	}

	fmt.Printf("Found %d portrait(s) in manifest\n", len(portraitLocations))

	// Step 5: Open original package for reading
	reader, err := pkg.NewReader(origPath)
	if err != nil {
		return fmt.Errorf("opening original package: %w", err)
	}
	defer reader.Close()

	// Step 6: Create temporary output file
	tempOutput := packagePath + ".tmp"
	writer, err := pkg.NewWriter(tempOutput, reader.Header().CompressionType, reader.Header().Version)
	if err != nil {
		return fmt.Errorf("creating temp output: %w", err)
	}
	defer writer.Close()

	// Process each entry
	textureCount := 0
	patchedCount := 0

	for {
		entry, err := readEntry(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading entry: %w", err)
		}

		if entry == nil {
			continue
		}

		// Check if this is a texture that contains a portrait
		if texEntry, ok := entry.(*entries.TextureEntry); ok {
			textureCount++

			// Check if this texture has a portrait location
			if rect, hasPortrait := portraitLocations[texEntry.Name()]; hasPortrait {
				fmt.Printf("Patching portrait in: %s (at x=%d, y=%d, w=%d, h=%d)\n",
					texEntry.Name(), rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())

				// Patch the texture at the specific location
				patchedData, err := patchTextureAtLocation(texEntry.Data, rect)
				if err != nil {
					fmt.Printf("  Warning: failed to patch: %v\n", err)
				} else {
					texEntry.Data = patchedData
					texEntry.Size = int32(len(patchedData))
					patchedCount++
					fmt.Printf("  Successfully patched!\n")
				}
			}

			// Write texture entry (always write, patched or not)
			if err := writeEntry(writer, texEntry); err != nil {
				return fmt.Errorf("writing texture entry: %w", err)
			}
		} else {
			// Write non-texture entries as-is
			if err := writeEntry(writer, entry); err != nil {
				return fmt.Errorf("writing entry: %w", err)
			}
		}
	}

	// Close writer to flush all data
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	// Step 7: Replace original with patched version
	fmt.Printf("\nReplacing package with patched version...\n")
	if err := os.Rename(tempOutput, packagePath); err != nil {
		return fmt.Errorf("replacing package: %w", err)
	}

	// Step 8: Calculate and write checksum
	patchedData, err := os.ReadFile(packagePath)
	if err != nil {
		return fmt.Errorf("reading patched package: %w", err)
	}
	checksum := calculateSHA256(patchedData)
	if err := os.WriteFile(checksumPath, []byte(checksum+"\n"), 0644); err != nil {
		return fmt.Errorf("writing checksum: %w", err)
	}

	fmt.Printf("\nPatching complete!\n")
	fmt.Printf("Textures processed: %d\n", textureCount)
	fmt.Printf("Portraits patched: %d\n", patchedCount)
	fmt.Printf("Checksum written to: %s\n", checksumPath)

	return nil
}

// calculateSHA256 calculates the SHA256 hash of data and returns it as a hex string
func calculateSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// findAphroditeManifests searches for Aphrodite.pkg_manifest files in default Steam locations
func findAphroditeManifests() ([]string, error) {
	var basePaths []string

	// Determine base paths based on OS
	switch runtime.GOOS {
	case "windows":
		basePaths = []string{
			`C:\Program Files (x86)\Steam\steamapps\common\Hades II`,
		}
	case "linux":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		basePaths = []string{
			filepath.Join(homeDir, ".steam", "steam", "steamapps", "common", "Hades II"),
		}
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		basePaths = []string{
			filepath.Join(homeDir, "Library", "Application Support", "Steam", "steamapps", "common", "Hades II"),
		}
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	var manifestPaths []string

	// Search each base path for Aphrodite.pkg_manifest
	for _, basePath := range basePaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue // Skip if path doesn't exist
		}

		// Search for Aphrodite.pkg_manifest recursively
		err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors, continue walking
			}
			if !info.IsDir() && strings.HasSuffix(path, "Aphrodite.pkg_manifest") {
				manifestPaths = append(manifestPaths, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("searching %s: %w", basePath, err)
		}
	}

	return manifestPaths, nil
}

// findPortraitLocations reads the manifest and finds base portrait sprites
func findPortraitLocations(manifestPath string) (map[string]image.Rectangle, error) {
	locations := make(map[string]image.Rectangle)

	// Open manifest package
	reader, err := pkg.NewReader(manifestPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Read all atlas entries
	for {
		entry, err := readEntry(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if entry == nil {
			continue
		}

		// Check if this is an atlas entry
		if atlasEntry, ok := entry.(*entries.AtlasEntry); ok {
			// Look for base portrait sprites (format: "Portraits\<Character>\Portraits_<Character>_*")
			for _, sprite := range atlasEntry.SubAtlases {
				if strings.Contains(sprite.Name, "\\Portraits_") {
					// This is a base portrait
					rect := image.Rect(
						int(sprite.Rect.X),
						int(sprite.Rect.Y),
						int(sprite.Rect.X+sprite.Rect.Width),
						int(sprite.Rect.Y+sprite.Rect.Height),
					)
					locations[atlasEntry.EntryName] = rect
					break // Only use the first portrait in each texture
				}
			}
		}
	}

	return locations, nil
}

// patchTextureAtLocation patches an XNB texture with the SUSSY BAKA overlay at a specific location
func patchTextureAtLocation(xnbData []byte, portraitRect image.Rectangle) ([]byte, error) {
	// Decode XNB to PNG image
	img, err := xnb.DecodeXNBToImage(xnbData)
	if err != nil {
		return nil, fmt.Errorf("decoding XNB: %w", err)
	}

	// Convert to RGBA for drawing
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Calculate overlay rectangle within the portrait bounds
	// 30% margin on left and right = 40% width, 45% margin top, 2% margin bottom = 53% height
	overlayWidth := portraitRect.Dx() * 40 / 100
	overlayHeight := portraitRect.Dy() * 53 / 100
	overlayX := portraitRect.Min.X + portraitRect.Dx()*30/100  // 30% margin from left
	overlayY := portraitRect.Min.Y + portraitRect.Dy()*45/100  // 45% margin from top

	overlayRect := image.Rect(overlayX, overlayY, overlayX+overlayWidth, overlayY+overlayHeight)

	// Draw the overlay
	drawOverlay(rgba, overlayRect)

	// Encode back to XNB
	patchedXNB, err := xnb.EncodeImageToXNB(rgba)
	if err != nil {
		return nil, fmt.Errorf("encoding to XNB: %w", err)
	}

	return patchedXNB, nil
}

func drawOverlay(img *image.RGBA, rect image.Rectangle) {
	// Draw blue background
	draw.Draw(img, rect, &image.Uniform{turboPascalBlue}, image.Point{}, draw.Src)

	// Draw yellow border (3 pixels thick, scaled by rectangle size)
	borderThickness := max(1, rect.Dx()/100)
	drawBorder(img, rect, turboPascalYellow, borderThickness)

	// Draw text with drop shadow (two lines)
	drawTextWithShadow(img, rect)
}

func drawBorder(img *image.RGBA, rect image.Rectangle, col color.Color, thickness int) {
	// Top border
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+thickness),
		&image.Uniform{col}, image.Point{}, draw.Src)
	// Bottom border
	draw.Draw(img, image.Rect(rect.Min.X, rect.Max.Y-thickness, rect.Max.X, rect.Max.Y),
		&image.Uniform{col}, image.Point{}, draw.Src)
	// Left border
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+thickness, rect.Max.Y),
		&image.Uniform{col}, image.Point{}, draw.Src)
	// Right border
	draw.Draw(img, image.Rect(rect.Max.X-thickness, rect.Min.Y, rect.Max.X, rect.Max.Y),
		&image.Uniform{col}, image.Point{}, draw.Src)
}

func drawTextWithShadow(img *image.RGBA, rect image.Rectangle) {
	// Parse the Go regular font
	ttf, err := truetype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}

	// Two lines of text
	line1 := "SUSSY"
	line2 := "BAKA!"

	// Calculate font size to fit within rectangle with padding
	// Start with 25% of height for each line, then adjust if needed for width
	fontSize := float64(rect.Dy()) * 0.25

	// Create font face with calculated size
	face := truetype.NewFace(ttf, &truetype.Options{
		Size: fontSize,
		DPI:  72,
	})

	// Measure text and adjust if too wide
	line1Width := font.MeasureString(face, line1).Ceil()
	line2Width := font.MeasureString(face, line2).Ceil()
	maxWidth := line1Width
	if line2Width > maxWidth {
		maxWidth = line2Width
	}

	// If text is too wide, scale down the font size
	availableWidth := rect.Dx() - (rect.Dx() / 10) // 10% padding on sides
	if maxWidth > availableWidth {
		fontSize = fontSize * float64(availableWidth) / float64(maxWidth)
		face = truetype.NewFace(ttf, &truetype.Options{
			Size: fontSize,
			DPI:  72,
		})
		line1Width = font.MeasureString(face, line1).Ceil()
		line2Width = font.MeasureString(face, line2).Ceil()
	}

	metrics := face.Metrics()
	lineHeight := (metrics.Ascent + metrics.Descent).Ceil()

	// Calculate positions (centered)
	line1X := rect.Min.X + (rect.Dx()-line1Width)/2
	line2X := rect.Min.X + (rect.Dx()-line2Width)/2

	// Vertically center both lines
	totalHeight := lineHeight * 2
	startY := rect.Min.Y + (rect.Dy()-totalHeight)/2 + metrics.Ascent.Ceil()

	line1Y := startY
	line2Y := startY + lineHeight

	// Scale shadow offset with rectangle size
	shadowOffset := max(2, rect.Dy()/40)

	// Draw drop shadow for line 1
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(dropShadowColor),
		Face: face,
		Dot:  fixed.P(line1X+shadowOffset, line1Y+shadowOffset),
	}
	drawer.DrawString(line1)

	// Draw white text for line 1
	drawer.Src = image.NewUniform(turboPascalWhite)
	drawer.Dot = fixed.P(line1X, line1Y)
	drawer.DrawString(line1)

	// Draw drop shadow for line 2
	drawer.Src = image.NewUniform(dropShadowColor)
	drawer.Dot = fixed.P(line2X+shadowOffset, line2Y+shadowOffset)
	drawer.DrawString(line2)

	// Draw white text for line 2
	drawer.Src = image.NewUniform(turboPascalWhite)
	drawer.Dot = fixed.P(line2X, line2Y)
	drawer.DrawString(line2)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
