package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	wf "github.com/veggiemonk/workflow"
)

// Report is what the pipeline returns. Every step between the input and the
// report has its own output type, so no single struct carries the whole run.
type Report struct {
	Words  int
	Unique int
	Head   string
}

func main() {
	// A step declares its input type and its output type. Pure is for a
	// function that cannot fail.
	clean := wf.Pure("Clean", strings.TrimSpace)     // string   -> string
	split := wf.Pure("Split", strings.Fields)        // string   -> []string
	count := wf.Pure("Count", func(w []string) int { // []string -> int
		return len(w)
	})
	unique := wf.Pure("Unique", func(w []string) int { // []string -> int
		seen := make(map[string]struct{}, len(w))
		for _, word := range w {
			seen[strings.ToLower(word)] = struct{}{}
		}
		return len(seen)
	})
	head := wf.Pure("Head", func(w []string) string { // []string -> string
		return strings.Join(w[:min(3, len(w))], " ")
	})

	// Count, Unique and Head read the same words. Par runs two steps at the
	// same time and joins their results with a function you supply. The join
	// is typed, so the compiler will not let a result be dropped.
	analyse := count.
		Par(unique, func(n, u int) ([2]int, error) { return [2]int{n, u}, nil }).
		Par(head, func(nu [2]int, h string) (Report, error) {
			return Report{Words: nu[0], Unique: nu[1], Head: h}, nil
		})

	// Then links the steps. The chain changes type at every link.
	pipeline := clean.Then(split).Then(analyse) // Step[string, Report]

	fmt.Println("🚀 Starting basic workflow example...")
	fmt.Println()

	report, err := pipeline.Run(context.Background(), "  the quick brown fox jumps over the lazy dog  ")
	if err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	fmt.Println("📊 Results:")
	fmt.Printf("Words:  %d\n", report.Words)
	fmt.Printf("Unique: %d\n", report.Unique)
	fmt.Printf("Head:   %q\n", report.Head)

	fmt.Println("\n🌳 Pipeline structure:")
	fmt.Println(pipeline)
}
