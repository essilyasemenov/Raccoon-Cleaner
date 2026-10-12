package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsCandidate(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"a.txt", true},
		{"a.TXT", true},
		{"a.csv", true},
		{"a_cleaned.txt", false},
		{"a_cleaned_1.txt", false},
		{"a_CLEANED.TXT", false},
		{"readme.txt", false},
		{"README.TXT", false},
		{"a.log", false},
		{"cleaned.txt", true}, // "_cleaned" must be before ext as substring of stem
	}
	for _, tc := range cases {
		if got := isCandidate(tc.name); got != tc.ok {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.ok)
		}
	}
}

func TestWalkTXTCSVSkipsCleanedAndRootFiles(t *testing.T) {
	root := t.TempDir()
	top := filepath.Join(root, "batch1")
	if err := os.Mkdir(top, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(top, "keep.txt"), "http://x.com\n")
	mustWrite(filepath.Join(top, "skip_cleaned.txt"), "x")
	mustWrite(filepath.Join(top, "readme.txt"), "x")
	mustWrite(filepath.Join(top, "other.log"), "x")
	sub := filepath.Join(top, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(filepath.Join(sub, "nested.CSV"), "http://y.com\n")

	jobs, err := WalkTXTCSV("batch1", top)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs=%d want 2: %+v", len(jobs), jobs)
	}
}
