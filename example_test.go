package workflow_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	workflow "github.com/veggiemonk/workflow"
)

// A pipeline changes type at every link. The compiler checks each join.
func Example() {
	split := workflow.Pure("Split", func(s string) []string { return strings.Fields(s) })
	total := workflow.Func("Total", func(_ context.Context, words []string) (int, error) {
		sum := 0
		for _, w := range words {
			n, err := strconv.Atoi(w)
			if err != nil {
				return 0, fmt.Errorf("bad number %q: %w", w, err)
			}
			sum += n
		}
		return sum, nil
	})

	pipeline := split.Then(total) // Step[string, int]

	got, err := pipeline.Run(context.Background(), "1 2 3 4")
	fmt.Println(got, err)

	_, err = pipeline.Run(context.Background(), "1 two 3")
	fmt.Println(err)
	// Output:
	// 10 <nil>
	// bad number "two": strconv.Atoi: parsing "two": invalid syntax
}

// Par runs two steps on one input and joins the results with your function.
func ExampleStep_Par() {
	length := workflow.Pure("Length", func(s string) int { return len(s) })
	upper := workflow.Pure("Upper", strings.ToUpper)

	both := length.Par(upper, func(n int, s string) (string, error) {
		return fmt.Sprintf("%s (%d)", s, n), nil
	})

	out, _ := both.Run(context.Background(), "hello")
	fmt.Println(out)
	// Output: HELLO (5)
}

// Fan runs any number of steps that share an output type.
func ExampleFan() {
	first := workflow.Pure("First", func(s string) string { return s[:1] })
	last := workflow.Pure("Last", func(s string) string { return s[len(s)-1:] })

	edges := workflow.Fan("Edges", func(parts []string) (string, error) {
		return strings.Join(parts, "…"), nil
	}, first, last)

	out, _ := edges.Run(context.Background(), "workflow")
	fmt.Println(out)
	// Output: w…w
}

// Each applies one step to every element of a slice.
func ExampleEach() {
	upper := workflow.Pure("Upper", strings.ToUpper)
	batch := workflow.Each(4, upper) // Step[[]string, []string]

	out, _ := batch.Run(context.Background(), []string{"a", "b", "c"})
	fmt.Println(out)
	// Output: [A B C]
}

// String prints the shape of a pipeline.
func ExampleStep_String() {
	length := workflow.Pure("Length", func(s string) int { return len(s) })
	double := workflow.Pure("Double", func(n int) int { return n * 2 })
	square := workflow.Pure("Square", func(n int) int { return n * n })

	p := length.Then(double.Par(square, func(a, b int) (int, error) { return a + b, nil })).
		Rename("Measure")

	fmt.Println(p)
	// Output:
	// Measure
	// ├── Length
	// └── Par
	//     ├── Double
	//     └── Square
}
