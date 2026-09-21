# Basic

A sequential pipeline that reads a sentence and returns a report.

## What it does

1. `Clean` trims the input. `string -> string`
2. `Split` cuts it into words. `string -> []string`
3. `Count`, `Unique` and `Head` read the same words at the same time.
4. Two joins turn the three results into one `Report`.

Every step has its own input type and its own output type, so no struct has
to carry the whole run.

## Running the example

```bash
cd examples/basic
go run main.go
```

## What to look at

- `Pure` builds a step from a function that cannot fail.
- `Then` links two steps and lets the chain change type.
- `Par` runs two steps on one input and joins them with a typed function. The
  compiler will not let a branch result be dropped.
- Printing the pipeline gives a tree of the steps as they were declared.
