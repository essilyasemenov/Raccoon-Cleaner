package encode

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestForEachReaderLine(t *testing.T) {
	cases := []struct {
		in string
		n  int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\r", 1},
		{"a\r\n", 1},
		{"a\n\n", 2},
		{"a\r\r", 2},
		{"a\nb", 2},
		{"a\r\nb", 2},
		{"a\rb", 2},
		{"a\n\nb", 3},
	}
	for _, tc := range cases {
		streamed := 0
		if err := ForEachReaderLine(strings.NewReader(tc.in), func(string) bool {
			streamed++
			return true
		}); err != nil {
			t.Fatal(err)
		}
		if streamed != tc.n {
			t.Fatalf("ForEachReaderLine(%q) count=%d want %d", tc.in, streamed, tc.n)
		}
	}
}

func TestOpenFileStreamsUTF8AndWindows1251(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "UTF-8 across buffer boundary",
			raw:  []byte(strings.Repeat("a", 64*1024+3) + "€"),
			want: strings.Repeat("a", 64*1024+3) + "€",
		},
		{
			name: "Windows-1251",
			raw:  mustEncode1251(t, "логин:пароль"),
			want: "логин:пароль",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.txt")
			if err := os.WriteFile(path, tc.raw, 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := OpenFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			got, err := io.ReadAll(f)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("decoded text mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func mustEncode1251(t *testing.T, value string) []byte {
	t.Helper()
	raw, err := charmap.Windows1251.NewEncoder().Bytes([]byte(value))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
