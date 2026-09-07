# PaperTrail v0.1

PaperTrail is a privacy-first personal document archive engine.

It scans local documents, extracts their content, understands them using local intelligence, and builds a normalized local index.

The product is **local-first and desktop-first**, with Windows as the initial target. Original files remain the source of truth; PaperTrail stores derived metadata, indexes, entities, and relationships locally.

## 1. Design

### Core engine

The core engine is format-independent and sits above the filesystem:

```
Folder
  ↓
Scanner
  ↓
Pipeline / Worker Pool
  ↓
Converter
  ↓
ExtractedContent
  ↓
Metadata / Intelligence
  ↓
Normalized Document
  ↓
SQLite Index
```

### Pipeline / worker pool

The `pipeline` package owns orchestration. It uses a fan-out/fan-in worker pool so multiple documents can be processed concurrently, with `maxWorkers` controlling concurrency.

### Document normalization

Converters turn supported files into `ExtractedContent`. The domain layer represents each file as a normalized `Document`, separating filesystem metadata from document metadata.

### Metadata / intelligence

The intelligence layer classifies document types and extracts entities using local ML. Go owns the domain model and confidence thresholds; Python runs GLiNER2 through a local gRPC service.

GLiNER2 is initialized once when the intelligence server starts and supports long-document extraction through chunking and overlap handling.

### SQLite storage

SQLite stores the derived document index, including documents, entities, and relationships. The filesystem remains the source of truth.

### Entity + relationship model

Documents contain typed entities such as people, organisations, locations, dates, money, products, vehicles, addresses, and IDs.

Entities are normalized and deduplicated in storage. A `document_entities` mapping connects documents to canonical entities, allowing the UI and relationship engine to navigate from a document to its entities and from an entity to related documents.

Relationships are derived from meaningful shared entities and document metadata such as author/creator, providing explainable connections between documents.

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

* Go
* Python 3
* Python virtual environment support (`python3-venv`)

### Setup

```
make setup
```

Creates `.venv` and installs dependencies from `internal/intelligence/requirements.txt`.

### Build

```
make build
```

Binary is created at `bin/papertrail`.

### Run / Scan

```
make run ARGS=./test
make scan ARGS=./test   # alias
```

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

Currently converted: **TXT, PDF**

Recognized but not yet converted (planned for v0.2): DOC, DOCX, XLS, XLSX, PPT, PPTX

All converters feed the same pipeline: `ExtractedContent → Intelligence → Document → SQLite`

## 5. v0.1 — Achieved

* Format-independent scanning with concurrent fan-out/fan-in pipeline
* TXT and PDF converters, with PDF Author/Creator metadata extraction
* Normalized document model persisted to SQLite
* Go ↔ Python gRPC intelligence service running GLiNER2, with type classification, confidence thresholds, and chunked/overlap-aware long-document entity extraction
* Canonical entity storage with normalization, deduplication, and document↔entity mapping
* Entity- and metadata-derived document relationships (shared org/product/ID/event/address/person/author), prioritized in related-document results
* Reliability foundations: SQLite WAL mode, protected concurrent writes, foreign keys with cascading cleanup, lookup indexes, graceful shutdown, pipeline error handling, tests/lint setup, and 5-worker performance validation

## 6. v0.2 — Persistent Archive + Product Foundation

* **Continuous indexing** — fingerprinting and incremental scans to detect new/modified/deleted files without reprocessing unchanged ones; long-running scanner with configurable interval and error resilience
* **Additional converters** — DOC, DOCX, XLS, XLSX, PPT, PPTX, feeding the existing pipeline
* **Intelligence/search improvements** — driven by real-document testing: better entity resolution, relationship semantics, and structured queries (date, amount/range, etc.) as needed
* **Local summarization research** — find the smallest viable local summarization model (e.g. FLAN-T5-small), tested on real documents, before deciding between indexed vs. on-demand summaries; no persistent `summary` field until then
* **Product foundation** — stable backend/API contract for the UI, remaining data-model fixes, and reliability/resource-usage testing

