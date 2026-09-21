package main

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	wf "github.com/veggiemonk/workflow"
)

// DataRecord is one row of input.
type DataRecord struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
	Type  string `json:"type"`
}

// ProcessedRecord is one row of output.
type ProcessedRecord struct {
	ID             string    `json:"id"`
	OriginalValue  int       `json:"original_value"`
	ProcessedValue int       `json:"processed_value"`
	ProcessedAt    time.Time `json:"processed_at"`
	Category       string    `json:"category"`
}

// Quality is the verdict on the input, with the input it judged.
type Quality struct {
	Records []DataRecord `json:"-"`
	Invalid int          `json:"invalid"`
	Ratio   float64      `json:"invalid_ratio"`
}

// Report is what the pipeline returns.
type Report struct {
	Quality        Quality                  `json:"quality"`
	Records        []ProcessedRecord        `json:"records"`
	Categories     map[string]int           `json:"categories"`
	TotalValue     int                      `json:"total_value"`
	StepDurations  map[string]time.Duration `json:"step_durations"`
	ProcessingTime time.Duration            `json:"processing_time"`
	Throughput     float64                  `json:"throughput_per_sec"`
}

// metrics collects what the timed middleware measures. A step is a value with
// no state of its own, so the state lives here, where you can read it.
type metrics struct {
	mu    sync.Mutex
	steps map[string]time.Duration
}

func (m *metrics) record(name string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.steps == nil {
		m.steps = make(map[string]time.Duration)
	}
	m.steps[name] += d
}

func (m *metrics) snapshot() map[string]time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.steps)
}

// timed is a custom middleware. Middleware[I, O] is a plain function from a
// step to a step, so writing one needs nothing from the library.
func timed[I, O any](m *metrics) wf.Middleware[I, O] {
	return func(next wf.Step[I, O]) wf.Step[I, O] {
		return wf.Func(next.Name(), func(ctx context.Context, in I) (O, error) {
			start := time.Now()
			out, err := next.Run(ctx, in)
			m.record(next.Name(), time.Since(start))
			return out, err
		})
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	m := &metrics{}

	// 1. Judge the input, and keep it: the gate after this needs both.
	inspect := wf.Pure("Inspect", func(records []DataRecord) Quality {
		invalid := 0
		for _, r := range records {
			if !valid(r) {
				invalid++
			}
		}
		return Quality{
			Records: records,
			Invalid: invalid,
			Ratio:   float64(invalid) / float64(max(len(records), 1)),
		}
	}).Use(timed[[]DataRecord, Quality](m))

	// 2. Clean the input when too much of it is bad. Both branches of If have
	// the same shape, so the gate returns one type whichever way it goes.
	gate := wf.If("Gate",
		func(_ context.Context, q Quality) bool { return q.Ratio < 0.1 },
		wf.Pure("Keep", func(q Quality) Quality { return q }),
		wf.Pure("Clean", func(q Quality) Quality {
			q.Records = slices.DeleteFunc(slices.Clone(q.Records), func(r DataRecord) bool {
				return !valid(r)
			})
			return q
		}),
	)

	// 3. Process the records, at most 8 at a time. Each applies one step to
	// every element of a slice; it is a function, not a method, because a
	// method would make the compiler instantiate Step[[][]I, [][]O] without
	// end.
	batch := wf.Each(8, transform.Use(timed[DataRecord, ProcessedRecord](m)))

	process := wf.Func("Process", func(ctx context.Context, q Quality) (Report, error) {
		start := time.Now()
		records, err := batch.Run(ctx, q.Records)
		if err != nil {
			return Report{}, err
		}
		return summarise(q, records, time.Since(start)), nil
	}).Log(logger)

	pipeline := inspect.Then(gate).Then(process).WithID()

	fmt.Println("🚀 Starting advanced data processing pipeline...")

	start := time.Now()
	report, err := pipeline.Run(context.Background(), generateSampleData(50))
	if err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}
	report.StepDurations = m.snapshot()

	fmt.Printf("\n⏱️  Total pipeline duration: %v\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("📈 Records processed: %d of %d (%d dropped as invalid)\n",
		len(report.Records), len(report.Quality.Records)+report.Quality.Invalid, report.Quality.Invalid)
	fmt.Printf("⚡ Throughput: %.0f records/sec\n", report.Throughput)

	fmt.Println("\n🏷️  Categories:")
	for _, name := range slices.Sorted(maps.Keys(report.Categories)) {
		fmt.Printf("  %-12s %d\n", name, report.Categories[name])
	}

	fmt.Println("\n⏱️  Step durations:")
	for _, name := range slices.Sorted(maps.Keys(report.StepDurations)) {
		fmt.Printf("  %-12s %v\n", name, report.StepDurations[name].Round(time.Microsecond))
	}

	// 4. Every branch of Each runs to the end, and Each returns every error
	// joined together. Nothing is dropped in silence.
	fmt.Println("\n🧪 The same batch without the gate:")
	if _, err := batch.Run(context.Background(), generateSampleData(8)); err != nil {
		for line := range strings.SplitSeq(err.Error(), "\n") {
			fmt.Println("  " + line)
		}
	}

	if err := exportResults(report); err != nil {
		fmt.Printf("\n⚠️  Failed to export results: %v\n", err)
	} else {
		fmt.Println("\n💾 Results exported to results.json")
	}

	fmt.Println("\n🌳 Pipeline structure:")
	fmt.Println(pipeline)
}

