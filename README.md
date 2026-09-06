# Papertrail

Papertrail is a privacy-first personal document archive engine.

It scans local documents, extracts their content, understands them using local intelligence, and builds a normalized local index.

The product is **local-first and desktop-first**, with Windows as the initial target. Original files remain the source of truth; Papertrail stores derived metadata, indexes, entities, and relationships locally.

## 1. Design

### Core engine

The core engine is format-independent and sits above the filesystem:

```text
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

Converters turn supported files into `ExtractedContent`. The domain layer then represents each file as a normalized `Document`, separating filesystem metadata from document metadata.

### Metadata / intelligence

The intelligence layer classifies document types and extracts entities using local ML. Go owns the domain model and confidence thresholds; Python currently runs GLiNER2.

### SQLite storage

SQLite stores the derived document index, including documents, entities, and relationships. The filesystem remains the source of truth.

### Entity + relationship model

Documents contain typed entities such as people, organisations, locations, dates, money, products, and IDs. Relationships are derived from meaningful shared entities, providing explainable connections between documents.

### Current Python / GLiNER boundary

The current POC uses a persistent local Python process and JSON Lines for Go ↔ Python communication. GLiNER2 is loaded by that process. This is a temporary boundary and will be replaced by local gRPC.

## 2. Usage

### Setup

Create the Python environment and install intelligence dependencies:

```bash
make venv
```

### Build

```bash
make build
```

### Run against a folder

```bash
make run ARGS=./test
```

`make run` builds the application first. `make scan ARGS=./test` is also available as an alias.

### Supported formats

Currently converted:

- TXT
- PDF

The scanner also recognizes DOC, DOCX, XLS, XLSX, PPT, and PPTX, but converters for those formats are not implemented yet.

### Current CLI output

The CLI currently scans the folder, converts documents, extracts metadata and entities, stores the results in SQLite, and creates document relationships. It reports pipeline progress and processing results to the console.

Other useful commands:

```bash
make test
make sca
make clean
```

`make sca` runs golangci-lint v2.13.2. `make clean` removes the built binary and `index.db`.

## 3. TODO

- Replace JSON Lines with a local gRPC Go ↔ Python intelligence interface
- Initialize GLiNER once at Python server startup
- Support concurrent gRPC requests
- Add batched inference
- Tie intelligence batch size to pipeline `maxWorkers`
- Add proper startup/readiness handling
- Tune performance and intelligence quality
