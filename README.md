# qdrant-embeding

Go 1.26 CLI that walks a directory for `*.md` files, embeds each file **locally** with
[FastEmbed](https://github.com/anush008/fastembed-go) (`BAAI/bge-small-en-v1.5`, 384-dim),
and upserts them into [Qdrant](https://github.com/qdrant/qdrant) (v1.18.x) over the REST API.

## Quick start

```sh
# 1. native dependency for FastEmbed
brew install onnxruntime
export ONNX_PATH=/opt/homebrew/lib/libonnxruntime.dylib

# 2. start Qdrant
docker run -d -p 6333:6333 -p 6334:6334 qdrant/qdrant:v1.18.2

# 3. fetch Go deps
go mod download

# 4. ingest the markdown in ./docs into the "markdown" collection
go run . -dir ./docs -collection markdown

# 5. confirm the points landed
curl -s http://localhost:6333/collections/markdown | jq .result.points_count
```

See [Prerequisites](#prerequisites), [Run](#run), and [Flags](#flags) below for detail.

## How it works

```
docs/*.md ──► FastEmbed (ONNX, local) ──► 384-d vector ──► Qdrant REST upsert
```

- **One vector per file** (whole-file chunking). One point per `.md`.
- Point IDs are a deterministic UUIDv5 of the file path, so re-running **updates** existing
  points instead of creating duplicates.
- Payload stored per point: `{ "path": ..., "content": ... }`.

> **Note:** `bge-small-en-v1.5` truncates input at `-max-length` tokens (default 512).
> Files longer than that are truncated to a single vector. For long documents, switch to
> a chunking strategy (split on headings or fixed windows) — because the `DocumentSource`
> port (`internal/infrastructure/filesystem`) is the only place files become documents,
> this is a localized change.

## Prerequisites

1. **ONNX Runtime** native library (FastEmbed runs the model through it):
   ```sh
   brew install onnxruntime
   export ONNX_PATH=/opt/homebrew/lib/libonnxruntime.dylib
   ```
   On Linux: download from the [onnxruntime releases](https://github.com/microsoft/onnxruntime/releases)
   and point `ONNX_PATH` at `libonnxruntime.so`.

2. **Qdrant** running locally:
   ```sh
   docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant:v1.18.2
   ```

## Run

```sh
ONNX_PATH=/opt/homebrew/lib/libonnxruntime.dylib \
  go run . -dir ./docs -collection markdown
```

First run downloads the embedding model (~77 MB) into `-cache` (default `model_cache`).

### Flags

| Flag           | Default                 | Description                                  |
|----------------|-------------------------|----------------------------------------------|
| `-dir`         | `.`                     | Directory scanned recursively for `.md`      |
| `-qdrant`      | `http://localhost:6333` | Qdrant REST base URL (`QDRANT_URL` env)      |
| `-collection`  | `markdown`              | Qdrant collection name                       |
| `-cache`       | `model_cache`           | Embedding model cache directory              |
| `-max-length`  | `512`                   | Max tokens per document before truncation    |
| `-batch`       | `16`                    | Embedding + upsert batch size                |

`QDRANT_API_KEY` is read from the environment and sent as the `api-key` header when set.

## Verify

```sh
curl -s http://localhost:6333/collections/markdown | jq .result.points_count
```

## Architecture (DDD / hexagonal)

Ports & adapters — the domain and application layers depend only on interfaces
(`domain.DocumentSource`, `domain.Embedder`, `domain.VectorRepository`); the
infrastructure layer provides the concrete adapters, wired together in `main.go`.

```
main.go                                       composition root (wires adapters)

internal/domain/                              the model + ubiquitous language
  document.go                                 Document value object (path identity, UUIDv5 ID)
  embedding.go                                Embedding value object
  embedded_document.go                        EmbeddedDocument
  ports.go                                    DocumentSource / Embedder / VectorRepository

internal/application/
  ingest.go                                   IngestService use case (Prepare → Load → Embed → Save)

internal/infrastructure/
  filesystem/source.go                        DocumentSource: walks *.md
  fastembed/embedder.go                       Embedder: local ONNX (bge-small-en-v1.5)
  qdrant/repository.go                         VectorRepository: REST, idempotent Prepare + Save
```

## Tests (BDD)

Behaviour is specified in Gherkin and executed with [godog](https://github.com/cucumber/godog).
The scenarios exercise the `IngestService` against in-memory fakes of the ports —
fast, and they need neither ONNX nor a running Qdrant.

```
features/ingest.feature                       Gherkin scenarios
internal/application/ingest_bdd_test.go        godog step definitions + fakes
internal/domain/document_test.go               table-driven value-object unit tests
```

```sh
go test ./...                 # unit + BDD
go test ./internal/application/ -run TestIngestFeatures -v   # pretty Gherkin output
```
