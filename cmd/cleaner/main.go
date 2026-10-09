package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cleaner/internal/discover"
	"cleaner/internal/metafile"
	"cleaner/internal/worker"
)

const (
	rootPath     = "C:/Source/combo"
	metafilePath = "C:/Source/combo/cleaned.txt"
)

func main() {
	os.Exit(run())
}

func run() int {
	deleteFlag := flag.Bool("delete", false, "delete source files after processing (no prompt)")
	saveFlag := flag.Bool("save", false, "keep source files after processing (no prompt)")
	flag.Parse()

	if *deleteFlag && *saveFlag {
		fmt.Fprintln(os.Stderr, "fatal: flags -delete and -save are mutually exclusive")
		return 2
	}

	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "fatal: ROOT %s does not exist or is not a directory\n", rootPath)
		return 2
	}

	meta := metafile.New(metafilePath)
	if err := meta.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: read metafile: %v\n", err)
		return 2
	}

	deleteSource := *deleteFlag
	if !*deleteFlag && !*saveFlag {
		deleteSource, err = promptLoop()
		if err != nil {
			fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
			return 2
		}
	}

	tops, err := discover.TopFolders(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: list top folders: %v\n", err)
		return 2
	}

	var jobs []discover.Job
	preFail := 0 // incomplete discovery (walk) errors

	for _, top := range tops {
		if meta.Contains(top) {
			continue
		}
		topPath := filepath.Join(rootPath, top)
		candidates, err := discover.WalkTXTCSV(top, topPath)
		if err != nil {
			// Incomplete walk must not plan/meta the top — unvisited files would be skipped forever.
			fmt.Fprintf(os.Stderr, "[ОШИБКА] walk %s: %v\n", top, err)
			preFail++
			continue
		}
		if len(candidates) == 0 {
			continue
		}
		jobs = append(jobs, candidates...)
	}

	if len(jobs) == 0 {
		fmt.Println("Нет файлов для обработки.")
		if preFail > 0 {
			return 1
		}
		return 0
	}

	agg := worker.Run(jobs, deleteSource, meta)

	worker.PrintSummary(agg, deleteSource)
	if agg.Fail > 0 || preFail > 0 {
		return 1
	}
	return 0
}

func promptLoop() (bool, error) {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Удалять исходные файлы после обработки? (1/delete = удалить, 2/save = оставить): ")
		line, err := in.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		switch ans {
		case "1", "delete":
			return true, nil
		case "2", "save":
			return false, nil
		}
		// F7/F29: invalid or EOF → re-ask forever (never fatal for lack of answer)
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		if errors.Is(err, io.EOF) {
			time.Sleep(200 * time.Millisecond) // closed stdin would otherwise busy-loop
		}
	}
}
