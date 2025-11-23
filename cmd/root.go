package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hades2suss",
	Short: "A tool for extracting and packing Hades 2 package files",
	Long: `hades2suss is a tool for working with Hades 2 package files (.pkg).

It can extract textures and atlas metadata from package files,
and pack them back into package files for modding.

Features:
- Extract textures as PNG files
- Extract atlas metadata as JSON
- Extract individual sprites from atlases
- Pack textures and atlases into package files
- Support for BC7 compressed textures
- LZ4 compression support`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
}
