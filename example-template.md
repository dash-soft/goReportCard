# Report Template Example

__author__: Julian R.
__date__: 2025-11-26
__project__: Example Project

## Overview

This is an example of using template placeholders in your markdown files. The `{{name}}` and `{{date}}` placeholders will be automatically replaced when generating the PDF.

## Benefits

- No hardcoded names in the code
- Flexible author information via environment variables
- Consistent date formatting

## Usage

Set your name:
```bash
export REPORT_AUTHOR="Your Full Name"
./main example-template.md output.pdf
```

Or use `{{author}}` instead of `{{name}}` for clarity.
