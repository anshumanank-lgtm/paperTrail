# Papertrail

Papertrail is a personal document archive engine that scans a folder, understands supported documents, and produces a normalized document representation.

The current engine is focused on the core ingestion and understanding pipeline. The UI, persistent indexing, OCR, semantic search, and cloud functionality are not part of the current POC.

## Current design

Papertrail processes documents through the following flow:

    Folder
       ↓
    Scanner
       ↓
    Worker pool
       ↓
    Converter
       ↓
    ExtractedContent
       ↓
    MetadataEngine
       ↓
    Document
       ↓
    Output

The pipeline uses a fan-out/fan-in worker pattern so files can be processed concurrently.

## Current support

- Recursive folder scanning and file metadata collection
- TXT text extraction
- PDF embedded-text extraction
- PDF page-count extraction
- Converter dispatch based on file extension
- Concurrent document processing using a worker pool
- Normalized document model separating file metadata from document metadata
- Type-specific document metadata models
- Basic metadata extraction through the MetadataEngine interface

The scanner recognizes additional office-document extensions, but converters for those formats have not been added yet.

Image-only PDFs are not OCR'd.

## Document model

Each processed file becomes a normalized `Document` containing two distinct parts:

- `FileMetadata` — information about the source file
- `DocumentMetadata` — semantic information extracted from the document

`DocumentMetadata` contains:

- Title
- Document type
- Entities
- Type-specific metadata

Supported document types currently include:

- Invoice
- Receipt
- Bill
- Statement
- Contract
- Insurance policy
- Ticket
- Identity
- Payslip
- Tax
- Warranty
- Other
- Unknown

Unknown and other documents can retain generic topics and keywords.

## Run

From the repository root:

```bash
go run ./cmd/papertrail /path/to/folder