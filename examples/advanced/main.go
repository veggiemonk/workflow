package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	wf "github.com/veggiemonk/workflow"
)

// DataProcessingContext represents complex data processing workflow
type DataProcessingContext struct {
	InputData     []DataRecord
	ProcessedData []ProcessedRecord
	Metrics       ProcessingMetrics
	Errors        []error
	Status        string
	Metadata      map[string]any
}

type DataRecord struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
	Type  string `json:"type"`
}

type ProcessedRecord struct {
	ID             string    `json:"id"`
	OriginalValue  int       `json:"original_value"`
	ProcessedValue int       `json:"processed_value"`
	ProcessedAt    time.Time `json:"processed_at"`
	Category       string    `json:"category"`
}

type ProcessingMetrics struct {
	TotalRecords     int           `json:"total_records"`
	ProcessedCount   int           `json:"processed_count"`
	ErrorCount       int           `json:"error_count"`
	ProcessingTime   time.Duration `json:"processing_time"`
	ThroughputPerSec float64       `json:"throughput_per_sec"`
}

// Custom middleware for metrics collection
func metricsMiddleware() wf.Middleware[DataProcessingContext] {
	return func(next wf.Step[DataProcessingContext]) wf.Step[DataProcessingContext] {
		return &wf.MidFunc[DataProcessingContext]{
			Name: "Metrics",
			Next: next,
			Fn: func(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
				start := time.Now()
				stepName := wf.Name(next)

				// Execute the step
				result, err := next.Run(ctx, data)

				duration := time.Since(start)

				// Update metadata with step timing
				if result.Metadata == nil {
					result.Metadata = make(map[string]any)
				}
				result.Metadata[fmt.Sprintf("step_%s_duration", stepName)] = duration

				return result, err
			},
		}
	}
}

// Custom middleware for error handling
func errorHandlingMiddleware(logger *slog.Logger) wf.Middleware[DataProcessingContext] {
	return func(next wf.Step[DataProcessingContext]) wf.Step[DataProcessingContext] {
		return &wf.MidFunc[DataProcessingContext]{
			Name: "ErrorHandler",
			Next: next,
			Fn: func(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
				result, err := next.Run(ctx, data)
				if err != nil {
					logger.Error("Step failed", "step", wf.Name(next), "error", err)
					result.Errors = append(result.Errors, err)
					result.Status = "error"
					// Don't return error to continue pipeline
					return result, nil
				}

				return result, nil
			},
		}
	}
}

func main() {
	// Set up structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create sample data
	inputData := generateSampleData(1000)

	// Create advanced pipeline with multiple middleware
	pipeline := wf.NewPipeline(
		wf.UUIDMiddleware[DataProcessingContext](),
		metricsMiddleware(),
		errorHandlingMiddleware(logger),
		wf.LoggerMiddleware[DataProcessingContext](logger),
	)

	// Define complex processing pipeline
	pipeline.Steps = []wf.Step[DataProcessingContext]{
		// Step 1: Initialize processing
		wf.StepFunc[DataProcessingContext](initializeProcessing),

		// Step 2: Parallel data validation and preprocessing
		wf.Parallel(nil, mergeDataProcessingResults,
			wf.StepFunc[DataProcessingContext](validateData),
			wf.StepFunc[DataProcessingContext](preprocessData),
			wf.StepFunc[DataProcessingContext](calculateInitialMetrics),
		),

		// Step 3: Conditional processing based on data quality
		wf.Select(nil,
			dataQualityGood,
			// High quality data path
			wf.Sequential(nil,
				wf.Parallel(nil, mergeDataProcessingResults,
					wf.StepFunc[DataProcessingContext](processHighValueRecords),
					wf.StepFunc[DataProcessingContext](processLowValueRecords),
					wf.StepFunc[DataProcessingContext](processSpecialRecords),
				),
				wf.StepFunc[DataProcessingContext](aggregateResults),
			),
			// Low quality data path
			wf.Sequential(nil,
				wf.StepFunc[DataProcessingContext](cleanData),
				wf.StepFunc[DataProcessingContext](reprocessData),
			),
		),

		// Step 4: Final validation and reporting
		wf.StepFunc[DataProcessingContext](finalValidation),
		wf.StepFunc[DataProcessingContext](generateReport),
	}

	// Execute the pipeline
	fmt.Println("🚀 Starting Advanced Data Processing Pipeline...")

	ctx := context.Background()
	startTime := time.Now()

	result, err := pipeline.Run(ctx, &DataProcessingContext{
		InputData: inputData,
		Metadata:  make(map[string]any),
		Status:    "initialized",
	})

	totalDuration := time.Since(startTime)

	if err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	// Display comprehensive results
	fmt.Printf("\n⏱️  Total Pipeline Duration: %v\n", totalDuration)
	fmt.Printf("📊 Processing Status: %s\n", result.Status)
	fmt.Printf("📈 Records Processed: %d/%d\n", result.Metrics.ProcessedCount, result.Metrics.TotalRecords)
	fmt.Printf("⚡ Throughput: %.2f records/sec\n", result.Metrics.ThroughputPerSec)

	if len(result.Errors) > 0 {
		fmt.Printf("\n❌ Errors encountered: %d\n", len(result.Errors))
		for i, err := range result.Errors {
			fmt.Printf("%d. %s\n", i+1, err.Error())
		}
	}

	// Display step timings
	fmt.Println("\n⏱️  Step Timings:")
	for key, value := range result.Metadata {
		if duration, ok := value.(time.Duration); ok {
			fmt.Printf("  %s: %v\n", key, duration)
		}
	}

	// Export results
	if err := exportResults(result); err != nil {
		fmt.Printf("⚠️  Failed to export results: %v\n", err)
	} else {
		fmt.Println("💾 Results exported to results.json")
	}

	fmt.Println("\n🌳 Pipeline Structure:")
	fmt.Print(pipeline.String())
}

