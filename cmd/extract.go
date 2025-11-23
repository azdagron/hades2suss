package cmd

import (
	"github.com/azdagron/hades2suss/internal/entries"
	"github.com/azdagron/hades2suss/internal/ioutils"
	"github.com/azdagron/hades2suss/internal/pkg"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	extractSubtextures bool
)

var extractCmd = &cobra.Command{
	Use:   "extract <package> <output-dir>",
	Short: "Extract textures and atlases from a package file",
	Long: `Extract textures and atlas metadata from a .pkg file.

Extracts:
- Textures as PNG files (textures/ directory)
- Atlas metadata as JSON files (manifest/ directory)
- Optional: Individual sprites from atlases (subtextures/ directory)`,
	Args: cobra.ExactArgs(2),
	RunE: runExtract,
}

func init() {
	extractCmd.Flags().BoolVarP(&extractSubtextures, "subtextures", "s", false, "Extract individual sprites from texture atlases")
	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	packagePath := args[0]
	outputDir := args[1]

	fmt.Printf("Extracting package: %s\n", packagePath)
	fmt.Printf("Output directory: %s\n", outputDir)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Open package
	reader, err := pkg.NewReader(packagePath)
	if err != nil {
		return fmt.Errorf("opening package: %w", err)
	}
	defer reader.Close()

	fmt.Printf("Package version: %d\n", reader.Header().Version)
	fmt.Printf("Compression: 0x%02X\n", reader.Header().CompressionType)

	// Read and extract entries
	entryCount := 0
	textureCount := 0
	atlasCount := 0

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

		entryCount++

		// Extract based on type
		switch e := entry.(type) {
		case *entries.TextureEntry:
			textureCount++
			fmt.Printf("Extracting texture: %s\n", e.Name())
			if err := e.Extract(outputDir, extractSubtextures); err != nil {
				fmt.Printf("  Warning: failed to extract texture %s: %v\n", e.Name(), err)
			}

		case *entries.AtlasEntry:
			atlasCount++
			fmt.Printf("Extracting atlas: %s\n", e.Name())
			if err := e.Extract(outputDir, extractSubtextures); err != nil {
				fmt.Printf("  Warning: failed to extract atlas %s: %v\n", e.Name(), err)
			}

		default:
			fmt.Printf("Skipping unknown entry type: %T\n", entry)
		}
	}

	fmt.Printf("\nExtraction complete!\n")
	fmt.Printf("Entries extracted: %d\n", entryCount)
	fmt.Printf("  Textures: %d\n", textureCount)
	fmt.Printf("  Atlases: %d\n", atlasCount)

	// Check for manifest file
	manifestPath := packagePath + "_manifest"
	if _, err := os.Stat(manifestPath); err == nil {
		fmt.Printf("\nExtracting manifest: %s\n", manifestPath)
		if err := extractManifest(manifestPath, outputDir); err != nil {
			fmt.Printf("  Warning: failed to extract manifest: %v\n", err)
		}
	}

	return nil
}

// extractManifest extracts entries from a manifest file
func extractManifest(manifestPath, outputDir string) error {
	reader, err := pkg.NewReader(manifestPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	manifestCount := 0

	for {
		entry, err := readEntry(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if entry == nil {
			continue
		}

		manifestCount++

		if atlas, ok := entry.(*entries.AtlasEntry); ok {
			fmt.Printf("  Extracting manifest entry: %s\n", atlas.Name())
			if err := atlas.Extract(outputDir, extractSubtextures); err != nil {
				fmt.Printf("    Warning: failed to extract: %v\n", err)
			}
		}
	}

	fmt.Printf("  Manifest entries extracted: %d\n", manifestCount)
	return nil
}

// readEntry reads the next entry from the package
func readEntry(r *pkg.Reader) (entries.Entry, error) {
	// Read entry type code
	typeCode, err := ioutils.ReadByte(r)
	if err != nil {
		return nil, err
	}

	// Check for end markers
	if typeCode == pkg.EntryTypeEndFile || typeCode == pkg.EntryTypeEndChunk {
		return nil, io.EOF
	}

	// Create entry based on type code
	var entry entries.Entry

	switch typeCode {
	case entries.EntryTypeTexture2D:
		entry = &entries.TextureEntry{}

	case entries.EntryTypeAtlas:
		entry = &entries.AtlasEntry{}

	default:
		// Unknown entry type - skip it
		fmt.Printf("Warning: unknown entry type: 0x%02X\n", typeCode)
		return nil, nil
	}

	// Read entry data
	if err := entry.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("reading entry data: %w", err)
	}

	return entry, nil
}
