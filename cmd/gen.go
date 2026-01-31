package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bananalabs-oss/potassium/diff"
	"github.com/bananalabs-oss/potassium/manifest"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate a patch from old and new files",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create output directory
		if err := os.MkdirAll(outPath, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Hash old and new files
		oldHash, err := manifest.HashFile(oldPath)
		if err != nil {
			return fmt.Errorf("failed to hash old file: %w", err)
		}

		newHash, err := manifest.HashFile(newPath)
		if err != nil {
			return fmt.Errorf("failed to hash new file: %w", err)
		}

		// Generate diff
		patchBytes, err := diff.Generate(oldPath, newPath)
		if err != nil {
			return fmt.Errorf("failed to generate diff: %w", err)
		}

		// Write patch file
		patchFileName := filepath.Base(oldPath) + ".patch"
		patchFilePath := filepath.Join(outPath, patchFileName)
		if err := os.WriteFile(patchFilePath, patchBytes, 0644); err != nil {
			return fmt.Errorf("failed to write patch file: %w", err)
		}

		// Create manifest
		m := manifest.New(fromVersion, toVersion)
		m.AddFile(manifest.FileEntry{
			Path:      filepath.Base(oldPath),
			Action:    manifest.ActionPatch,
			OldHash:   oldHash,
			NewHash:   newHash,
			PatchFile: patchFileName,
		})

		// Save manifest
		manifestPath := filepath.Join(outPath, "manifest.json")
		if err := m.Save(manifestPath); err != nil {
			return fmt.Errorf("failed to save manifest: %w", err)
		}

		fmt.Printf("Patch generated:\n")
		fmt.Printf("  Manifest: %s\n", manifestPath)
		fmt.Printf("  Patch:    %s\n", patchFilePath)
		fmt.Printf("  Old hash: %s\n", oldHash[:16]+"...")
		fmt.Printf("  New hash: %s\n", newHash[:16]+"...")

		return nil
	},
}

var oldPath, newPath, outPath string
var fromVersion, toVersion string

func init() {
	genCmd.Flags().StringVar(&oldPath, "old", "", "Path to old file")
	genCmd.Flags().StringVar(&newPath, "new", "", "Path to new file")
	genCmd.Flags().StringVar(&outPath, "out", "./patch", "Output path for patch")
	genCmd.Flags().StringVar(&fromVersion, "from-version", "0.0.0", "Source version")
	genCmd.Flags().StringVar(&toVersion, "to-version", "0.0.1", "Target version")
	genCmd.MarkFlagRequired("old")
	genCmd.MarkFlagRequired("new")
	rootCmd.AddCommand(genCmd)
}
