package encode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFileFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "cleaned.txt")
	if err := os.WriteFile(dest, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFile(filepath.Join(dir, "missing.tmp"), dest); err == nil {
		t.Fatal("replacement with missing source succeeded")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "existing\n" {
		t.Fatalf("destination changed: %q, %v", got, err)
	}
}

func TestPublishFileDoesNotOverwriteDestination(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "new.tmp")
	dest := filepath.Join(dir, "result.txt")
	if err := os.WriteFile(tmp, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PublishFile(tmp, dest); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "existing\n" {
		t.Fatalf("destination changed: %q, %v", got, err)
	}
}

func TestPublishFileCreatesDestination(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "new.tmp")
	dest := filepath.Join(dir, "result.txt")
	if err := os.WriteFile(tmp, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PublishFile(tmp, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "new\n" {
		t.Fatalf("destination=%q, %v", got, err)
	}
}
