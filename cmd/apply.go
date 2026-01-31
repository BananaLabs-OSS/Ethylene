package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bananalabs-oss/potassium/diff"
	"github.com/bananalabs-oss/potassium/manifest"
	"github.com/spf13/cobra"
)

var (
	patchPath  string
	targetPath string
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a patch to target files or directory",
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().StringVar(&patchPath, "patch", "", "Path to patch directory")
	applyCmd.Flags().StringVar(&targetPath, "target", ".", "Path to target directory")
	applyCmd.MarkFlagRequired("patch")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	m, err := manifest.Load(filepath.Join(patchPath, "manifest.json"))
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	fmt.Printf("Applying patch: %s -> %s\n", m.FromVersion, m.ToVersion)

	var patched, added, deleted int

	for _, f := range m.Files {
		targetFile := filepath.Join(targetPath, filepath.FromSlash(f.Path))

		switch f.Action {
		// PATCH: Apply byte-level diff
		case manifest.ActionPatch:
			currentHash, err := manifest.HashFile(targetFile)
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", f.Path, err)
			}
			if currentHash != f.OldHash {
				return fmt.Errorf("hash mismatch for %s\n  expected: %s\n       got: %s",
					f.Path, f.OldHash[:16], currentHash[:16])
			}

			patchFile := filepath.Join(patchPath, filepath.FromSlash(f.PatchFile))
			if err != nil {
				return fmt.Errorf("cannot read patch for %s: %w", f.Path, err)
			}

			if err := diff.Apply(targetFile, patchFile, targetFile); err != nil {
				return fmt.Errorf("failed to patch %s: %w", f.Path, err)
			}

			// Verify after patching
			newHash, err := manifest.HashFile(targetFile)
			if err != nil {
				return fmt.Errorf("cannot verify %s after patch: %w", f.Path, err)
			}
			if newHash != f.NewHash {
				return fmt.Errorf("post-patch hash mismatch for %s\n  expected: %s\n       got: %s",
					f.Path, f.NewHash[:16], newHash[:16])
			}

			patched++
			fmt.Printf("  Patched: %s\n", f.Path)

		// ADD: Copy new file into target
		case manifest.ActionAdd:
			srcFile := filepath.Join(patchPath, filepath.FromSlash(f.PatchFile))

			// Create parent directories if they don't exist
			if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
				return fmt.Errorf("cannot create directory for %s: %w", f.Path, err)
			}

			if err := copyFile(srcFile, targetFile); err != nil {
				return fmt.Errorf("failed to add %s: %w", f.Path, err)
			}

			// Verify hash of added file
			newHash, err := manifest.HashFile(targetFile)
			if err != nil {
				return fmt.Errorf("cannot verify added file %s: %w", f.Path, err)
			}
			if newHash != f.NewHash {
				return fmt.Errorf("hash mismatch for added file %s\n  expected: %s\n       got: %s",
					f.Path, f.NewHash[:16], newHash[:16])
			}

			added++
			fmt.Printf("  Added:   %s\n", f.Path)

		// DELETE: Remove file from target
		case manifest.ActionDelete:
			// Verify we're deleting the right file
			if f.OldHash != "" {
				currentHash, err := manifest.HashFile(targetFile)
				if err != nil {
					// File already gone — not an error
					fmt.Printf("  Skipped: %s (already removed)\n", f.Path)
					continue
				}
				if currentHash != f.OldHash {
					return fmt.Errorf("hash mismatch for %s before delete — refusing to remove wrong file",
						f.Path)
				}
			}

			if err := os.Remove(targetFile); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete %s: %w", f.Path, err)
			}

			// Clean up empty parent directories
			cleanEmptyDirs(filepath.Dir(targetFile), targetPath)

			deleted++
			fmt.Printf("  Deleted: %s\n", f.Path)

		default:
			fmt.Printf("  Unknown action '%s' for %s, skipping\n", f.Action, f.Path)
		}
	}

	fmt.Printf("\nPatch applied successfully!\n")
	fmt.Printf("  %d patched, %d added, %d deleted\n", patched, added, deleted)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// cleanEmptyDirs removes empty parent directories up to (but not including) root.
// Prevents leftover empty folders after deleting files.
func cleanEmptyDirs(dir, root string) {
	absRoot, _ := filepath.Abs(root)
	for {
		absDir, _ := filepath.Abs(dir)
		if absDir == absRoot || len(absDir) <= len(absRoot) {
			break
		}

		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break // not empty or can't read — stop
		}

		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
