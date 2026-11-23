package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "project.json")
	if err := WriteFileAtomic(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("content = %q", data)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "project.json" {
			t.Fatalf("unexpected temporary file %q", entry.Name())
		}
	}
}

func TestWithin(t *testing.T) {
	root := t.TempDir()
	if !Within(root, filepath.Join(root, "assets", "a.png")) {
		t.Fatal("descendant should be within root")
	}
	if !Within(root, root) {
		t.Fatal("root should be within itself")
	}
	if Within(root, filepath.Join(root, "..", "outside.png")) {
		t.Fatal("parent path should be outside root")
	}
}
