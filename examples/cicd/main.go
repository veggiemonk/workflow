package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	wf "github.com/veggiemonk/workflow"
)

// Each stage of the pipeline has its own type. A stage cannot read a field
// that the stage before it did not produce, because the field is not there.

// Commit is what the pipeline starts from.
type Commit struct {
	Repo string
	SHA  string
}

// Source is a checked-out tree.
type Source struct {
	Commit Commit
	Path   string
}

// Check is the verdict of one quality gate.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

// Quality holds every check, and says whether all of them passed.
type Quality struct {
	Source Source
	Checks []Check
	Passed bool
}

// Artifact is a built binary. Built is false when the gate skipped the build.
type Artifact struct {
	Version string
	Path    string
	Built   bool
}

// Release is the end of the pipeline.
type Release struct {
	Version  string
	Env      string
	Deployed bool
	Notes    []string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 1. Check out the source.
	checkout := wf.Func("Checkout", func(_ context.Context, c Commit) (Source, error) {
		fmt.Println("🔄 Checking out source code...")
		return Source{Commit: c, Path: "/src/" + c.Repo}, nil
	}).Log(logger).WithID()

	// 2. Run the quality checks at the same time. Fan takes any number of
	// steps that share an output type and joins their results with one
	// function. Every branch runs to the end; Fan returns every error.
	checks := wf.Fan("Checks", joinChecks,
		check("Tests", 100*time.Millisecond, "🧪 Running tests..."),
		check("Lint", 50*time.Millisecond, "🔍 Running linter..."),
		check("Security", 150*time.Millisecond, "🔒 Running security scan..."),
	)

	// The checks report verdicts, not the source they read. Identity returns
	// its input, so Par carries the source past the checks and the join puts
	// the two together.
	quality := wf.Identity[Source]().
		Par(checks, func(s Source, q Quality) (Quality, error) {
			q.Source = s
			return q, nil
		}).
		Rename("Quality").
		Log(logger)

	// 3. Build, but only when every check passed. If picks a branch on the
	// value that reaches it; both branches have the same shape.
	gate := wf.If("Gate",
		func(_ context.Context, q Quality) bool { return q.Passed },
		build,
		skipBuild,
	)

	// 4. Deploy what was built. Seq chains steps that keep one type.
	rollout := wf.Seq("Rollout",
		stage("Staging", "🚀 Deploying to staging...", 100*time.Millisecond),
		stage("SmokeTests", "💨 Running smoke tests...", 75*time.Millisecond),
		stage("Production", "🎯 Deploying to production...", 150*time.Millisecond),
	)

	deploy := wf.If("Deploy?",
		func(_ context.Context, a Artifact) bool { return a.Built },
		wf.Func("Release", func(ctx context.Context, a Artifact) (Release, error) {
			return rollout.Run(ctx, Release{Version: a.Version, Env: "staging"})
		}),
		wf.Func("Notify", func(_ context.Context, a Artifact) (Release, error) {
			fmt.Println("📧 Sending failure notifications...")
			return Release{Version: a.Version, Notes: []string{"build skipped, team notified"}}, nil
		}),
	)

	// The whole pipeline is one value of type Step[Commit, Release].
	pipeline := checkout.Then(quality).Then(gate).Then(deploy)

	fmt.Println("🚀 Starting CI/CD Pipeline...")
	fmt.Println()

	start := time.Now()
	release, err := pipeline.Run(context.Background(), Commit{Repo: "myapp", SHA: "8f3a1c2"})
	if err != nil {
		log.Fatalf("CI/CD pipeline failed: %v", err)
	}

	fmt.Printf("\n⏱️  Pipeline completed in %v\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("📦 Version: %s\n", release.Version)
	fmt.Printf("🚀 Deployed: %v (%s)\n", release.Deployed, release.Env)

	fmt.Println("\n📝 Notes:")
	for i, note := range release.Notes {
		fmt.Printf("%d. %s\n", i+1, note)
	}

	fmt.Println("\n🌳 Pipeline structure:")
	fmt.Println(pipeline)
}

// check builds one quality gate. A real one would run a command.
func check(name string, d time.Duration, msg string) wf.Step[Source, Check] {
	return wf.Func(name, func(ctx context.Context, s Source) (Check, error) {
		fmt.Println(msg)
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return Check{}, ctx.Err()
		}
		return Check{Name: name, Passed: true, Detail: name + " passed on " + s.Commit.SHA}, nil
	})
}

// joinChecks is the typed merge that Fan needs. It replaces the reflective
// merge of v1: the shape of the result is decided here, not by a library.
func joinChecks(checks []Check) (Quality, error) {
	q := Quality{Checks: checks, Passed: true}
	for _, c := range checks {
		q.Passed = q.Passed && c.Passed
	}
	return q, nil
}

var build = wf.Func("Build", func(_ context.Context, q Quality) (Artifact, error) {
	fmt.Println("🏗️  Building application...")
	time.Sleep(200 * time.Millisecond)
	return Artifact{
		Version: "1.0.0+" + q.Source.Commit.SHA,
		Path:    "/build/" + q.Source.Commit.Repo,
		Built:   true,
	}, nil
})

var skipBuild = wf.Func("SkipBuild", func(_ context.Context, q Quality) (Artifact, error) {
	fmt.Println("⏭️  Skipping build: a quality check failed")
	return Artifact{Version: "none"}, nil
})

// stage is one step of the rollout. It keeps the Release type, so Seq can
// chain the stages.
func stage(name, msg string, d time.Duration) wf.Step[Release, Release] {
	return wf.Func(name, func(ctx context.Context, r Release) (Release, error) {
		fmt.Println(msg)
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return Release{}, ctx.Err()
		}
		r.Env = name
		r.Deployed = true
		r.Notes = append(r.Notes, name+" ok")
		return r, nil
	})
}
