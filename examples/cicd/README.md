# CI/CD

A build pipeline: check out, test, build, deploy.

## What it does

1. `Checkout` turns a `Commit` into a `Source`.
2. `Fan` runs the tests, the linter and the security scan at the same time,
   and one join function turns the three `Check` values into a `Quality`.
3. `Identity` carries the source past the checks, so the build can read both.
4. `If` builds only when every check passed.
5. `Seq` chains the rollout: staging, smoke tests, production.
6. A second `If` deploys the artifact, or notifies the team.

Each stage has its own type. A stage cannot read a field that the stage
before it did not produce, because the field is not there.

## Running the example

```bash
cd examples/cicd
go run main.go
```

## What to look at

- `Fan` replaces the reflective merge of v1. The shape of the joined result
  is decided by the join function, in the example, not by the library.
- Every branch of `Fan` runs to the end. `Fan` returns every error, joined
  with `errors.Join`.
- `Log` and `WithID` are methods on the step that needs them, not settings on
  a pipeline.
