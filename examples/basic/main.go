package main

import (
	"context"
	"fmt"
	"log"

	wf "github.com/veggiemonk/workflow"
)

// ProcessData represents the data flowing through our pipeline
type ProcessData struct {
	Input    string
	Output   string
	Messages []string
	Counter  int
}

func main() {
	// Create a simple pipeline
	pipeline := wf.NewPipeline[ProcessData]()

	// Define the steps
	pipeline.Tasks = []*wf.Task[ProcessData]{
		// Step 1: Initialize
		wf.NewTask("initialize", wf.StepFunc[ProcessData](func(ctx context.Context, data *ProcessData) (*ProcessData, error) {
			data.Messages = append(data.Messages, "Pipeline started")
			fmt.Println("✓ Pipeline initialized")
			return data, nil
		})),

		// Step 2: Transform input
		wf.NewTask("transform", wf.StepFunc[ProcessData](func(ctx context.Context, data *ProcessData) (*ProcessData, error) {
			data.Output = fmt.Sprintf("Processed: %s", data.Input)
			data.Messages = append(data.Messages, "Input transformed")
			fmt.Println("✓ Input transformed")
			return data, nil
		})),

		// Step 3: Count processing
		wf.NewTask("count", wf.StepFunc[ProcessData](func(ctx context.Context, data *ProcessData) (*ProcessData, error) {
			data.Counter++
			data.Messages = append(data.Messages, "Counter incremented")
			fmt.Println("✓ Counter incremented")
			return data, nil
		})),

		// Step 4: Finalize
		wf.NewTask("finalize", wf.StepFunc[ProcessData](func(ctx context.Context, data *ProcessData) (*ProcessData, error) {
			data.Messages = append(data.Messages, "Pipeline completed")
			fmt.Println("✓ Pipeline completed")
			return data, nil
		})),
	}

	// Execute the pipeline
	fmt.Println("🚀 Starting basic workflow example...")
	fmt.Println()

	result, err := pipeline.Run(context.Background(), &ProcessData{
		Input: "Hello World",
	})
	if err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	// Display results
	fmt.Println("\n📊 Results:")
	fmt.Printf("Input: %s\n", result.Input)
	fmt.Printf("Output: %s\n", result.Output)
	fmt.Printf("Counter: %d\n", result.Counter)
	fmt.Println("\n📝 Messages:")
	for i, msg := range result.Messages {
		fmt.Printf("%d. %s\n", i+1, msg)
	}

	fmt.Println("\n🌳 Pipeline Structure:")
	fmt.Print(pipeline.String())
}
