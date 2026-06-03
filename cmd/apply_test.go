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
