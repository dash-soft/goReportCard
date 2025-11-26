# Report Generator

A tool for converting Markdown files to professional PDF reports with custom formatting, fonts, and logo embedding.

## Usage

```bash
./main <input.md> <output.pdf>
```

## Markdown Formatting Guide

### Metadata Variables

The tool extracts metadata from your markdown files using special variables. Place these variables at the top of your markdown file (after the title):

```markdown
# Your Report Title

__author__: Your Name Here
__date__: Date or time period
__project__: Project or department name

## Content starts here
...
```

### Supported Variables

- `__author__`: The author/creator of the report
- `__date__`: Date, time period, or version information
- `__project__`: Project name, department, or company information

### Variable Format

Variables must be formatted exactly as:
```
__variable_name__: value
```

Examples:

```markdown
# Daily Standup Report

__author__: Julian R.
__date__: 2025-11-26
__project__: Novelized Platform

## What Was Completed Yesterday
- Task 1
- Task 2
```

```markdown
# Incident Report

__author__: Julian R.
__date__: 2025-01-22
__project__: Network Infrastructure

## Executive Summary
...
```

```markdown
# Weekly Summary

__author__: Julian R.
__date__: Week 47 / 2025
__project__: Platform Engineering - Novelized

## Highlights of the Week
...
```

### PDF Metadata

The extracted variables are automatically embedded in the PDF metadata:
- **Author**: Set as PDF author
- **Date**: Used for creation date
- **Project**: Set as PDF title and subject

This allows PDF viewers and document management systems to properly index and search your reports.

### Code Blocks and Inline Code

Code blocks and inline code are fully supported with appropriate formatting:

#### Code Blocks

Use standard Markdown code blocks with optional language specification:

````markdown
```javascript
function greet(name) {
    console.log(`Hello, ${name}!`);
}
```

```bash
echo "Hello World"
```
````

Code blocks are rendered with:
- Monospace font (Maple Mono)
- Light gray background
- Proper line spacing

#### Inline Code

Inline code spans are rendered with:
- Monospace font
- Light gray background
- Appropriate padding

Example: Use `console.log()` for debugging.

## Examples

See the included example reports:
- `standup.md` - Daily standup report format
- `incident.md` - Incident report format
- `weekly-summery.md` - Weekly summary format

## Building

```bash
go build -o report ./cmd/app
```

Or use the Makefile:

```bash
make build
```

## Development

### Prerequisites

- Go 1.21 or later
- Make (optional, for using the Makefile)

### Development Setup

```bash
make dev-setup    # Install dependencies and development tools
make dev          # Run with live reload (requires air)
make build        # Build the binary
make fmt          # Format code
make vet          # Run go vet
make check        # Run all checks (formatting, vet)
make clean        # Clean build artifacts
make example      # Generate example PDF from showcase.md
```

### Code Quality

```bash
make fmt          # Format code
make vet          # Run go vet
make check-fmt    # Check formatting
```

## CI/CD

This project uses GitHub Actions for continuous integration and deployment:

### Build Verification
- Runs on every push and pull request to main/master
- Builds against multiple Go versions (1.21.x, 1.22.x, 1.23.x)
- Runs on Ubuntu, macOS, and Windows
- Includes code formatting and static analysis checks
- Verifies that the binary can successfully generate PDFs

### Releases
- Triggered by pushing tags starting with `v*`
- Builds binaries for:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64)
  - Windows (amd64, 386)
- Creates GitHub releases with downloadable binaries
- Includes SHA256 checksums

## Features

- Markdown to PDF conversion
- Custom font embedding (Maple Mono)
- Logo embedding
- PDF metadata embedding (__author__, __date__, __project__)
- Metadata variable extraction from markdown
- Professional formatting
- Support for headings, lists, code blocks, inline code, and tables
- Syntax highlighting for code blocks
- Cross-platform CI/CD
