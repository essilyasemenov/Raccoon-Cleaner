package worker

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestAggregateAddCountsEmptyResult(t *testing.T) {
	agg := &Aggregate{Reasons: make(map[string]int)}
	agg.add(result{})
	if agg.OK != 1 || agg.EmptyResult != 1 {
		t.Fatalf("ok=%d empty=%d want 1, 1", agg.OK, agg.EmptyResult)
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
	}}, false)
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
}
