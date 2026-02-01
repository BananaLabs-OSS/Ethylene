package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bananalabs-oss/potassium/diff"
	"github.com/bananalabs-oss/potassium/manifest"
	"github.com/spf13/cobra"
)

var (
	oldPath     string
	newPath     string
	outPath     string
	fromVersion string
	toVersion   string
	algorithm   string
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate a patch from old and new files",
	RunE:  runGen,
}

func init() {
	genCmd.Flags().StringVar(&oldPath, "old", "", "Path to old file")
	genCmd.Flags().StringVar(&newPath, "new", "", "Path to new file")
	genCmd.Flags().StringVar(&outPath, "out", "./patch", "Output path for patch")
	genCmd.Flags().StringVar(&fromVersion, "from-version", "0.0.0", "Source version")
	genCmd.Flags().StringVar(&toVersion, "to-version", "0.0.1", "Target version")
	genCmd.Flags().StringVar(&algorithm, "algorithm", "bsdiff", "Diff algorithm: bsdiff or hdiff")
	genCmd.MarkFlagRequired("old")
	genCmd.MarkFlagRequired("new")
	rootCmd.AddCommand(genCmd)
}

func runGen(cmd *cobra.Command, args []string) error {
	// Validate algorithm
	if algorithm != "bsdiff" && algorithm != "hdiff" {
		return fmt.Errorf("unknown algorithm %q: must be bsdiff or hdiff", algorithm)
	}

	oldInfo, err := os.Stat(oldPath)
	if err != nil {
		return fmt.Errorf("cannot access old path: %w", err)
	}

	newInfo, err := os.Stat(newPath)
	if err != nil {
		return fmt.Errorf("cannot access new path: %w", err)
	}

	// Both must be the same type
	if oldInfo.IsDir() != newInfo.IsDir() {
		return fmt.Errorf("--old and --new must both be files or both be directories")
	}

	if oldInfo.IsDir() {
		return genDirectory(oldPath, newPath, outPath, fromVersion, toVersion)
	}
	return genFile(oldPath, newPath, outPath, fromVersion, toVersion)
}

// generatePatch creates a patch file using the selected algorithm.
// bsdiff returns bytes (we write them), hdiff writes directly to disk.
func generatePatch(oldFile, newFile, patchFilePath string) error {
	switch algorithm {
	case "hdiff":
		return diff.GenerateHDiff(oldFile, newFile, patchFilePath)
	default:
		patchBytes, err := diff.Generate(oldFile, newFile)
		if err != nil {
			return err
		}
		return os.WriteFile(patchFilePath, patchBytes, 0644)
	}
}

// genFile handles single-file patch generation (existing v0.1.0 behavior)
func genFile(oldFile, newFile, outDir, fromVer, toVer string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
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
		Algorithm: algorithm,
	})

	// Save manifest
	if err := m.Save(filepath.Join(outDir, "manifest.json")); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("Patch generated: %s -> %s\n", fromVer, toVer)
	fmt.Printf("  Patched: %s\n", filepath.Base(oldFile))
	return nil
}

// walkDir builds a map of relative file paths to their SHA256 hashes.
// All paths are normalized to forward slashes for cross-platform manifests.
func walkDir(root string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Forward slashes in manifest so patches work cross-platform
		rel = filepath.ToSlash(rel)

		hash, err := manifest.HashFile(path)
		if err != nil {
			return fmt.Errorf("failed to hash %s: %w", rel, err)
		}

		files[rel] = hash
		return nil
	})

	return files, err
}

// genDirectory handles directory-to-directory patch generation.
// Walks both trees, compares hashes, and produces patch/add/delete entries.
func genDirectory(oldDir, newDir, outDir, fromVer, toVer string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Println("Scanning old directory...")
	oldFiles, err := walkDir(oldDir)
	if err != nil {
		return fmt.Errorf("failed to walk old directory: %w", err)
	}
	fmt.Printf("  Found %d files\n", len(oldFiles))

	fmt.Println("Scanning new directory...")
	newFiles, err := walkDir(newDir)
	if err != nil {
		return fmt.Errorf("failed to walk new directory: %w", err)
	}
	fmt.Printf("  Found %d files\n", len(newFiles))

	m := manifest.New(fromVer, toVer)
	var patched, added, deleted, unchanged int

	// Pass 1: Check every file in OLD against NEW
	for relPath, oldHash := range oldFiles {
		newHash, existsInNew := newFiles[relPath]

		if !existsInNew {
			// File was removed in the new version
			m.AddFile(manifest.FileEntry{
				Path:    relPath,
				Action:  "delete",
				OldHash: oldHash,
			})
			deleted++
			fmt.Printf("  Delete: %s\n", relPath)
			continue
		}

		if oldHash == newHash {
			// Identical — nothing to do
			unchanged++
			continue
		}

		// File changed — generate patch
		oldFilePath := filepath.Join(oldDir, filepath.FromSlash(relPath))
		newFilePath := filepath.Join(newDir, filepath.FromSlash(relPath))

		patchFileName := relPath + ".patch"
		patchFilePath := filepath.Join(outDir, filepath.FromSlash(patchFileName))
		if err := os.MkdirAll(filepath.Dir(patchFilePath), 0755); err != nil {
			return fmt.Errorf("failed to create patch dir: %w", err)
		}

		if err := generatePatch(oldFilePath, newFilePath, patchFilePath); err != nil {
			return fmt.Errorf("failed to diff %s: %w", relPath, err)
		}

		m.AddFile(manifest.FileEntry{
			Path:      relPath,
			Action:    manifest.ActionPatch,
			OldHash:   oldHash,
			NewHash:   newHash,
			PatchFile: patchFileName,
			Algorithm: algorithm,
		})
		patched++
		fmt.Printf("  Patch:  %s\n", relPath)
	}

	// Pass 2: Find files that only exist in NEW
	for relPath, newHash := range newFiles {
		if _, existsInOld := oldFiles[relPath]; existsInOld {
			continue // already handled above
		}

		// Brand new file — copy the whole thing into patch bundle
		srcPath := filepath.Join(newDir, filepath.FromSlash(relPath))
		addFileName := "new/" + relPath
		destPath := filepath.Join(outDir, filepath.FromSlash(addFileName))

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create dir for new file: %w", err)
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read new file %s: %w", relPath, err)
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to copy new file %s: %w", relPath, err)
		}

		m.AddFile(manifest.FileEntry{
			Path:      relPath,
			Action:    "add",
			NewHash:   newHash,
			PatchFile: addFileName,
		})
		added++
		fmt.Printf("  Add:    %s\n", relPath)
	}

	// Save manifest
	if err := m.Save(filepath.Join(outDir, "manifest.json")); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("\nPatch generated: %s -> %s\n", fromVer, toVer)
	fmt.Printf("  %d patched, %d added, %d deleted, %d unchanged\n",
		patched, added, deleted, unchanged)
	return nil
}
