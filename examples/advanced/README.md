# Advanced

A data pipeline with bounded concurrency, a quality gate and a custom
middleware.

## What it does

1. `Inspect` counts the invalid records and keeps the input it judged.
2. `If` cleans the input when more than a tenth of it is invalid.
3. `Each(8, transform)` processes the records, at most eight at a time.
4. `summarise` counts the categories and the throughput, and the report is
   written to `results.json`.
5. The last part runs the same batch on uncleaned input, to show the errors
   that `Each` joins together.

## Running the example

```bash
cd examples/advanced
go run main.go
```

## What to look at

- `timed` is a custom middleware. `Middleware[I, O]` is a function from a step
  to a step, so writing one needs nothing from the library. A step holds no
  state of its own, so the durations go to a collector you can read.
- `Each` is a function, not a method. A method returning `Step[[]I, []O]`
  would make the compiler instantiate `Step[[][]I, [][]O]`, and so on without
  end.
- `Each` runs every element to the end and returns every error joined with
  `errors.Join`. Nothing is dropped in silence.
- The export uses `encoding/json/v2`. The options keep the duration format and
  the sorted map order that v1 gave.

## Files generated

- `results.json`
