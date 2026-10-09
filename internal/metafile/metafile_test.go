package metafile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cleaned.txt")
	s := New(path)

	if err := s.Append("folder_a"); err != nil {
		t.Fatal(err)
	}
	// Second append must replace existing file (Windows cannot rename-overwrite).
	if err := s.Append("folder_b"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "folder_a\nfolder_b\n" {
		t.Fatalf("metafile=%q want sorted folder_a/folder_b", got)
	}
	if !s.Contains("folder_a") || !s.Contains("folder_b") {
		t.Fatal("missing names in store")
	}
}

func TestAppendFailureLeavesNoStateOrTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(dir)
	if err := s.Append("folder_a"); err == nil {
		t.Fatal("expected replacement error")
	}
	if s.Contains("folder_a") {
		t.Fatal("failed append left name in store")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Fatalf("temporary files remain: %v", entries)
	}
}