// Step implementations

func initializeProcessing(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("🔄 Initializing data processing...")
	data.Metrics.TotalRecords = len(data.InputData)
	data.Status = "processing"
	return data, nil
}

func validateData(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("✅ Validating data quality...")
	time.Sleep(50 * time.Millisecond) // Simulate processing

	// Count invalid records
	invalidCount := 0
	for _, record := range data.InputData {
		if record.Value < 0 || record.Type == "" {
			invalidCount++
		}
	}

	data.Metadata["invalid_records"] = invalidCount
	return data, nil
}

func preprocessData(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("🔧 Preprocessing data...")
	time.Sleep(75 * time.Millisecond) // Simulate processing

	data.Metadata["preprocessing_complete"] = true
	return data, nil
}

func calculateInitialMetrics(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("📊 Calculating initial metrics...")
	time.Sleep(25 * time.Millisecond) // Simulate processing

	totalValue := 0
	for _, record := range data.InputData {
		totalValue += record.Value
	}

	data.Metadata["total_input_value"] = totalValue
	return data, nil
}

func processHighValueRecords(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("💎 Processing high-value records...")
	time.Sleep(100 * time.Millisecond) // Simulate processing

	count := 0
	for _, record := range data.InputData {
		if record.Value > 500 {
			processed := ProcessedRecord{
				ID:             record.ID,
				OriginalValue:  record.Value,
				ProcessedValue: record.Value * 2, // High-value multiplier
				ProcessedAt:    time.Now(),
				Category:       "high-value",
			}
			data.ProcessedData = append(data.ProcessedData, processed)
			count++
		}
	}

	data.Metadata["high_value_processed"] = count
	return data, nil
}

func processLowValueRecords(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("📉 Processing low-value records...")
	time.Sleep(80 * time.Millisecond) // Simulate processing

	count := 0
	for _, record := range data.InputData {
		if record.Value <= 500 && record.Value > 0 {
			processed := ProcessedRecord{
				ID:             record.ID,
				OriginalValue:  record.Value,
				ProcessedValue: record.Value + 100, // Low-value bonus
				ProcessedAt:    time.Now(),
				Category:       "low-value",
			}
			data.ProcessedData = append(data.ProcessedData, processed)
			count++
		}
	}

	data.Metadata["low_value_processed"] = count
	return data, nil
}

func processSpecialRecords(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("⭐ Processing special records...")
	time.Sleep(60 * time.Millisecond) // Simulate processing

	count := 0
	for _, record := range data.InputData {
		if record.Type == "special" {
			processed := ProcessedRecord{
				ID:             record.ID,
				OriginalValue:  record.Value,
				ProcessedValue: record.Value * 3, // Special multiplier
				ProcessedAt:    time.Now(),
				Category:       "special",
			}
			data.ProcessedData = append(data.ProcessedData, processed)
			count++
		}
	}

	data.Metadata["special_processed"] = count
	return data, nil
}

