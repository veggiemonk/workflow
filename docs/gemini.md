# Component: Documentation

## Purpose

This directory contains the documentation for the `workflow` engine. It provides information for developers who want to understand, use, or contribute to the project.

## Key Files

-   **`architecture.md`**: Provides a high-level overview of the workflow engine's architecture, including its core abstractions (`Step`, `Pipeline`, `Middleware`, etc.) and design principles.
-   **`best-practices.md`**: Offers guidance and examples on the recommended patterns for using the workflow engine, as well as anti-patterns to avoid.
-   **`gopls.md`**: Contains instructions and best practices for using the `gopls` MCP server for Go development within this project.
-   **`llms.md`**: This file contains the auto-generated documentation for the public API of the `workflow` package, created using `gomarkdoc`.

## Dependencies

### External

This component has no external dependencies.

### Internal

The documentation in this directory is for the core workflow engine, so it is conceptually dependent on the code in the root directory.

## Interactions

The `docs` directory does not interact with the application at runtime. It serves as a source of information for developers. The `llms.md` file is generated from the source code in the root directory using the `gomarkdoc` tool, as defined in the `Makefile`.

## Important Notes

-   **Auto-generated Content**: The `llms.md` file is automatically generated and should not be edited directly. Any changes to the public API documentation should be made by updating the source code comments and then regenerating the file using the `make docs` command (which runs `gomarkdoc`).
