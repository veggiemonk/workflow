package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNilStep is returned when a zero Step is run. A zero Step holds no
// function. Build every step with [Func], [Of] or [Pure].
var ErrNilStep = errors.New("workflow: step holds no function")

// Runner is the extension point for a type that carries state.
//
// Its method has no type parameters, which an interface method may not have,
// so any type may implement it. Wrap an implementation with [Of] to get a
// composable [Step].
type Runner[I, O any] interface {
	Run(context.Context, I) (O, error)
}

// Step is one unit of work. It reads an I and returns an O.
//
// Step is a value. Every method returns a new Step and leaves the receiver
// unchanged, so a Step is safe to share and to run more than once.
//
// The zero Step is not usable; it returns [ErrNilStep].
type Step[I, O any] struct {
	name string
	run  func(context.Context, I) (O, error)
	kids []fmt.Stringer
}

// Func builds a step from a function.
func Func[I, O any](name string, f func(context.Context, I) (O, error)) Step[I, O] {
	return Step[I, O]{name: name, run: f}
}

// Pure builds a step from a function that cannot fail.
func Pure[I, O any](name string, f func(I) O) Step[I, O] {
	return Step[I, O]{
		name: name,
		run:  func(_ context.Context, in I) (O, error) { return f(in), nil },
	}
}

// Of builds a step from a [Runner].
func Of[I, O any](name string, r Runner[I, O]) Step[I, O] {
	if r == nil {
		return Step[I, O]{name: name}
	}
	return Step[I, O]{name: name, run: r.Run}
}

// Identity returns a step that returns its input unchanged.
func Identity[T any]() Step[T, T] {
	return Pure("Identity", func(v T) T { return v })
}

// Run executes the step.
//
// Run checks the context before it starts. It returns the zero O on error.
func (s Step[I, O]) Run(ctx context.Context, in I) (O, error) {
	var zero O
	if s.run == nil {
		return zero, fmt.Errorf("%w: %s", ErrNilStep, s.displayName())
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return s.run(ctx, in)
}

// Name returns the name of the step.
func (s Step[I, O]) Name() string { return s.name }

// Rename returns a copy of the step with a new name.
func (s Step[I, O]) Rename(name string) Step[I, O] {
	s.name = name
	return s
}

func (s Step[I, O]) displayName() string {
	if s.name == "" {
		var i I
		var o O
		return fmt.Sprintf("Step[%T,%T]", i, o)
	}
	return s.name
}

// wrap returns a copy of s that runs f instead, and that prints as a parent of
// s. It is how every middleware method is built.
func (s Step[I, O]) wrap(name string, f func(context.Context, I) (O, error)) Step[I, O] {
	return Step[I, O]{name: name, run: f, kids: []fmt.Stringer{s}}
}

// String prints the step and its children as a tree.
func (s Step[I, O]) String() string {
	var b strings.Builder
	b.WriteString(s.displayName())
	for i, kid := range s.kids {
		prefix, indent := "├── ", "│   "
		if i == len(s.kids)-1 {
			prefix, indent = "└── ", "    "
		}
		lines := strings.Split(kid.String(), "\n")
		b.WriteString("\n" + prefix + lines[0])
		for _, line := range lines[1:] {
			b.WriteString("\n" + indent + line)
		}
	}
	return b.String()
}
