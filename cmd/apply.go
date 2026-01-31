package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bananalabs-oss/potassium/diff"
	"github.com/bananalabs-oss/potassium/manifest"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a patch to target files",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load manifest
		manifestPath := filepath.Join(patchPath, "manifest.json")
		m, err := manifest.Load(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		fmt.Printf("Applying patch: %s -> %s\n", m.FromVersion, m.ToVersion)

		for _, file := range m.Files {
			switch file.Action {
			case manifest.ActionPatch:
				// Verify old file hash
				targetFile := filepath.Join(targetPath, file.Path)
				currentHash, err := manifest.HashFile(targetFile)
				if err != nil {
					return fmt.Errorf("failed to hash target file: %w", err)
				}

				if currentHash != file.OldHash {
					return fmt.Errorf("hash mismatch for %s: expected %s, got %s", file.Path, file.OldHash[:16]+"...", currentHash[:16]+"...")
				}

				// Apply patch
				patchFile := filepath.Join(patchPath, file.PatchFile)
				tempOut := targetFile + ".new"

				if err := diff.Apply(targetFile, patchFile, tempOut); err != nil {
					return fmt.Errorf("failed to apply patch: %w", err)
				}

				// Verify new file hash
				newHash, err := manifest.HashFile(tempOut)
				if err != nil {
					os.Remove(tempOut)
					return fmt.Errorf("failed to hash new file: %w", err)
				}

				if newHash != file.NewHash {
					os.Remove(tempOut)
					return fmt.Errorf("output hash mismatch for %s", file.Path)
				}

				// Replace old with new
				if err := os.Rename(tempOut, targetFile); err != nil {
					return fmt.Errorf("failed to replace file: %w", err)
				}

				fmt.Printf("  Patched: %s\n", file.Path)

			case manifest.ActionAdd:
				fmt.Printf("  Add not implemented yet: %s\n", file.Path)

			case manifest.ActionDelete:
				targetFile := filepath.Join(targetPath, file.Path)
				if err := os.Remove(targetFile); err != nil {
					return fmt.Errorf("failed to delete file: %w", err)
				}
				fmt.Printf("  Deleted: %s\n", file.Path)
			}
		}

		fmt.Println("Patch applied successfully!")
		return nil
	},
}

var patchPath, targetPath string

func init() {
	applyCmd.Flags().StringVar(&patchPath, "patch", "", "Path to patch directory")
	applyCmd.Flags().StringVar(&targetPath, "target", ".", "Path to target directory")
	applyCmd.MarkFlagRequired("patch")
	rootCmd.AddCommand(applyCmd)
}
