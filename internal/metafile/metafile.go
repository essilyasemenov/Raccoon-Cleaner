package metafile

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"cleaner/internal/encode"
)

// Store manages cleaned.txt top-folder names.
type Store struct {
	path string
	mu   sync.Mutex
	set  map[string]struct{}
}

func New(path string) *Store {
	return &Store{
		path: path,
		set:  make(map[string]struct{}),
	}
}

// Load reads existing metafile; missing file → empty set.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name != "" {
			s.set[name] = struct{}{}
		}
	}
	return sc.Err()
}

func (s *Store) Contains(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.set[name]
	return ok
}

// Append adds top-folder name atomically (rewrite whole file).
func (s *Store) Append(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.set[name]; ok {
		return nil
	}

	names := make([]string, 0, len(s.set)+1)
	for n := range s.set {
		names = append(names, n)
	}
	names = append(names, name)
	sort.Strings(names)
	contents := strings.Join(names, "\n") + "\n"

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "cleaned-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := encode.ReplaceFile(tmpPath, s.path); err != nil {
		return err
	}
	s.set[name] = struct{}{}
	return nil
}