func aggregateResults(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("📋 Aggregating results...")
	time.Sleep(30 * time.Millisecond) // Simulate processing

	data.Metrics.ProcessedCount = len(data.ProcessedData)

	if processingTime, ok := data.Metadata["step_series_duration"].(time.Duration); ok {
		data.Metrics.ProcessingTime = processingTime
		if processingTime > 0 {
			data.Metrics.ThroughputPerSec = float64(data.Metrics.ProcessedCount) / processingTime.Seconds()
		}
	}

	return data, nil
}

func cleanData(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("🧹 Cleaning low-quality data...")
	time.Sleep(120 * time.Millisecond) // Simulate processing

	// Simulate data cleaning
	cleanedCount := 0
	for _, record := range data.InputData {
		if record.Value >= 0 && record.Type != "" {
			cleanedCount++
		}
	}

	data.Metadata["cleaned_records"] = cleanedCount
	return data, nil
}

func reprocessData(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("🔄 Reprocessing cleaned data...")
	time.Sleep(100 * time.Millisecond) // Simulate processing

	// Simple reprocessing for cleaned data
	for _, record := range data.InputData {
		if record.Value >= 0 && record.Type != "" {
			processed := ProcessedRecord{
				ID:             record.ID,
				OriginalValue:  record.Value,
				ProcessedValue: record.Value, // No transformation for cleaned data
				ProcessedAt:    time.Now(),
				Category:       "cleaned",
			}
			data.ProcessedData = append(data.ProcessedData, processed)
		}
	}

	data.Metrics.ProcessedCount = len(data.ProcessedData)
	return data, nil
}

func finalValidation(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("🔍 Performing final validation...")
	time.Sleep(40 * time.Millisecond) // Simulate processing

	// Validate processed data
	validCount := 0
	for _, record := range data.ProcessedData {
		if record.ProcessedValue > 0 {
			validCount++
		}
	}

	if validCount == len(data.ProcessedData) {
		data.Status = "completed"
	} else {
		data.Status = "completed_with_warnings"
		data.Errors = append(data.Errors, fmt.Errorf("validation warnings: %d/%d records passed", validCount, len(data.ProcessedData)))
	}

	return data, nil
}

func generateReport(ctx context.Context, data *DataProcessingContext) (*DataProcessingContext, error) {
	fmt.Println("📊 Generating final report...")
	time.Sleep(20 * time.Millisecond) // Simulate processing

	data.Metadata["report_generated"] = true
	data.Metadata["report_timestamp"] = time.Now()
	return data, nil
}

// Selector functions

func dataQualityGood(ctx context.Context, data *DataProcessingContext) bool {
	if invalidRecords, ok := data.Metadata["invalid_records"].(int); ok {
		return float64(invalidRecords)/float64(data.Metrics.TotalRecords) < 0.1 // Less than 10% invalid
	}
	return true
}

// Merge function for data processing results
func mergeDataProcessingResults(ctx context.Context, base *DataProcessingContext, results ...*DataProcessingContext) (*DataProcessingContext, error) {
	var mu sync.Mutex

	for _, result := range results {
		mu.Lock()
		// Merge processed data
		base.ProcessedData = append(base.ProcessedData, result.ProcessedData...)

		// Merge errors
		base.Errors = append(base.Errors, result.Errors...)

		// Merge metadata
		for key, value := range result.Metadata {
			if base.Metadata == nil {
				base.Metadata = make(map[string]any)
			}
			base.Metadata[key] = value
		}

		// Update metrics
		base.Metrics.ProcessedCount += result.Metrics.ProcessedCount
		base.Metrics.ErrorCount += result.Metrics.ErrorCount
		mu.Unlock()
	}

	return base, nil
}

// Helper functions

func generateSampleData(count int) []DataRecord {
	data := make([]DataRecord, count)
	types := []string{"normal", "special", "premium", ""}

	for i := 0; i < count; i++ {
		data[i] = DataRecord{
			ID:    fmt.Sprintf("record-%d", i),
			Value: (i * 37) % 1000, // Pseudo-random values
			Type:  types[i%len(types)],
		}
	}

	return data
}

func exportResults(data *DataProcessingContext) error {
	file, err := os.Create("results.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
