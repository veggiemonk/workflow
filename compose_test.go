package workflow_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	workflow "github.com/veggiemonk/workflow"
)

type report struct {
	Words  int
	Unique int
	Upper  string
}

// v1 lost a branch here: mergo refused to overwrite a non-empty field, so two
// branches that each added 1 produced 1, not 2. A typed join cannot lose one.
func TestParKeepsEveryResult(t *testing.T) {
	inc := workflow.Pure("Inc", func(n int) int { return n + 1 })
	sum := inc.Par(inc, func(a, b int) (int, error) { return a + b, nil })

	got, err := sum.Run(t.Context(), 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != 2 {
		t.Errorf("got %d, want 2 (v1 returned 1 here)", got)
	}
}

func TestParJoinsBothErrors(t *testing.T) {
	boom := func(msg string) workflow.Step[int, int] {
		return workflow.Func(msg, func(context.Context, int) (int, error) { return 0, errors.New(msg) })
	}
	s := boom("left").Par(boom("right"), func(a, b int) (int, error) { return a + b, nil })
	_, err := s.Run(t.Context(), 0)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"left", "right"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// v1 logged the panic, returned nil from the goroutine, and then crashed the
// caller with a second panic inside mergo.
func TestParTurnsPanicIntoError(t *testing.T) {
	ok := workflow.Pure("Ok", func(n int) int { return n })
	bad := workflow.Func("Bad", func(context.Context, int) (int, error) { panic("boom") })

	_, err := ok.Par(bad, func(a, b int) (int, error) { return a + b, nil }).Run(t.Context(), 1)
	if err == nil {
		t.Fatal("want an error, not a crash")
	}
	var pe *workflow.PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("got %T, want *workflow.PanicError", err)
	}
	if pe.Step != "Bad" || pe.Value != "boom" {
		t.Errorf("got step=%q value=%v, want Bad and boom", pe.Step, pe.Value)
	}
	if len(pe.Stack) == 0 {
		t.Error("the error carries no stack")
	}
}

func TestParTyped(t *testing.T) {
	count := workflow.Pure("Count", func(t tokens) int { return len(t) })
	uniq := workflow.Pure("Uniq", func(t tokens) int {
		set := map[string]struct{}{}
		for _, w := range t {
			set[w] = struct{}{}
		}
		return len(set)
	})
	upper := workflow.Pure("Upper", func(t tokens) string { return strings.ToUpper(strings.Join(t, " ")) })

	analyse := count.
		Par(uniq, func(n, u int) ([2]int, error) { return [2]int{n, u}, nil }).
		Par(upper, func(nu [2]int, s string) (report, error) {
			return report{Words: nu[0], Unique: nu[1], Upper: s}, nil
		})

	got, err := parse().Then(analyse).Run(t.Context(), Doc{Body: "go go gadget"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := report{Words: 3, Unique: 2, Upper: "GO GO GADGET"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFan(t *testing.T) {
	double := workflow.Pure("Double", func(n int) int { return n * 2 })
	square := workflow.Pure("Square", func(n int) int { return n * n })
	negate := workflow.Pure("Negate", func(n int) int { return -n })

	s := workflow.Fan("All", func(out []int) (int, error) {
		total := 0
		for _, v := range out {
			total += v
		}
		return total, nil
	}, double, square, negate)

	got, err := s.Run(t.Context(), 3) // 6 + 9 - 3
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != 12 {
		t.Errorf("got %d, want 12", got)
	}
}

func TestFanKeepsOrder(t *testing.T) {
	step := func(n int) workflow.Step[int, int] {
		return workflow.Pure(fmt.Sprint(n), func(int) int { return n })
	}
	s := workflow.Fan("Order", func(out []int) ([]int, error) { return out, nil },
		step(1), step(2), step(3), step(4), step(5))
	got, err := s.Run(t.Context(), 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i, v := range got {
		if v != i+1 {
			t.Fatalf("got %v, want [1 2 3 4 5]", got)
		}
	}
}

func TestEach(t *testing.T) {
	p := workflow.Each(4, parse().Then(count()))
	got, err := p.Run(t.Context(), []Doc{{Body: "a b c"}, {Body: "d d"}, {Body: "e"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []int{3, 2, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestEachRespectsConcurrencyLimit(t *testing.T) {
	var inFlight, peak atomic.Int64
	slow := workflow.Func("Slow", func(ctx context.Context, n int) (int, error) {
		cur := inFlight.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
		defer inFlight.Add(-1)
		return n, nil
	})
	in := make([]int, 50)
	if _, err := workflow.Each(3, slow).Run(t.Context(), in); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if peak.Load() > 3 {
		t.Errorf("peak concurrency = %d, want at most 3", peak.Load())
	}
}

func TestEachJoinsEveryError(t *testing.T) {
	_, err := workflow.Each(0, parse()).Run(t.Context(), []Doc{{Body: "ok"}, {}, {}})
	if err == nil {
		t.Fatal("want an error")
	}
	if n := strings.Count(err.Error(), "empty document"); n != 2 {
		t.Errorf("got %d errors, want 2: %v", n, err)
	}
}

func TestEachEmptyInput(t *testing.T) {
	got, err := workflow.Each(2, parse()).Run(t.Context(), nil)
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v; want an empty slice and no error", got, err)
	}
}

func TestSeq(t *testing.T) {
	add := func(n int) workflow.Step[int, int] {
		return workflow.Pure(fmt.Sprint("Add", n), func(v int) int { return v + n })
	}
	got, err := workflow.Seq("Sum", add(1), add(2), add(3)).Run(t.Context(), 0)
	if err != nil || got != 6 {
		t.Errorf("got %d, %v; want 6, nil", got, err)
	}
}

func TestSeqStopsOnError(t *testing.T) {
	ran := false
	bad := workflow.Func("Bad", func(context.Context, int) (int, error) { return 0, errors.New("nope") })
	after := workflow.Pure("After", func(int) int { ran = true; return 0 })
	if _, err := workflow.Seq("S", bad, after).Run(t.Context(), 0); err == nil {
		t.Fatal("want an error")
	}
	if ran {
		t.Error("the later step ran")
	}
}

func TestIf(t *testing.T) {
	even := workflow.Pure("Even", func(int) string { return "even" })
	odd := workflow.Pure("Odd", func(int) string { return "odd" })
	s := workflow.If("Parity", func(_ context.Context, n int) bool { return n%2 == 0 }, even, odd)

	for in, want := range map[int]string{2: "even", 3: "odd"} {
		if got, err := s.Run(t.Context(), in); err != nil || got != want {
			t.Errorf("Run(%d) = %q, %v; want %q", in, got, err, want)
		}
	}
}
