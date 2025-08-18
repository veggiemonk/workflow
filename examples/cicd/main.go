package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	wf "github.com/veggiemonk/workflow"
)

// BuildContext represents the state of a CI/CD pipeline
type BuildContext struct {
	SourcePath     string
	BuildPath      string
	TestsPassed    bool
	LintPassed     bool
	SecurityPassed bool
	BuildSucceeded bool
	Deployed       bool
	Messages       []string
	Errors         []error
}

func main() {
	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create CI/CD pipeline with middleware
	pipeline := wf.NewPipeline(
		wf.LoggerMiddleware[BuildContext](logger),
		wf.UUIDMiddleware[BuildContext](),
	)

	// Define the CI/CD pipeline steps
	pipeline.Steps = []wf.Step[BuildContext]{
		// Step 1: Checkout code
		wf.StepFunc[BuildContext](checkoutCode),

		// Step 2: Run parallel quality checks
		wf.Parallel(nil, wf.Merge[BuildContext],
			wf.StepFunc[BuildContext](runTests),
			wf.StepFunc[BuildContext](runLinter),
			wf.StepFunc[BuildContext](runSecurityScan),
		),

		// Step 3: Build if quality checks pass
		wf.Select(nil,
			qualityChecksPassed,
			wf.StepFunc[BuildContext](buildApplication),
			wf.StepFunc[BuildContext](skipBuild),
		),

		// Step 4: Deploy if build succeeded
		wf.Select(nil,
			buildSucceeded,
			wf.Series(nil,
				wf.StepFunc[BuildContext](deployToStaging),
				wf.StepFunc[BuildContext](runSmokeTests),
				wf.StepFunc[BuildContext](deployToProduction),
			),
			wf.StepFunc[BuildContext](notifyFailure),
		),

		// Step 5: Final reporting
		wf.StepFunc[BuildContext](generateReport),
	}

	// Execute the pipeline
	fmt.Println("🚀 Starting CI/CD Pipeline...")
	fmt.Println()

	startTime := time.Now()
	result, err := pipeline.Run(context.Background(), &BuildContext{
		SourcePath: "/src/myapp",
		BuildPath:  "/build/myapp",
	})

	duration := time.Since(startTime)

	if err != nil {
		log.Fatalf("CI/CD Pipeline failed: %v", err)
	}

	// Display results
	fmt.Printf("\n⏱️  Pipeline completed in %v\n", duration)
	fmt.Printf("📦 Build Status: %v\n", result.BuildSucceeded)
	fmt.Printf("🚀 Deployment Status: %v\n", result.Deployed)

	fmt.Println("\n📝 Pipeline Messages:")
	for i, msg := range result.Messages {
		fmt.Printf("%d. %s\n", i+1, msg)
	}

	if len(result.Errors) > 0 {
		fmt.Println("\n❌ Errors:")
		for i, err := range result.Errors {
			fmt.Printf("%d. %s\n", i+1, err.Error())
		}
	}

	fmt.Println("\n🌳 Pipeline Structure:")
	fmt.Print(pipeline.String())
}

// Step implementations

func checkoutCode(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🔄 Checking out source code...")
	bc.Messages = append(bc.Messages, "Source code checked out successfully")
	return bc, nil
}

func runTests(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🧪 Running tests...")
	time.Sleep(100 * time.Millisecond) // Simulate test execution

	// Simulate occasional test failures
	bc.TestsPassed = true // In real scenario, this would be based on actual test results

	if bc.TestsPassed {
		bc.Messages = append(bc.Messages, "All tests passed")
	} else {
		bc.Errors = append(bc.Errors, errors.New("some tests failed"))
	}
	return bc, nil
}

func runLinter(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🔍 Running linter...")
	time.Sleep(50 * time.Millisecond) // Simulate linting

	bc.LintPassed = true
	bc.Messages = append(bc.Messages, "Code linting passed")
	return bc, nil
}

func runSecurityScan(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🔒 Running security scan...")
	time.Sleep(150 * time.Millisecond) // Simulate security scanning

	bc.SecurityPassed = true
	bc.Messages = append(bc.Messages, "Security scan completed - no vulnerabilities found")
	return bc, nil
}

func buildApplication(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🏗️  Building application...")
	time.Sleep(200 * time.Millisecond) // Simulate build process

	bc.BuildSucceeded = true
	bc.Messages = append(bc.Messages, "Application built successfully")
	return bc, nil
}

func skipBuild(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("⏭️  Skipping build due to quality check failures")
	bc.Messages = append(bc.Messages, "Build skipped - quality checks failed")
	return bc, nil
}

func deployToStaging(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🚀 Deploying to staging...")
	time.Sleep(100 * time.Millisecond) // Simulate deployment

	bc.Messages = append(bc.Messages, "Deployed to staging environment")
	return bc, nil
}

func runSmokeTests(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("💨 Running smoke tests...")
	time.Sleep(75 * time.Millisecond) // Simulate smoke tests

	bc.Messages = append(bc.Messages, "Smoke tests passed")
	return bc, nil
}

func deployToProduction(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("🎯 Deploying to production...")
	time.Sleep(150 * time.Millisecond) // Simulate production deployment

	bc.Deployed = true
	bc.Messages = append(bc.Messages, "Successfully deployed to production")
	return bc, nil
}

func notifyFailure(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("📧 Sending failure notifications...")
	bc.Messages = append(bc.Messages, "Failure notifications sent to team")
	return bc, nil
}

func generateReport(ctx context.Context, bc *BuildContext) (*BuildContext, error) {
	fmt.Println("📊 Generating pipeline report...")
	bc.Messages = append(bc.Messages, "Pipeline report generated")
	return bc, nil
}

// Selector functions

func qualityChecksPassed(ctx context.Context, bc *BuildContext) bool {
	return bc.TestsPassed && bc.LintPassed && bc.SecurityPassed
}

func buildSucceeded(ctx context.Context, bc *BuildContext) bool {
	return bc.BuildSucceeded
}
