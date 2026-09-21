package workflow

import "fmt"

// PanicError reports a panic raised by a step that ran in its own goroutine.
//
// [Parallel] returns one of these instead of letting the panic stop the
// program. Test for it with errors.As.
type PanicError struct {
	// Step is the String() of the step that panicked.
	Step string
	// Value is what the step passed to panic.
	Value any
	// Stack is the stack trace taken where the panic was recovered.
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("workflow: panic in %s: %v\n%s", e.Step, e.Value, e.Stack)
}
