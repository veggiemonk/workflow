// Package workflow composes typed units of work into a pipeline.
//
// # The Step value
//
// [Step] is a concrete generic struct, not an interface. A step declares its
// own input type and its own output type:
//
//	Step[I, O any]  // Run(ctx, I) (O, error)
//
// A struct is necessary because Go 1.27 allows type parameters on a method
// only when the receiver is a concrete type. An interface method must have no
// type parameters. Generic methods are what let a chain change its type:
//
//	parse := workflow.Func("Parse", func(ctx context.Context, s string) (Tokens, error) { ... })
//	count := workflow.Func("Count", func(ctx context.Context, t Tokens) (int, error) { ... })
//	pipeline := parse.Then(count) // Step[string, int]
//
// To write a step on a type that carries state, implement [Runner] and wrap it
// with [Of]. [Runner] is a plain interface, so it stays legal.
//
// # A step is immutable
//
// Every method returns a new [Step]. No method writes to its receiver. A step
// is therefore safe to share, to reuse and to run concurrently. Middleware is
// applied once, when the step is built, never while it runs.
//
// # Concurrency
//
// [Step.Par] runs two steps on the same input and joins their results with a
// function you supply. [Fan] does the same for any number of steps that share
// an output type. [Each] applies one step to every element of a slice. All
// three keep the results typed, so no result can be lost, and none of them
// copies your data.
//
// # Errors
//
// A concurrent combinator collects every branch error with [errors.Join]. It
// does not stop at the first one.
package workflow
