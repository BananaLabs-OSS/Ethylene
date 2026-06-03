package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShortHandlesUntrustedHash(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"x":                "x",
		"0123456789abcdef": "0123456789abcdef",                 // exactly 16
		strings.Repeat("a", 64): strings.Repeat("a", 16),       // full sha256 → 16
	}
	for in, want := range cases {
		if got := short(in); got != want {
			t.Errorf("short(%q) = %q, want %q", in, got, want)
		}
	}
	// A 1-char hash must not panic (regression for the f.OldHash[:16] slice).
	_ = short("x")
}

func TestWithinRootAllowsInside(t *testing.T) {
	root := t.TempDir()
	for _, child := range []string{
		filepath.Join(root, "file.bin"),
		filepath.Join(root, "sub", "dir", "file.bin"),
		root,
	} {
		if err := withinRoot(child, root); err != nil {
			t.Errorf("withinRoot(%q, %q) rejected an in-root path: %v", child, root, err)
		}
	}
}

func TestWithinRootBlocksLexicalTraversal(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		filepath.Join("..", "escape.bin"),
		filepath.Join("..", "..", "etc", "passwd"),
		filepath.Join("sub", "..", "..", "escape.bin"),
	} {
		child := filepath.Join(root, rel)
		if err := withinRoot(child, root); err == nil {
			t.Errorf("withinRoot allowed traversal escape via %q", rel)
		}
	}
}

func TestWithinRootBlocksSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir() // sibling dir, definitely outside root

	// root/link -> outside ; a write to root/link/evil.bin lexically passes a
	// HasPrefix(root) check but really lands in outside/.
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation unavailable (needs privilege): %v", err)
		}
		t.Fatalf("could not create symlink: %v", err)
	}

	escapingChild := filepath.Join(link, "evil.bin")
	if err := withinRoot(escapingChild, root); err == nil {
		t.Errorf("withinRoot allowed symlink escape: %q resolves into %q", escapingChild, outside)
	}

	// A non-escaping path through a real subdir must still be allowed.
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := withinRoot(filepath.Join(root, "real", "ok.bin"), root); err != nil {
		t.Errorf("withinRoot wrongly rejected a legitimate in-root path: %v", err)
	}
}

// TestWithinRootNotYetExistingLeafInside pins the deepest-existing-ancestor +
// lexical-suffix branch of withinRoot (the "leaf may not exist yet for an
// add/patch write" case in the doc comment). Cross-platform — no symlink, so
// it runs on the locked-down Windows audit host too, unlike the symlink test.
// An add/patch writes to a file that does not exist yet, possibly several dirs
// deep; that target must still validate as in-root.
func TestWithinRootNotYetExistingLeafInside(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"newfile.bin",                                   // leaf missing, parent = root
		filepath.Join("newdir", "newfile.bin"),          // parent dir missing too
		filepath.Join("a", "b", "c", "deep.bin"),        // several missing levels
	} {
		child := filepath.Join(root, rel)
		if err := withinRoot(child, root); err != nil {
			t.Errorf("withinRoot rejected a not-yet-existing in-root write %q: %v", rel, err)
		}
	}
}

// TestWithinRootNotYetExistingLeafEscapes pins that the suffix branch does NOT
// become a traversal bypass: a not-yet-existing target whose lexical path
// climbs out of root must still be rejected. Cross-platform.
func TestWithinRootNotYetExistingLeafEscapes(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		filepath.Join("newdir", "..", "..", "escape.bin"),
		filepath.Join("a", "b", "..", "..", "..", "escape.bin"),
	} {
		child := filepath.Join(root, rel)
		if err := withinRoot(child, root); err == nil {
			t.Errorf("withinRoot allowed not-yet-existing traversal escape via %q", rel)
		}
	}
}
