package worker

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cleaner/internal/discover"
	"cleaner/internal/metafile"
)

func TestRunReportsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "students.csv")
	if err := os.WriteFile(path, []byte("StudentID,Name,Surname,Year\n1,Alice,Smith,2024\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	meta := metafile.New(filepath.Join(dir, "cleaned.txt"))
	agg := Run([]discover.Job{{Top: "students", FilePath: path, FileName: "students.csv"}}, true, meta)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if agg.Fail != 1 || !strings.Contains(string(output), "[ОШИБКА ФОРМАТА] students.csv") || !strings.Contains(string(output), "многоколоночная таблица") {
		t.Fatalf("fail=%d output=%q", agg.Fail, output)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	if meta.Contains("students") {
		t.Fatal("failed folder was marked complete")
	}
}

func TestRunRetriesFailedFolderWithoutDuplicateResults(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	bad := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(good, []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("id,name,year,department\n1,Alice,2024,Math\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := []discover.Job{
		{Top: "mixed", FilePath: good, FileName: "good.txt"},
		{Top: "mixed", FilePath: bad, FileName: "bad.csv"},
	}
	meta := metafile.New(filepath.Join(dir, "cleaned.txt"))
	first := Run(jobs, false, meta)
	if first.OK != 1 || first.Fail != 1 || meta.Contains("mixed") {
		t.Fatalf("first run=%+v marked=%t", first, meta.Contains("mixed"))
	}
	if err := os.WriteFile(bad, []byte("user456:pass5678\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := Run(jobs, false, meta)
	if second.OK != 2 || second.Fail != 0 || !meta.Contains("mixed") {
		t.Fatalf("second run=%+v marked=%t", second, meta.Contains("mixed"))
	}
	if _, err := os.Stat(filepath.Join(dir, "good_cleaned_1.txt")); !os.IsNotExist(err) {
		t.Fatalf("duplicate result exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bad_cleaned.csv")); err != nil {
		t.Fatalf("repaired file has no result: %v", err)
	}
}

func TestRunRetriesFailedFolderInDeleteMode(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	bad := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(good, []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("id,name,year,department\n1,Alice,2024,Math\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goodJob := discover.Job{Top: "mixed", FilePath: good, FileName: "good.txt"}
	badJob := discover.Job{Top: "mixed", FilePath: bad, FileName: "bad.csv"}
	meta := metafile.New(filepath.Join(dir, "cleaned.txt"))
	first := Run([]discover.Job{goodJob, badJob}, true, meta)
	if first.OK != 1 || first.Fail != 1 || meta.Contains("mixed") {
		t.Fatalf("first run=%+v marked=%t", first, meta.Contains("mixed"))
	}
	if _, err := os.Stat(good); !os.IsNotExist(err) {
		t.Fatalf("successful source still exists: %v", err)
	}
	if err := os.WriteFile(bad, []byte("user456:pass5678\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := Run([]discover.Job{badJob}, true, meta)
	if second.OK != 1 || second.Fail != 0 || !meta.Contains("mixed") {
		t.Fatalf("second run=%+v marked=%t", second, meta.Contains("mixed"))
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatalf("repaired source still exists: %v", err)
	}
}

func TestPrintSummarySortsReasons(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	PrintSummary(&Aggregate{Reasons: map[string]int{
		"early_url_full": 1,
		"artifact_field": 2,
	}}, true)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	a := strings.Index(string(output), "artifact_field: 2")
	b := strings.Index(string(output), "early_url_full: 1")
	if a < 0 || b < 0 || a >= b {
		t.Fatalf("reasons not sorted: %q", output)
	}
	if !strings.Contains(string(output), "при наличии непустого очищенного результата") {
		t.Fatalf("delete summary is stale: %q", output)
	}
}