func valid(r DataRecord) bool { return r.Value >= 0 && r.Type != "" }

// transform is the work itself: one record in, one record out. It fails on a
// record it cannot classify, instead of returning a zero value.
var transform = wf.Func("Transform", func(ctx context.Context, r DataRecord) (ProcessedRecord, error) {
	if !valid(r) {
		return ProcessedRecord{}, fmt.Errorf("record %s: no type", r.ID)
	}
	select {
	case <-time.After(2 * time.Millisecond):
	case <-ctx.Done():
		return ProcessedRecord{}, ctx.Err()
	}

	out := ProcessedRecord{
		ID:            r.ID,
		OriginalValue: r.Value,
		ProcessedAt:   time.Now(),
	}
	switch {
	case r.Type == "special":
		out.Category, out.ProcessedValue = "special", r.Value*3
	case r.Value > 500:
		out.Category, out.ProcessedValue = "high-value", r.Value*2
	default:
		out.Category, out.ProcessedValue = "low-value", r.Value+100
	}
	return out, nil
})

func summarise(q Quality, records []ProcessedRecord, took time.Duration) Report {
	report := Report{
		Quality:        q,
		Records:        records,
		Categories:     make(map[string]int),
		ProcessingTime: took,
	}
	for _, r := range records {
		report.Categories[r.Category]++
		report.TotalValue += r.ProcessedValue
	}
	if took > 0 {
		report.Throughput = float64(len(records)) / took.Seconds()
	}
	return report
}

func generateSampleData(count int) []DataRecord {
	data := make([]DataRecord, count)
	types := []string{"normal", "special", "premium", ""}
	for i := range count {
		data[i] = DataRecord{
			ID:    fmt.Sprintf("record-%d", i),
			Value: (i * 37) % 1000, // pseudo-random values
			Type:  types[i%len(types)],
		}
	}
	return data
}

// exportOptions holds the JSON behaviour that the export needs.
//
//   - json/v2 has no default representation for a time.Duration. The option
//     keeps the v1 form, a number of nanoseconds.
//   - v2 writes the members of a map in the order it reads them. The report
//     goes to a file that a person reads and a diff compares, so the option
//     keeps the sorted order that v1 gave.
var exportOptions = json.JoinOptions(
	jsontext.WithIndent("  "),
	json.Deterministic(true),
	jsonv1.FormatDurationAsNano(true),
)

func exportResults(report Report) error {
	// check if results.json already exists
	if _, err := os.Stat("results.json"); err == nil {
		return fmt.Errorf("results.json already exists, please remove it before exporting")
	}
	file, err := os.Create("results.json")
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.MarshalWrite(file, report, exportOptions); err != nil {
		return err
	}
	// MarshalWrite writes no trailing newline; Encoder.Encode did.
	_, err = file.WriteString("\n")
	return err
}
