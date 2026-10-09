package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFileStreamsMixedFormats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.txt")
	input := strings.Join([]string{
		"https://example.com:user123:pass1234",
		strings.Repeat("x", 128*1024),
		`"user456","pass5678","https://example.com"`,
		"https://example.com/path",
	}, "\r\n")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, true)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Kept != 2 {
		t.Fatalf("kept=%d want 2", result.Kept)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	cleaned, err := os.ReadFile(filepath.Join(dir, "mixed_cleaned.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"user123","pass1234"` + "\n" + `"user456","pass5678"` + "\n"
	if string(cleaned) != want {
		t.Fatalf("cleaned=%q want %q", cleaned, want)
	}
}

func TestProcessFileReadsCRDelimitedTwoColumnCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quoted.csv")
	bcrypt := "$2y$10$" + strings.Repeat("Ab", 26) + "A"
	input := strings.Join([]string{
		`"QUARTO","` + bcrypt + `"`,
		`"user123","pa,ss""word"`,
	}, "\r")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, false)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Skipped || result.Kept != 2 {
		t.Fatalf("skipped=%t kept=%d reasons=%v want false, 2", result.Skipped, result.Kept, result.Reasons)
	}
	cleaned, err := os.ReadFile(filepath.Join(dir, "quoted_cleaned.csv"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"QUARTO","` + bcrypt + `"` + "\n" + `"user123","pa,ss""word"` + "\n"
	if string(cleaned) != want {
		t.Fatalf("cleaned=%q want %q", cleaned, want)
	}
}

func TestProcessFileSkipsUnsuitableInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	input := "documentation\nhttps://example.com\n{\"enabled\":true}\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, true)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if !result.Skipped {
		t.Fatal("unsuitable file was not skipped")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes_cleaned.txt")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}

	agg := &Aggregate{Reasons: make(map[string]int)}
	agg.add(result)
	if agg.Skipped != 1 || agg.OK != 0 {
		t.Fatalf("skipped=%d ok=%d want 1, 0", agg.Skipped, agg.OK)
	}
}

func TestProcessFileFindsCredentialsAfterLongJunkPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "late.txt")
	input := strings.Repeat("documentation only\n", 5000) + "user123:pass1234\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, false)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Skipped || result.Kept != 1 {
		t.Fatalf("skipped=%t kept=%d want false, 1", result.Skipped, result.Kept)
	}
}

func TestUniqueCleanedPathSkipsExistingNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"input_cleaned.txt", "input_cleaned_1.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := uniqueCleanedPath(filepath.Join(dir, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "input_cleaned_2.txt"); got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
}
