# Pipeline Serialization and Deserialization Example

This example demonstrates how to serialize a pipeline to a JSON file and deserialize it back into a runnable pipeline.

## What it does

1.  **Register Components**: Registers tasks, selectors, and merge functions in a `Registry`.
2.  **Build Pipeline**: Programmatically builds a new pipeline.
3.  **Serialize**: Saves the pipeline to a `pipeline.json` file.
4.  **Deserialize**: Loads the pipeline from the `pipeline.json` file using a `Builder`.
5.  **Execute**: Runs the loaded pipeline and prints the result.

## Features Demonstrated

- **Pipeline Serialization**: Saving a pipeline definition to a JSON file.
- **Pipeline Deserialization**: Loading a pipeline from a JSON file.
- **Task Registry**: Using a registry to make tasks available for deserialization.
- **Dynamic Pipelines**: Building pipelines from a stored specification.

## Running the example

```bash
cd examples/serialization
go run main.go
```

## Expected output

The example will:
- Build a pipeline and save it to `pipeline.json`.
- Load the pipeline from `pipeline.json`.
- Run the loaded pipeline.
- Print the final result of the pipeline execution.

## Files generated

- `pipeline.json`: The JSON representation of the pipeline.

## Key concepts

- **Serialization**: Converting a pipeline object into a format that can be stored or transmitted.
- **Deserialization**: Reconstructing a pipeline object from a stored format.
- **Registry**: A central place to store and retrieve tasks, selectors, and merge functions by name.
- **Builder**: A tool to construct a pipeline from a specification.
