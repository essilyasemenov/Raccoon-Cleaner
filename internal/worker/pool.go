package worker

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"cleaner/internal/discover"
	"cleaner/internal/metafile"
)

const progressInterval = time.Second

// Aggregate holds run-wide counters.
type Aggregate struct {
	OK           int
	Fail         int
	KeptLines    int
	RemovedLines int
	DupLines     int
	Reasons      map[string]int
	mu           sync.Mutex
}

func (a *Aggregate) add(r result) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.Err != nil {
		a.Fail++
		return
	}
	a.OK++
	a.KeptLines += r.Kept
	a.RemovedLines += r.Removed
	a.DupLines += r.Duplicates
	for k, v := range r.Reasons {
		a.Reasons[k] += v
	}
}

type progressItem struct {
	file string
	top  string
	at   time.Time
}

type progress struct {
	mu    sync.Mutex
	items map[string]progressItem // key = file path
	stop  chan struct{}
	done  chan struct{}
	logMu *sync.Mutex // shared with log output
	lines int         // last painted line count
}

func (p *progress) add(path, file, top string) {
	p.mu.Lock()
	p.items[path] = progressItem{file: file, top: top, at: time.Now()}
	p.mu.Unlock()
}

func (p *progress) del(path string) {
	p.mu.Lock()
	delete(p.items, path)
	p.mu.Unlock()
}

func (p *progress) start() {
	go func() {
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		defer close(p.done)
		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
				p.render()
			}
		}
	}()
}

func (p *progress) stopAndWait() {
	close(p.stop)
	<-p.done
	p.logMu.Lock()
	p.clearLocked()
	p.logMu.Unlock()
}

func (p *progress) clearLocked() {
	if p.lines <= 0 {
		fmt.Print("\r\033[K")
		return
	}
	// cursor is on first progress line
	for i := 0; i < p.lines; i++ {
		fmt.Print("\r\033[K")
		if i+1 < p.lines {
			fmt.Print("\n")
		}
	}
	if p.lines > 1 {
		fmt.Printf("\033[%dA", p.lines-1)
	}
	fmt.Print("\r")
	p.lines = 0
}

func (p *progress) render() {
	p.mu.Lock()
	items := make([]progressItem, 0, len(p.items))
	for _, it := range p.items {
		items = append(items, it)
	}
	p.mu.Unlock()

	p.logMu.Lock()
	defer p.logMu.Unlock()
	p.clearLocked()
	if len(items) == 0 {
		return
	}
	now := time.Now()
	for i, it := range items {
		sec := int(now.Sub(it.at).Seconds())
		line := fmt.Sprintf("обработка %s в папке %s - %d секунд", it.file, it.top, sec)
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print(line)
		fmt.Print("\033[K")
	}
	p.lines = len(items)
	if len(items) > 1 {
		fmt.Printf("\033[%dA", len(items)-1)
	}
	fmt.Print("\r")
}

// Run executes candidate jobs with GOMAXPROCS workers.
// A top folder is marked complete only when every file succeeds.
func Run(jobs []discover.Job, deleteSource bool, meta *metafile.Store) *Aggregate {
	agg := &Aggregate{Reasons: make(map[string]int)}
	workers := runtime.GOMAXPROCS(0)

	plan := make(map[string]int)
	for _, job := range jobs {
		plan[job.Top]++
	}
	var logMu sync.Mutex
	prog := &progress{
		items: make(map[string]progressItem),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		logMu: &logMu,
	}
	prog.start()
	defer prog.stopAndWait()

	type topState struct {
		finished atomic.Int32
		failed   atomic.Bool
	}
	states := make(map[string]*topState, len(plan))
	for top := range plan {
		states[top] = &topState{}
	}

	jobsCh := make(chan discover.Job)
	outputPaths := &outputIndex{}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobsCh {
				prog.add(j.FilePath, j.FileName, j.Top)
				r := processFileIndexed(j.FilePath, deleteSource, outputPaths)
				prog.del(j.FilePath)

				logMu.Lock()
				prog.clearLocked()
				switch {
				case errors.Is(r.Err, errUnsupportedFormat):
					fmt.Printf("[ОШИБКА ФОРМАТА] %s: %v\n", j.FileName, r.Err)
				case r.Err != nil:
					fmt.Printf("[ОШИБКА] %s: %v\n", j.FileName, r.Err)
				default:
					sourceDisposition := "сохранён"
					if deleteSource {
						sourceDisposition = "удалён"
					}
					fmt.Printf(
						"[OK] %s -> оставлено: %d, удалено: %d, дубли: %d; исходник %s\n",
						j.FileName,
						r.Kept,
						r.Removed,
						r.Duplicates,
						sourceDisposition,
					)
				}
				logMu.Unlock()
				agg.add(r)

				state := states[j.Top]
				if r.Err != nil {
					state.failed.Store(true)
				}
				if int(state.finished.Add(1)) == plan[j.Top] && !state.failed.Load() {
					if err := meta.Append(j.Top); err != nil {
						logMu.Lock()
						prog.clearLocked()
						fmt.Printf("[ОШИБКА] cleaned.txt: %v\n", err)
						logMu.Unlock()
						agg.add(result{Err: err})
					}
				}
			}
		}()
	}

	for _, j := range jobs {
		jobsCh <- j
	}
	close(jobsCh)
	wg.Wait()
	return agg
}

// PrintSummary prints final statistics.
func PrintSummary(agg *Aggregate, deleteMode bool) {
	fmt.Println("Готово.")
	fmt.Printf("Успешно обработано файлов: %d\n", agg.OK)
	fmt.Printf("Ошибок: %d\n", agg.Fail)
	fmt.Printf("Всего оставлено строк: %d\n", agg.KeptLines)
	fmt.Printf("Всего удалено строк: %d\n", agg.RemovedLines)
	fmt.Printf("Всего удалено дублей: %d\n", agg.DupLines)
	fmt.Println()
	fmt.Println("Причины удаления строк:")
	if len(agg.Reasons) == 0 {
		fmt.Println("  (нет)")
	} else {
		keys := make([]string, 0, len(agg.Reasons))
		for key := range agg.Reasons {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("  %s: %d\n", key, agg.Reasons[key])
		}
	}
	fmt.Println()
	if deleteMode {
		fmt.Println("Исходные TXT/CSV удалялись только при наличии непустого очищенного результата.")
	} else {
		fmt.Println("Режим save: исходные файлы не удалялись.")
	}
}
