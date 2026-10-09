package encode

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

type decodedFile struct {
	io.Reader
	io.Closer
}

// OpenFile opens UTF-8 or Windows-1251 text without loading it all into memory.
func OpenFile(path string) (io.ReadCloser, error) {
	isUTF8, err := validUTF8File(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(f)
	if isUTF8 {
		if bom, _ := reader.Peek(3); bytes.Equal(bom, []byte{0xEF, 0xBB, 0xBF}) {
			_, _ = reader.Discard(3)
		}
		return &decodedFile{Reader: reader, Closer: f}, nil
	}
	return &decodedFile{
		Reader: transform.NewReader(reader, charmap.Windows1251.NewDecoder()),
		Closer: f,
	}, nil
}

func validUTF8File(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 64*1024+utf8.UTFMax)
	carry := 0
	for {
		n, readErr := f.Read(buf[carry:])
		data := buf[:carry+n]
		if readErr != nil {
			if readErr == io.EOF {
				return utf8.Valid(data), nil
			}
			return false, readErr
		}

		cut := len(data)
		start := cut
		for start > 0 && cut-start < utf8.UTFMax && !utf8.RuneStart(data[start-1]) {
			start--
		}
		if start > 0 && !utf8.FullRune(data[start-1:]) {
			cut = start - 1
		}
		if !utf8.Valid(data[:cut]) {
			return false, nil
		}
		carry = copy(buf, data[cut:])
	}
}

// ForEachReaderLine reads LF, CRLF, or CR-delimited lines one at a time.
func ForEachReaderLine(r io.Reader, fn func(line string) bool) error {
	reader := bufio.NewReader(r)
	trailingLF := false
	for {
		chunk, err := reader.ReadString('\n')
		if len(chunk) > 0 {
			trailingLF = strings.HasSuffix(chunk, "\n")
			if trailingLF {
				chunk = strings.TrimSuffix(chunk, "\n")
				chunk = strings.TrimSuffix(chunk, "\r")
			}
			if !forEachCRLine(chunk, fn) {
				return nil
			}
		}
		if err != nil {
			if err == io.EOF {
				if len(chunk) == 0 && trailingLF {
					_ = fn("")
				}
				return nil
			}
			return err
		}
	}
}

func forEachCRLine(s string, fn func(line string) bool) bool {
	for {
		line, rest, found := strings.Cut(s, "\r")
		if !fn(line) {
			return false
		}
		if !found {
			return true
		}
		s = rest
	}
}
