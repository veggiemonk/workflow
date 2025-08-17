package workflow_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"dario.cat/mergo"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	wf "github.com/veggiemonk/workflow"
)

var lf = flag.Bool("log", false, "show the logs")

type Result struct {
	Err      error
	Messages []string
	State    State
}
type State struct{ Counter int }

func (r *Result) String() string {
	if r == nil {
		return "none"
	}
	if r.Err != nil {
		return fmt.Sprintf("Result{State: %#v, Messages: %v, Err: %v}", r.State, r.Messages, r.Err)
	}
	return fmt.Sprintf("Result{State: %#v, Messages: %v}", r.State, r.Messages)
}

func TestEmptyPipeline(t *testing.T) {
	p := wf.NewPipeline[Result]()
	_, err := p.Run(context.Background(), &Result{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMiddleware(t *testing.T) {
	cpt := 0
	incr := func() {
		cpt++
	}
	mid := func(inc func()) wf.Middleware[Result] {
		return func(next wf.Step[Result]) wf.Step[Result] {
			return &wf.MidFunc[Result]{
				Name: "Incr",
				Next: next,
				Fn: func(ctx context.Context, res *Result) (*Result, error) {
					inc()
					return next.Run(ctx, res)
				},
			}
		}
	}
	p := wf.NewPipeline(mid(incr))
	_, err := p.Run(context.Background(), &Result{Messages: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if cpt != 0 {
		t.Fatalf("cpt %d != 0", cpt)
	}

	p.Steps = append(p.Steps, wf.StepFunc[Result](func(ctx context.Context, res *Result) (*Result, error) {
		return res, nil
	}))
	_, err = p.Run(context.Background(), &Result{Messages: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if cpt != 1 {
		t.Fatalf("cpt %d != 1", cpt)
	}
}

func TestPipeline(t *testing.T) {
	wf.SetIDGenerator(&wf.StaticID{})

	var w io.Writer
	if *lf {
		w = os.Stdout
	} else {
		w = io.Discard
	}

	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				t := a.Value.Time()
				a = slog.Attr{Key: "time", Value: slog.StringValue(t.Format("15:04:05"))}
			}
			return a
		},
	}))

	selector := func(ctx context.Context, r *Result) bool {
		return r.Err == nil
	}
	ifstep := wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
		r.Messages = append(r.Messages, "if step selected")
		return r, nil
	})
	elsestep := wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
		r.Messages = append(r.Messages, "else step selected")
		return r, nil
	})

	sf := make([]wf.Step[Result], 0)
	for range 10 {
		sf = append(sf, wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
			r.State.Counter++
			return r, nil
		}))
	}

	mid := []wf.Middleware[Result]{
		LoggerMiddleware[Result](logger),
	}
	p := wf.NewPipeline(mid...)
	p.Steps = []wf.Step[Result]{
		wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
			r.Messages = append(r.Messages, "first step")
			return r, nil
		}),
		wf.Series(mid,
			wf.Parallel(mid, wf.MergeTransform[Result](mergo.WithTransformers(addInt{})), sf...),
			wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
				r.Messages = append(r.Messages, "extra serial step")
				r.Err = errors.Join(r.Err, errIgnoreMe)
				return r, nil
			}),
		),
		handleErr{l: logger},
		wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
			f := wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
				r.Messages = append(r.Messages, "extra inner step")
				r.Err = errors.Join(r.Err, errors.New("oops"))
				return r, nil
			})
			resp, err := f.Run(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			sid, err := wf.GetStepID(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if sid.String() != "00000000-0000-0000-0000-000000000029" {
				t.Fatalf("got %q, want %q",
					sid.String(),
					"00000000-0000-0000-0000-000000000029")
			}
			r.Messages = append(r.Messages, "last step")
			return resp, err
		}),
		wf.Select(mid, selector, ifstep, elsestep),
	}

	ctx := context.Background()
	got, err := p.Run(ctx, &Result{Messages: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	want := &Result{
		Err:   errors.Join(nil, errors.New("oops")),
		State: struct{ Counter int }{Counter: 10},
		Messages: []string{
			"first step",
			"extra serial step",
			"extra inner step",
			"last step",
			"else step selected",
		},
	}
	if diff := Diff(got, want); diff != "" {
		t.Fatal(diff)
	}

	if diff := Diff(p.String(), wantTree); diff != "" {
		t.Fatal(diff)
	}
}

var wantTree = `
Pipeline[Result]
├── Logger(StepFunc[workflow_test.Result])
├── Logger(series[Result]
│   ├── Logger(parallel[Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   ├── StepFunc[workflow_test.Result]
│   │   └── StepFunc[workflow_test.Result])
│   └── Logger(StepFunc[workflow_test.Result]))
├── Logger(ErrorHandler)
├── Logger(StepFunc[workflow_test.Result])
└── Logger(selector[Result]
    ├── IF: StepFunc[workflow_test.Result]
    └── ELSE: StepFunc[workflow_test.Result])
`

/*

.
├── testutil
│   ├── assert.go
│   ├── assert_test.go
│   ├── diff.go
│   ├── out
│   │   └── diff
│   ├── testdata
│   │   └── deployment.yaml
│   └── update_test.go
├── todo.md
└── workflow
    ├── ctx.go
    ├── doc.go
    ├── workflow.go
    ├── workflow_example_test.go
    └── workflow_test.go

*/

func TestString(t *testing.T) {
	p := wf.NewPipeline[Result]()
	p.Steps = []wf.Step[Result]{}

	want := "Pipeline[Result]"
	if diff := Diff(p.String(), want); diff != "" {
		t.Fatal(diff)
	}
}

func Diff(got, want any) string {
	diff := cmp.Diff(got, want,
		cmp.Exporter(func(reflect.Type) bool { return true }),
		cmpopts.EquateEmpty(),
	)
	if diff != "" {
		return "\n-got +want\n" + diff
	}
	return ""
}

func LoggerMiddleware[T any](l *slog.Logger) wf.Middleware[T] {
	return func(next wf.Step[T]) wf.Step[T] {
		return &wf.MidFunc[T]{
			Name: "Logger",
			Next: next,
			Fn: func(ctx context.Context, res *T) (*T, error) {
				start := time.Now()
				name := wf.Name(next)
				if name != "MidFunc" {
					id, _ := wf.GetStepID(ctx)
					l.Info("start", "Type", name, "id", id, "STEP", next)
				}
				resp, err := next.Run(ctx, res)

				if name != "MidFunc" {
					id, _ := wf.GetStepID(ctx)
					l.Info("done", "Type", name, "id", id, "duration", time.Since(start),
						"Result", fmt.Sprintf("%v", resp))
				}
				return resp, err
			},
		}
	}
}

type handleErr struct{ l *slog.Logger }

func (h handleErr) String() string { return "ErrorHandler" }

var errIgnoreMe = errors.New("ignore me")

func (h handleErr) Run(ctx context.Context, r *Result) (*Result, error) {
	if errors.Is(r.Err, errIgnoreMe) {
		h.l.Error("ignoring error", "err", r.Err)
		r.Err = nil
		return r, r.Err
	}
	h.l.Error("handling error", "err", r.Err)
	return r, r.Err
}

type addInt struct{}

func (t addInt) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(int(0)) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				dst.Set(reflect.ValueOf(int(dst.Int() + src.Int())))
			}
			return nil
		}
	}
	return nil
}
