package cmd

import (
	"github.com/azdagron/hades2suss/internal/entries"
	"github.com/azdagron/hades2suss/internal/pkg"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	packBC7         bool
	packSubtextures bool
)

var packCmd = &cobra.Command{
	Use:   "pack <input-dir> <output-package>",
	Short: "Pack textures and atlases into a package file",
	Long: `Pack textures and atlas metadata into a .pkg file.

Expects directory structure:
- textures/ - PNG or DDS texture files
- manifest/ - Atlas JSON files
- subtextures/ - Individual sprite PNGs (with --subtextures)`,
	Args: cobra.ExactArgs(2),
	RunE: runPack,
}

func init() {
	packCmd.Flags().BoolVar(&packBC7, "bc7", false, "Use BC7 compressed DDS files instead of PNG")
	packCmd.Flags().BoolVarP(&packSubtextures, "subtextures", "s", false, "Build atlases from individual sprite images")
	rootCmd.AddCommand(packCmd)
}

func runPack(cmd *cobra.Command, args []string) error {
	inputDir := args[0]
	outputPackage := args[1]

	fmt.Printf("Packing directory: %s\n", inputDir)
	fmt.Printf("Output package: %s\n", outputPackage)
	if packBC7 {
		fmt.Println("Using BC7 compression (DDS files)")
	}

	// Scan manifest directory for atlas JSON files
	manifestDir := filepath.Join(inputDir, "manifest")
	atlasFiles, err := filepath.Glob(filepath.Join(manifestDir, "*.atlas.json"))
	if err != nil {
		return fmt.Errorf("scanning manifest directory: %w", err)
	}

	if len(atlasFiles) == 0 {
		fmt.Println("Warning: no atlas files found in manifest/ directory")
	}

	// Load atlas entries
	var atlasEntries []*entries.AtlasEntry
	for _, atlasPath := range atlasFiles {
		atlas := &entries.AtlasEntry{}
		if err := atlas.Import(atlasPath); err != nil {
			return fmt.Errorf("importing atlas %s: %w", atlasPath, err)
		}
		atlasEntries = append(atlasEntries, atlas)
	}

	// Create package writer
	writer, err := pkg.NewWriter(outputPackage, pkg.CompressionLZ4, pkg.VersionHades)
	if err != nil {
		return fmt.Errorf("creating package: %w", err)
	}
	defer writer.Close()

	// Create manifest writer
	manifestWriter, err := pkg.NewWriter(outputPackage+"_manifest", pkg.CompressionUncompressed, pkg.VersionHades)
	if err != nil {
		return fmt.Errorf("creating manifest: %w", err)
	}
	defer manifestWriter.Close()

	textureCount := 0
	atlasCount := 0

	// Process each atlas
	for _, atlas := range atlasEntries {
		// Write manifest entry
		if err := writeEntry(manifestWriter, atlas); err != nil {
			return fmt.Errorf("writing atlas %s to manifest: %w", atlas.Name(), err)
		}
		atlasCount++
		fmt.Printf("Packed atlas: %s\n", atlas.Name())

		// Find corresponding texture file
		textureName := atlas.ReferencedTextureName
		if textureName == "" {
			fmt.Printf("  Warning: atlas %s has no referenced texture\n", atlas.Name())
			continue
		}

		texturePath := findTexturePath(inputDir, textureName, packBC7)
		if texturePath == "" {
			fmt.Printf("  Warning: texture not found: %s\n", textureName)
			continue
		}

		// Create texture entry
		texture := &entries.TextureEntry{
			EntryName: textureName,
		}

		if err := texture.Import(texturePath); err != nil {
			return fmt.Errorf("importing texture %s: %w", texturePath, err)
		}

		// Write texture entry to main package
		if err := writeEntry(writer, texture); err != nil {
			return fmt.Errorf("writing texture %s: %w", texture.Name(), err)
		}
		textureCount++
		fmt.Printf("  Packed texture: %s (%s)\n", texture.Name(), filepath.Ext(texturePath))
	}

	fmt.Printf("\nPacking complete!\n")
	fmt.Printf("Textures packed: %d\n", textureCount)
	fmt.Printf("Atlases packed: %d\n", atlasCount)

	return nil
}

// findTexturePath finds a texture file (PNG or DDS) in the input directory
func findTexturePath(inputDir, textureName string, preferDDS bool) string {
	texturesDir := filepath.Join(inputDir, "textures")

	// Try with subdirectories
	candidates := []string{
		filepath.Join(texturesDir, textureName),
		filepath.Join(texturesDir, "atlases", textureName),
	}

	extensions := []string{".png", ".dds"}
	if preferDDS {
		extensions = []string{".dds", ".png"}
	}

	for _, base := range candidates {
		for _, ext := range extensions {
			path := base + ext
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	return ""
}

// writeEntry writes an entry to a package writer
func writeEntry(w *pkg.Writer, entry entries.Entry) error {
	// Write type code
	if err := w.WriteByte(entry.TypeCode()); err != nil {
		return err
	}

	// Write entry data
	if err := entry.WriteTo(w); err != nil {
		return err
	}

	return nil
}

// buildAtlasFromSubtextures builds a texture atlas from individual sprite images
func buildAtlasFromSubtextures(inputDir string, atlas *entries.AtlasEntry) error {
	// This is a placeholder for atlas building functionality
	// In production, you would use a texture packer library
	return fmt.Errorf("building atlases from subtextures not yet implemented")
}
