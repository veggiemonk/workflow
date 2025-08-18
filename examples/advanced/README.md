# Advanced Data Processing Example

This example demonstrates sophisticated workflow patterns including custom middleware, complex data structures, parallel processing with custom merge functions, and comprehensive error handling.

## What it does

This advanced data processing pipeline includes:

1. **Data Initialization**: Set up processing context with metrics
2. **Parallel Validation & Preprocessing**: 
   - Data quality validation
   - Preprocessing operations
   - Initial metrics calculation
3. **Conditional Processing Paths**:
   - **High Quality Path**: Parallel processing by record type with aggregation
   - **Low Quality Path**: Data cleaning and reprocessing
4. **Final Validation**: Quality checks and report generation

## Advanced features demonstrated

### Custom Middleware
- **Metrics Middleware**: Automatic timing collection for each step
- **Error Handling Middleware**: Graceful error recovery and continuation
- **Combined Middleware**: Multiple middleware working together

### Complex Data Flow
- **Custom Merge Function**: Safe parallel data aggregation with mutex
- **Rich Data Structures**: Nested structs with comprehensive metadata
- **Metrics Collection**: Throughput, timing, and quality metrics

### Real-world Patterns
- **Data Quality Branching**: Different processing paths based on data quality
- **Parallel Processing**: Independent operations on different data subsets
- **Comprehensive Logging**: Structured JSON logging with step tracking
- **Result Export**: JSON serialization of complete processing results

## Running the example

```bash
cd examples/advanced
go run main.go
```

This will:
- Process 1000 sample records
- Show real-time execution progress
- Generate detailed metrics and timings
- Export results to `results.json`
- Display the complete pipeline structure

## Expected output

The pipeline will show:
- Parallel execution visualization
- Step-by-step timing information
- Processing metrics and throughput
- Quality analysis results
- Comprehensive error handling
- Final pipeline structure tree

## Files generated

- `results.json`: Complete processing results with metadata

## Key concepts

- **Custom middleware**: Building domain-specific pipeline enhancements
- **Complex merge functions**: Safe data aggregation in parallel workflows
- **Conditional workflows**: Dynamic pipeline behavior based on data conditions
- **Performance monitoring**: Automatic metrics collection and reporting
- **Error resilience**: Graceful degradation and error recovery patterns
