package worker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cleaner/internal/encode"
	"cleaner/internal/pipeline"
)

type result struct {
	Kept       int
	Removed    int // invalid + duplicates
	Duplicates int
	Skipped    bool
	Reasons    map[string]int
	Err        error
}

func processFile(path string, deleteSource bool) result {
	var res result

	suitable, err := isProcessableFile(path)
	if err != nil {
		res.Err = err
		return res
	}
	if !suitable {
		res.Skipped = true
		return res
	}

	input, err := encode.OpenFile(path)
	if err != nil {
		res.Err = err
		return res
	}
	defer input.Close()

	outPath, err := uniqueCleanedPath(path)
	if err != nil {
		res.Err = err
		return res
	}
	tmpPath := outPath + ".tmp"
	output, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		res.Err = err
		return res
	}
	defer os.Remove(tmpPath)

	p := pipeline.NewProcessor()
	writer := bufio.NewWriter(output)
	var writeErr error
	err = encode.ForEachReaderLine(input, func(line string) bool {
		if cleaned, ok := p.ProcessLine(line); ok {
			_, writeErr = writer.WriteString(cleaned + "\n")
			return writeErr == nil
		}
		return true
	})
	if closeErr := input.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = writeErr
	}
	if err == nil {
		err = writer.Flush()
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = encode.ReplaceFile(tmpPath, outPath)
	}
	if err != nil {
		res.Err = err
		return res
	}

	st := p.Stats
	res.Kept = st.Kept
	res.Duplicates = st.Duplicates
	res.Removed = st.Invalid + st.Duplicates
	res.Reasons = st.Reasons

	if res.Kept == 0 {
		return res
	}

	if deleteSource {
		if err := os.Remove(path); err != nil {
			res.Err = fmt.Errorf("write ok but delete source: %w", err)
			return res
		}
	}
	return res
}

func isProcessableFile(path string) (bool, error) {
	input, err := encode.OpenFile(path)
	if err != nil {
		return false, err
	}
	defer input.Close()

	p := pipeline.NewProcessor()
	suitable := false
	err = encode.ForEachReaderLine(input, func(line string) bool {
		_, suitable = p.ProcessLine(line)
		return !suitable
	})
	return suitable, err
}

func uniqueCleanedPath(src string) (string, error) {
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for i := 0; ; i++ {
		suffix := "_cleaned"
		if i > 0 {
			suffix = fmt.Sprintf("_cleaned_%d", i)
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}
