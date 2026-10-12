package main

import (
	"os"
	"path/filepath"
	"testing"

	"cleaner/internal/discover"
	"cleaner/internal/metafile"
	"cleaner/internal/worker"
)

func TestUniqueJobsDeduplicatesAliasedDirectories(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	alias := filepath.Join(root, "alias")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "users.txt"), []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := []discover.Job{
		{Top: "real", FilePath: filepath.Join(real, "users.txt")},
		{Top: "alias", FilePath: filepath.Join(alias, "users.txt")},
	}
	unique := uniqueJobs(jobs)
	if len(unique) != 1 || unique[0].Top != "real" {
		t.Fatalf("unique=%+v", unique)
	}
}

func TestUniqueJobsDoesNotBlockOtherCandidatesWhenOneDisappears(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.txt")
	if err := os.WriteFile(good, []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := []discover.Job{
		{Top: "gone", FilePath: filepath.Join(root, "gone.txt")},
		{Top: "good", FilePath: good},
	}
	unique := uniqueJobs(jobs)
	if len(unique) != 2 {
		t.Fatalf("unique=%+v want both candidates", unique)
	}
	meta := metafile.New(filepath.Join(root, "cleaned.txt"))
	agg := worker.Run(unique, true, meta)
	if agg.OK != 1 || agg.Fail != 1 {
		t.Fatalf("ok=%d fail=%d want 1, 1", agg.OK, agg.Fail)
	}
	if !meta.Contains("good") || meta.Contains("gone") {
		t.Fatalf("completed folders: good=%v gone=%v", meta.Contains("good"), meta.Contains("gone"))
	}
	if _, err := os.Stat(good); !os.IsNotExist(err) {
		t.Fatalf("good source still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "good_cleaned.txt")); err != nil {
		t.Fatalf("good result missing: %v", err)
	}
}
