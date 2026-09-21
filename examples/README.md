# Examples

Four programs, from the shortest to the longest. Each one is its own module
with a `replace` directive to the library, so `go run main.go` uses the
working tree.

## The examples

### 1. [Basic](./basic/)

A sentence in, a report out. `Pure`, `Then`, `Par`, and the tree that a
pipeline prints.

### 2. [CI/CD](./cicd/)

A build pipeline where every stage has its own type. `Fan` for the quality
checks, `If` for the gates, `Seq` for the rollout.

### 3. [Middleware](./middleware/)

`Retry`, `Timeout`, `Breaker`, `Log` and `WithID`, each on the step that needs
it. A stateful step behind `Of`.

### 4. [Advanced](./advanced/)

`Each` with bounded concurrency, a custom `Middleware`, a quality gate, joined
errors, and a JSON report.

## Running them

```bash
cd basic && go run main.go
cd cicd && go run main.go
cd middleware && go run main.go
cd advanced && go run main.go
```

Or run them all from the root of the repository:

```bash
go run github.com/magefile/mage@v1.17.2 examples
```
