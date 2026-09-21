package workflow

import (
	"fmt"
	"runtime/debug"
)

// PanicError reports a panic that a step raised. Every concurrent combinator
// turns a panic into a PanicError, so a panic in one branch never leaves a
// zero result behind and never crashes the program.
type PanicError struct {
	Step  string
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("workflow: panic in %s: %v\n%s", e.Step, e.Value, e.Stack)
}

// recovered turns the result of recover() into an error, or nil.
func recovered(v any, step string) error {
	if v == nil {
		return nil
	}
	return &PanicError{Step: step, Value: v, Stack: debug.Stack()}
}
