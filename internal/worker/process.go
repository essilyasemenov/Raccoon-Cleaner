package worker

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cleaner/internal/encode"
	"cleaner/internal/pipeline"
)

var errUnsupportedFormat = errors.New("неподходящий формат файла")

type result struct {
	Kept       int
	Removed    int // invalid + duplicates
	Duplicates int
	Reasons    map[string]int
	Err        error
}

func processFile(path string, deleteSource bool) result {
	return processFileIndexed(path, deleteSource, &outputIndex{})
}

func processFileIndexed(path string, deleteSource bool, index *outputIndex) result {
	var res result
	input, err := encode.OpenFile(path)
	if err != nil {
		res.Err = err
		return res
	}
	defer input.Close()

	outPath, existing, err := index.uniqueCleanedPath(path)
	if err != nil {
		res.Err = err
		return res
	}
	output, err := os.CreateTemp(filepath.Dir(outPath), filepath.Base(outPath)+".*.tmp")
	if err != nil {
		res.Err = err
		return res
	}
	tmpPath := output.Name()
	defer os.Remove(tmpPath)

	p := pipeline.NewProcessor()
	writer := bufio.NewWriter(output)
	var writeErr error
	lineCount := 0
	err = encode.ForEachReaderLine(input, func(line string) bool {
		lineCount++
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
	if err != nil {
		res.Err = err
		return res
	}

	st := p.Stats
	validRows := st.Kept + st.Duplicates
	if st.WideRows >= 2 && st.WideRows >= validRows {
		allowed := false
		if validRows > 0 {
			allowed, err = middleHasSourceMarkers(path, lineCount)
			if err != nil {
				res.Err = err
				return res
			}
		}
		if !allowed {
			res.Err = fmt.Errorf("%w: многоколоночная таблица (%d строк)", errUnsupportedFormat, st.WideRows)
			return res
		}
	}
	if validRows == 0 {
		res.Err = fmt.Errorf("%w: нет допустимых записей login:password", errUnsupportedFormat)
		return res
	}
	matched, err := matchesAnyFile(tmpPath, existing)
	if err != nil {
		res.Err = err
		return res
	}
	if !matched {
		if err := encode.PublishFile(tmpPath, outPath); err != nil {
			res.Err = err
			return res
		}
		index.record(outPath)
	}

	res.Kept = st.Kept
	res.Duplicates = st.Duplicates
	res.Removed = st.Invalid + st.Duplicates
	res.Reasons = st.Reasons

	if deleteSource {
		if err := os.Remove(path); err != nil {
			res.Err = fmt.Errorf("write ok but delete source: %w", err)
			return res
		}
	}
	return res
}

func middleHasSourceMarkers(path string, totalLines int) (bool, error) {
	window := min(totalLines, 100)
	if window == 0 {
		return false, nil
	}
	input, err := encode.OpenFile(path)
	if err != nil {
		return false, err
	}
	defer input.Close()

	start := (totalLines - window) / 2
	index, matches := 0, 0
	err = encode.ForEachReaderLine(input, func(line string) bool {
		if index >= start && index < start+window {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "http") || strings.Contains(lower, "android") {
				matches++
			}
		}
		index++
		return index < start+window
	})
	if err != nil {
		return false, err
	}
	return matches*10 > window*7, nil
}

func fileDigest(path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	f, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return digest, err
	}
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

func matchesAnyFile(path string, candidates []string) (bool, error) {
	if len(candidates) == 0 {
		return false, nil
	}
	digest, err := fileDigest(path)
	if err != nil {
		return false, err
	}
	var firstErr error
	for _, candidate := range candidates {
		candidateDigest, err := fileDigest(candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read existing result %s: %w", candidate, err)
			}
			continue
		}
		if candidateDigest == digest {
			return true, nil
		}
	}
	return false, firstErr
}

type outputEntry struct {
	path    string
	regular bool
}

type outputIndex struct {
	mu   sync.Mutex
	dirs map[string]map[string]map[int]outputEntry
}

func outputKey(name string) (string, int, bool) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	mark := strings.LastIndex(strings.ToLower(stem), "_cleaned")
	if mark < 0 {
		return "", 0, false
	}
	suffix := stem[mark+len("_cleaned"):]
	index := 0
	if suffix != "" {
		if !strings.HasPrefix(suffix, "_") {
			return "", 0, false
		}
		number := suffix[1:]
		parsed, err := strconv.Atoi(number)
		if err != nil || parsed < 1 || strconv.Itoa(parsed) != number {
			return "", 0, false
		}
		index = parsed
	}
	return strings.ToLower(stem[:mark] + ext), index, true
}

func (o *outputIndex) uniqueCleanedPath(src string) (string, []string, error) {
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	key := strings.ToLower(stem + ext)

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.dirs == nil {
		o.dirs = make(map[string]map[string]map[int]outputEntry)
	}
	groups, loaded := o.dirs[dir]
	if !loaded {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", nil, err
		}
		groups = make(map[string]map[int]outputEntry)
		for _, entry := range entries {
			entryKey, number, ok := outputKey(entry.Name())
			if !ok {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return "", nil, err
			}
			if groups[entryKey] == nil {
				groups[entryKey] = make(map[int]outputEntry)
			}
			groups[entryKey][number] = outputEntry{filepath.Join(dir, entry.Name()), info.Mode().IsRegular()}
		}
		o.dirs[dir] = groups
	}
	if groups[key] == nil {
		groups[key] = make(map[int]outputEntry)
	}
	occupied := groups[key]
	var existing []string
	for _, entry := range occupied {
		if entry.regular {
			existing = append(existing, entry.path)
		}
	}
	sort.Strings(existing)
	for i := 0; ; i++ {
		if _, found := occupied[i]; found {
			continue
		}
		suffix := "_cleaned"
		if i > 0 {
			suffix = fmt.Sprintf("_cleaned_%d", i)
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
		if info, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, existing, nil
		} else if err != nil {
			return "", nil, err
		} else {
			occupied[i] = outputEntry{candidate, info.Mode().IsRegular()}
			if info.Mode().IsRegular() {
				existing = append(existing, candidate)
			}
		}
	}
}

func (o *outputIndex) record(path string) {
	key, number, ok := outputKey(filepath.Base(path))
	if !ok {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if groups := o.dirs[filepath.Dir(path)]; groups != nil {
		groups[key][number] = outputEntry{path, true}
	}
}
