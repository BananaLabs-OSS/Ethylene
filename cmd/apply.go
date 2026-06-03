package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		if err := withinRoot(targetFile, targetPath); err != nil {
			return fmt.Errorf("path traversal blocked: %s: %w", f.Path, err)
		}

		switch f.Action {
		// PATCH: Apply byte-level diff
		case manifest.ActionPatch:
			currentHash, err := manifest.HashFile(targetFile)
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", f.Path, err)
			}
			if currentHash != f.OldHash {
				return fmt.Errorf("hash mismatch for %s\n  expected: %s\n       got: %s",
					f.Path, short(f.OldHash), short(currentHash))
			}

			patchFile := filepath.Join(patchPath, filepath.FromSlash(f.PatchFile))
			if err := withinRoot(patchFile, patchPath); err != nil {
				return fmt.Errorf("path traversal blocked in patch: %s: %w", f.PatchFile, err)
			}

			if err := applyPatch(targetFile, patchFile, f.Algorithm); err != nil {
				return fmt.Errorf("failed to patch %s: %w", f.Path, err)
			}

			// Verify after patching
			newHash, err := manifest.HashFile(targetFile)
			if err != nil {
				return fmt.Errorf("cannot verify %s after patch: %w", f.Path, err)
			}
			if newHash != f.NewHash {
				return fmt.Errorf("post-patch hash mismatch for %s\n  expected: %s\n       got: %s",
					f.Path, short(f.NewHash), short(newHash))
			}

			patched++
			fmt.Printf("  Patched: %s\n", f.Path)

		// ADD: Copy new file into target
		case manifest.ActionAdd:
			srcFile := filepath.Join(patchPath, filepath.FromSlash(f.PatchFile))
			if err := withinRoot(srcFile, patchPath); err != nil {
				return fmt.Errorf("path traversal blocked in add source: %s: %w", f.PatchFile, err)
			}

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
					f.Path, short(f.NewHash), short(newHash))
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

// applyPatch dispatches to the correct engine based on the algorithm field.
// Empty or "bsdiff" uses bsdiff. "hdiff" shells out to hpatchz.
func applyPatch(targetFile, patchFile, algo string) error {
	switch algo {
	case "hdiff":
		// hpatchz writes to a new file — we use a temp, then replace
		tmpFile := targetFile + ".hdiff_tmp"
		if err := diff.ApplyHDiff(targetFile, patchFile, tmpFile); err != nil {
			os.Remove(tmpFile)
			return err
		}
		// Replace original with patched version
		if err := os.Remove(targetFile); err != nil {
			os.Remove(tmpFile)
			return fmt.Errorf("failed to remove old file: %w", err)
		}
		return os.Rename(tmpFile, targetFile)
	default:
		// bsdiff (or empty for backward compatibility with old manifests)
		return diff.Apply(targetFile, patchFile, targetFile)
	}
}

// withinRoot verifies that child resolves to a path inside root, defeating both
// lexical "../" traversal and symlink-escape. It resolves symlinks on the
// deepest existing ancestor of child (the leaf itself may not exist yet for an
// add/patch write), then re-checks the realpath prefix. This also guards the
// TOCTOU case where a path element is a symlink redirecting the write outside
// root, because the symlink is dereferenced before the prefix check.
func withinRoot(child, root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// Root must exist; if it can't be resolved, refuse.
		return err
	}

	absChild, err := filepath.Abs(child)
	if err != nil {
		return err
	}

	// Resolve symlinks on the deepest ancestor that exists on disk. The
	// remaining (not-yet-created) suffix is appended lexically — it cannot
	// itself be a symlink because it does not exist.
	resolved := absChild
	suffix := ""
	for {
		r, err := filepath.EvalSymlinks(resolved)
		if err == nil {
			resolved = r
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(resolved)
		if parent == resolved {
			// Reached the volume/filesystem root without finding an
			// existing ancestor — fall back to the lexical absolute path.
			resolved = absChild
			suffix = ""
			break
		}
		suffix = filepath.Join(filepath.Base(resolved), suffix)
		resolved = parent
	}
	if suffix != "" {
		resolved = filepath.Join(resolved, suffix)
	}

	if resolved != realRoot && !strings.HasPrefix(resolved, realRoot+string(filepath.Separator)) {
		return fmt.Errorf("resolves outside root (%s)", resolved)
	}
	return nil
}

// short safely truncates a hash to 16 chars for display. Hash fields come from
// untrusted JSON, so a naive h[:16] panics on a short/malformed value.
func short(h string) string {
	if len(h) <= 16 {
		return h
	}
	return h[:16]
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
