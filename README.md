# PaperTrail

PaperTrail is a privacy-first personal document archive engine.

It scans local documents, extracts their content, understands them using local intelligence, and builds a normalized local index.

PaperTrail is **local-first and desktop-first**. The current development/runtime environment is Linux/Unix; Windows is the target platform for v0.2 bundling. Original files remain the source of truth; PaperTrail stores derived metadata, indexes, entities, and relationships locally.

## 1. Design

PaperTrail separates document ingestion, intelligence, storage, and search.

### Core architecture

```text
                ┌─────────────┐
                │    Folder   │
                └──────┬──────┘
                       ↓
                ┌─────────────┐
                │   Scanner   │
                └──────┬──────┘
                       ↓
                ┌─────────────┐
                │   Pipeline  │
                └──────┬──────┘
                       ↓
                ┌─────────────┐
                │  Converter  │
                └──────┬──────┘
                       ↓
                ┌─────────────┐
                │   SQLite    │
                │   Document  │
                └──────┬──────┘
                       │
                       ↓
              ┌─────────────────┐
              │ Intelligence    │
              │ Queue / Worker  │
              └────────┬────────┘
                       ↓
                ┌─────────────┐
                │   GLiNER2   │
                │   (Python)  │
                └──────┬──────┘
                       ↓
                ┌─────────────┐
                │    Go       │
                │  Metadata + │
                │  Relations  │
                └──────┬──────┘
                       ↓
                    SQLite
```

### Document flow

* Scanner detects new, modified, and deleted files.
* Converter produces `ExtractedContent`.
* A basic document is persisted immediately.
* Intelligence runs asynchronously and updates derived metadata and entities.
* Relationships are rebuilt after intelligence completes.

### Storage

SQLite stores the local index:

* Documents
* Entities
* Document ↔ entity mappings
* Relationships
* Intelligence/indexing state

Original files remain the source of truth.

### Search

```text
UI / CLI
   ↓
Search Service
   ↓
SQLite
```

The search layer handles query routing; storage handles SQL.

### UI

The current Fyne UI is a simple inspection/demo tool for search, document metadata, extracted information, and related documents.

## 2. Intelligence Architecture

Turns raw document text into structured, searchable information, kept as a replaceable subsystem behind a stable Go ↔ Python boundary.

### Built With

* **GLiNER2** (`fastino/gliner2-base-v1`) — zero/few-shot entity extraction & classification
* **gRPC + Protocol Buffers** — Go ↔ Python service boundary
* **Python 3** — intelligence server; GLiNER2 loaded once at startup, not per document
* **Go** — confidence policy, filtering, normalization, persistence

### How it works

* One call returns title, document type, and entities together (`ExtractResponse`)
* Confidence thresholds gate what gets persisted: title `0.50`, document type `0.70` (else `other`), entities `0.80`
* Long documents use GLiNER2's `extract_long()` with `chunk_size=384` / `chunk_overlap=64` so entities spanning chunk boundaries aren't lost
* Runs entirely on `localhost` — no document content leaves the machine

## 3. Usage

### Prerequisites

* Go 1.27.1
* Python 3
* Python virtual environment support (`python3-venv`)

PaperTrail currently targets Linux/Unix for development. Windows packaging is planned for v0.2.

### Setup

```
make setup
```

Creates `.venv` and installs dependencies from `internal/intelligence/requirements.txt`.

The first startup downloads/caches the configured GLiNER2 model if it is not already available locally. This requires network access and additional local disk space; subsequent starts use the cached model but still incur model loading time.

### Build

```
make build
```

Binary is created at `bin/papertrail`.

### Run / Scan

The current executable launches the Fyne inspection UI and takes the folder to index as its argument:

```
make run ARGS=./test
```

`make scan` is an alias for the same command.

### Tests & Static Analysis

```
make test
make sca   # golangci-lint v2.13.2
```

### Clean

```
make clean        # removes binary and local SQLite database
make deep-clean   # also removes .venv; re-run `make setup && make build` after
```

## 4. Supported Formats

Currently converted: **TXT, PDF**. PDF support currently extracts embedded text; scanned/image-only PDFs require OCR, which is not yet included.

Recognized but not yet converted (planned for v0.2): DOC, DOCX, XLS, XLSX, PPT, PPTX

All converters feed the same pipeline: `ExtractedContent → Intelligence → Document → SQLite`

## 5. v0.1 — Complete

v0.1 establishes the core local document intelligence engine and search foundation.

* Format-independent scanning with concurrent fan-out/fan-in pipeline
* TXT and PDF conversion, including PDF Author/Creator metadata extraction
* Normalized document model persisted to SQLite
* Go ↔ Python gRPC intelligence service running GLiNER2
* Document title extraction and document-type classification
* Confidence-based intelligence filtering
* Long-document extraction using GLiNER2 `extract_long()` with overlap handling
* Canonical entity storage, normalization, deduplication, and document↔entity mapping
* Explainable document relationships derived from shared entities and metadata
* Incremental indexing with file fingerprinting and detection of new, modified, and deleted files
* Two-phase persistence with asynchronous intelligence processing
* Search across filename, date, author, creator, extracted intelligence/entities, and file type
* Simple Fyne-based search/inspection UI showing metadata, extracted information, and related documents
* SQLite WAL, protected writes, foreign keys, indexes, graceful shutdown, and pipeline error handling

## 6. v0.2 — Intelligence + Packaging

The next release focuses on expanding PaperTrail beyond the v0.1 core.

* **Additional file types** — DOC, DOCX, XLS, XLSX, PPT, PPTX, feeding the existing conversion and intelligence pipeline
* **LLM integration** — add an LLM layer for document Q&A, summaries, multi-document questions, comparisons, and Ask PaperTrail while keeping local retrieval and intelligence as the foundation
* **Windows bundling** — package the Go application, Python runtime/dependencies, and required ML assets so PaperTrail can run on Windows without manual Python/virtual-environment setup

