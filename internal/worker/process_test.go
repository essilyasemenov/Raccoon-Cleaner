package worker

import (
	"errors"
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
		`"column1","column2","column3","column4"`,
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
		`"user@example.com","pass1234"`,
	}, "\r")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, false)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Kept != 3 {
		t.Fatalf("kept=%d reasons=%v want 3", result.Kept, result.Reasons)
	}
	cleaned, err := os.ReadFile(filepath.Join(dir, "quoted_cleaned.csv"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"QUARTO","` + bcrypt + `"` + "\n" + `"user123","pa,ss""word"` + "\n" + `"user@example.com","pass1234"` + "\n"
	if string(cleaned) != want {
		t.Fatalf("cleaned=%q want %q", cleaned, want)
	}
}

func TestProcessFileKeepsMultipleArgon2Hashes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "argon2.txt")
	first := "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaFZhbHVl"
	second := "$argon2i$v=19$m=32768,t=2,p=2$c2FsdHNhbHQ$aGFzaFZhbHVl"
	input := "user123:" + first + "\nuser456:" + second + "\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, false)
	if result.Err != nil || result.Kept != 2 {
		t.Fatalf("result=%+v want two hashes", result)
	}
	want := `"user123","` + first + `"` + "\n" + `"user456","` + second + `"` + "\n"
	cleaned, err := os.ReadFile(filepath.Join(dir, "argon2_cleaned.txt"))
	if err != nil || string(cleaned) != want {
		t.Fatalf("cleaned=%q err=%v want %q", cleaned, err, want)
	}
}

func TestProcessFileRejectsUnsuitableInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	input := "documentation\nhttps://example.com\n{\"enabled\":true}\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes_cleaned.txt")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}

	agg := &Aggregate{Reasons: make(map[string]int)}
	agg.add(result)
	if agg.Fail != 1 || agg.OK != 0 {
		t.Fatalf("fail=%d ok=%d want 1, 0", agg.Fail, agg.OK)
	}
}

func TestProcessFileRejectsWideTableDespiteCredentialLikeRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "students.csv")
	input := strings.Join([]string{
		`"StudentID","Name","Surname","Year","Department"`,
		`"1","Alice","Smith","2024","Physics"`,
		`"2","Bob","Jones","2024","Math"`,
		`"3","Carol","Evans","2024","Chemistry"`,
		"user123:pass1234",
	}, "\n")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "students_cleaned.csv")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}
}

func TestProcessFileRejectsOversizeWideTableWithCredentialLikeRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "students.csv")
	wide := `"1","Alice","Smith","` + strings.Repeat("x", 4096) + `"`
	input := wide + "\n" + wide + "\nuser123:pass1234\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != input {
		t.Fatalf("source changed: %q, err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "students_cleaned.csv")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}
}

func TestProcessFileRejectsWideTableWithVaryingWidths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "varying.csv")
	input := strings.Join([]string{
		"id,name,year,department",
		"1,Alice,2024,Physics,Campus",
		"2,Bob,2024,Math,Campus,Group",
		"user123:pass1234",
	}, "\n")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "varying_cleaned.csv")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}
}

func TestProcessFileRejectsTableWithColonInFirstColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orders.csv")
	input := strings.Join([]string{
		"id,name,team,status",
		"product123:2024,Alice,Sales,Active",
		"product456:2024,Bob,Support,Active",
		"product789:2024,Carol,Finance,Active",
	}, "\n")
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("source was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "orders_cleaned.csv")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}
}

func TestProcessFileRejectsHeaderlessTableWithColonInFirstColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orders.csv")
	input := "product123:2024,Alice,Sales,Active\nproduct456:2024,Bob,Support,Active\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != input {
		t.Fatalf("source changed: %q, err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "orders_cleaned.csv")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}
}

func TestProcessFileRejectsThreeColumnTableWithColonInFirstColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orders.csv")
	input := "product123:2024,Alice,Active\nproduct456:2024,Bob,Active\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, true)
	if !errors.Is(result.Err, errUnsupportedFormat) {
		t.Fatalf("error=%v want unsupported format", result.Err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != input {
		t.Fatalf("source changed: %q, err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "orders_cleaned.csv")); !os.IsNotExist(err) {
		t.Fatalf("cleaned file exists: %v", err)
	}
}

func TestProcessFileKeepsColonPasswordWithCommasWithoutTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pairs.txt")
	if err := os.WriteFile(path, []byte("user123:pa,ss,wo,rd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, false)
	if result.Err != nil || result.Kept != 1 {
		t.Fatalf("result=%+v want one credential", result)
	}
	cleaned, err := os.ReadFile(filepath.Join(dir, "pairs_cleaned.txt"))
	if err != nil || string(cleaned) != `"user123","pa,ss,wo,rd"`+"\n" {
		t.Fatalf("cleaned=%q err=%v", cleaned, err)
	}
}

func TestProcessFileDoesNotTruncateExistingTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	stale := filepath.Join(dir, "input_cleaned.txt.tmp")
	if err := os.WriteFile(path, []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("older temporary data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, false)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	got, err := os.ReadFile(stale)
	if err != nil || string(got) != "older temporary data\n" {
		t.Fatalf("existing temp changed: %q, %v", got, err)
	}
}

func TestProcessFileMiddleMarkersIgnoreTerminalNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.csv")
	lines := []string{
		"user123:pass1234",
		`"1","HTTP://example.com","Smith","2024"`,
		`"1","android","Smith","2024"`,
		`"1","HTTP://example.com","Smith","2024"`,
		`"1","android","Smith","2024"`,
		`"1","HTTP://example.com","Smith","2024"`,
		`"1","Alice","Smith","2024"`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := processFile(path, false)
	if result.Err != nil || result.Kept != 1 {
		t.Fatalf("result=%+v want one credential", result)
	}
}

func TestProcessFileMiddleMarkersAllowMixedWideFile(t *testing.T) {
	tests := []struct {
		name     string
		markers  int
		outside  bool
		noCred   bool
		wantKeep bool
	}{
		{name: "71 central markers", markers: 71, wantKeep: true},
		{name: "70 central markers", markers: 70},
		{name: "outside marker does not count", markers: 70, outside: true},
		{name: "markers without credentials", markers: 71, noCred: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "mixed.csv")
			lines := make([]string, 500)
			lines[0] = "user123:pass1234"
			for i := 1; i < len(lines); i++ {
				lines[i] = `"1","Alice","Smith","2024"`
			}
			if tt.noCred {
				lines[0] = `"0","Alice","Smith","2024"`
			}
			for i := 200; i < 200+tt.markers; i++ {
				if i%2 == 0 {
					lines[i] = `"1","HTTP://example.com","Smith","2024"`
				} else {
					lines[i] = `"1","ANDROID","Smith","2024"`
				}
			}
			if tt.outside {
				lines[199] = `"1","http://example.com","Smith","2024"`
			}
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				t.Fatal(err)
			}

			result := processFile(path, true)
			cleanedPath := filepath.Join(dir, "mixed_cleaned.csv")
			if tt.wantKeep {
				if result.Err != nil || result.Kept != 1 {
					t.Fatalf("result=%+v want one credential", result)
				}
				cleaned, err := os.ReadFile(cleanedPath)
				if err != nil || string(cleaned) != `"user123","pass1234"`+"\n" {
					t.Fatalf("cleaned=%q err=%v", cleaned, err)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("source still exists: %v", err)
				}
			} else {
				if !errors.Is(result.Err, errUnsupportedFormat) {
					t.Fatalf("error=%v want unsupported format", result.Err)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("source was not preserved: %v", err)
				}
				if _, err := os.Stat(cleanedPath); !os.IsNotExist(err) {
					t.Fatalf("cleaned file exists: %v", err)
				}
			}
		})
	}
}

func TestMiddleHasSourceMarkersShortFile(t *testing.T) {
	tests := []struct {
		name    string
		markers int
		want    bool
	}{
		{name: "seven of ten", markers: 7},
		{name: "eight of ten", markers: 8, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "short.txt")
			lines := make([]string, 10)
			for i := range lines {
				lines[i] = "plain text"
				if i < tt.markers {
					lines[i] = "android http"
				}
			}
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := middleHasSourceMarkers(path, len(lines))
			if err != nil || got != tt.want {
				t.Fatalf("middleHasSourceMarkers=%v, %v want %v", got, err, tt.want)
			}
		})
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
	if result.Kept != 1 {
		t.Fatalf("kept=%d want 1", result.Kept)
	}
}

func TestUniqueCleanedPathSkipsExistingNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"input_cleaned.txt", "input_cleaned_1.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, existing, err := (&outputIndex{}).uniqueCleanedPath(filepath.Join(dir, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "input_cleaned_2.txt"); got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
	if len(existing) != 2 {
		t.Fatalf("existing=%v want two paths", existing)
	}
}

func TestProcessFileReusesMatchingOutputAfterCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	baseResult := filepath.Join(dir, "input_cleaned.txt")
	index := &outputIndex{}
	if err := os.WriteFile(path, []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baseResult, []byte("preexisting result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := processFileIndexed(path, false, index); result.Err != nil {
		t.Fatal(result.Err)
	}
	if result := processFileIndexed(path, false, index); result.Err != nil {
		t.Fatal(result.Err)
	}
	if _, err := os.Stat(filepath.Join(dir, "input_cleaned_2.txt")); !os.IsNotExist(err) {
		t.Fatalf("duplicate result exists: %v", err)
	}
	if got, err := os.ReadFile(baseResult); err != nil || string(got) != "preexisting result\n" {
		t.Fatalf("preexisting result changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "input_cleaned_1.txt")); err != nil || string(got) != `"user123","pass1234"`+"\n" {
		t.Fatalf("cleaned result=%q, %v", got, err)
	}
}

func TestProcessFileReusesMatchingOutputAcrossNumberingGap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("user123:pass1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input_cleaned.txt"), []byte("older result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input_cleaned_2.txt"), []byte(`"user123","pass1234"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := processFile(path, false); result.Err != nil {
		t.Fatal(result.Err)
	}
	if _, err := os.Stat(filepath.Join(dir, "input_cleaned_1.txt")); !os.IsNotExist(err) {
		t.Fatalf("duplicate result exists: %v", err)
	}
}

func TestMatchesAnyFileFindsMatchAfterMissingCandidate(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current.txt")
	matching := filepath.Join(dir, "matching.txt")
	contents := []byte(`"user123","pass1234"` + "\n")
	for _, path := range []string{current, matching} {
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	matched, err := matchesAnyFile(current, []string{filepath.Join(dir, "missing.txt"), matching})
	if err != nil || !matched {
		t.Fatalf("matched=%v err=%v want true, nil", matched, err)
	}
}
