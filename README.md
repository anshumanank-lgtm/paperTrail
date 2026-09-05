# Papertrail

Papertrail scans a folder and converts supported files into a small, normalized
JSON document representation.

## Current support

- Recursive folder scanning and file metadata collection
- Plain-text extraction from `.txt` files
- Embedded-text extraction and page counts for `.pdf` files

The scanner recognizes additional office-document extensions, but their
converters have not been added yet. Image-only PDFs are not OCR'd.

## Run

From the repository root, scan the included example folder:

```bash
go run . ./test
```

Or pass any folder path as the only argument:

```bash
go run . /path/to/folder
```

The standard Go command path works too:

```bash
go run ./cmd/papertrail /path/to/folder
```

Converted documents are written as JSON to standard output. File metadata is
kept separately from normalized document metadata. Files that cannot currently
be converted are reported on standard error without stopping the remaining
files.

For example, to save the JSON from the included fixture:

```bash
go run . ./test > documents.json
```

The `test/test.txt` fixture demonstrates TXT metadata extraction, including a
title, policy identifier, person and vehicle entities, dates, and INR amounts.

## Test

Run the full test suite from the repository root:

```bash
go test ./...
```

## Example output

```json
[
  {
    "file_metadata": {
      "source_path": "/path/to/folder/test.txt",
      "filename": "test.txt",
      "extension": ".txt",
      "size": 738,
      "modified_at": "2026-09-02T12:00:00Z"
    },
    "document_metadata": {
      "title": "HDFC Ergo Car Insurance Policy",
      "document_type": "unknown",
      "identifiers": [
        { "type": "policy", "value": "HDFC-MOTOR-2026-458921" }
      ],
      "amounts": [
        { "type": "premium", "value": 18450, "currency": "INR" }
      ]
    }
  }
]
```

## Project layout

```text
cmd/papertrail/  Command entrypoint
internal/cli/    Shared command-line wiring
scanner/         Recursive file discovery and metadata
converter/       Document model and TXT/PDF converters
```
