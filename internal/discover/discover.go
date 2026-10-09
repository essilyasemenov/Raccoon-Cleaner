package discover

import (
	"os"
	"path/filepath"
	"strings"
)

// Job is one candidate file under a top-folder.
type Job struct {
	Top      string // top-folder name
	FilePath string
	FileName string
}

// TopFolders lists immediate child directories of root (not files).
func TopFolders(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var tops []string
	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(root, name)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			tops = append(tops, name)
		}
	}
	return tops, nil
}

// WalkTXTCSV finds .txt/.csv (CI) under topPath, following dir symlinks/junctions.
// Skips names containing "_cleaned" before extension (F18).
func WalkTXTCSV(topName, topPath string) ([]Job, error) {
	var jobs []Job
	visited := make(map[string]struct{})

	var walk func(dir string) error
	walk = func(dir string) error {
		resolved, err := filepath.Abs(dir)
		if err != nil {
			resolved = dir
		}
		if real, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = real
		}
		if _, ok := visited[resolved]; ok {
			return nil
		}
		visited[resolved] = struct{}{}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			name := e.Name()
			full := filepath.Join(dir, name)
			info, err := os.Lstat(full)
			if err != nil {
				continue
			}

			mode := info.Mode()
			if mode&os.ModeSymlink != 0 {
				info, err = os.Stat(full)
				if err != nil {
					continue
				}
			}

			if info.IsDir() {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}

			if isCandidate(name) {
				jobs = append(jobs, Job{
					Top:      topName,
					FilePath: full,
					FileName: name,
				})
			}
		}
		return nil
	}

	if err := walk(topPath); err != nil {
		// Do not return partial jobs: caller must not plan/meta an incomplete top.
		return nil, err
	}
	return jobs, nil
}

func isCandidate(name string) bool {
	if strings.EqualFold(name, "readme.txt") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".txt" && ext != ".csv" {
		return false
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.Contains(stem, "_cleaned") {
		return false
	}
	return true
}
