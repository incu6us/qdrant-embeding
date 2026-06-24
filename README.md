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
- The collection is written in the layout the [official Qdrant MCP server](https://github.com/qdrant/mcp-server-qdrant)
  expects, so the data is searchable from Claude Code with no extra steps
  (see [Use the ingested data in Claude Code](#use-the-ingested-data-in-claude-code)):
  - **named vector** `fast-bge-small-en-v1.5` (the server's `fast-<model>` convention),
  - payload `{ "document": <file text>, "metadata": { "path": <file path> } }`.

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

## Use the ingested data in Claude Code

Claude Code reads the collection through the official
[Qdrant MCP server](https://github.com/qdrant/mcp-server-qdrant), which exposes a
`qdrant-find` tool for semantic search. This ingester writes the collection in exactly the
layout that server reads (named vector `fast-bge-small-en-v1.5`, `document` + `metadata`
payload), so no conversion is needed.

> **Critical:** the MCP server must embed queries with the **same model** used for ingestion.
> Set `EMBEDDING_MODEL=BAAI/bge-small-en-v1.5`. If you leave it at the server's default
> (`all-MiniLM-L6-v2`), query vectors land in a different space / under a different vector
> name and `qdrant-find` returns nothing.

### 1. Register the MCP server

Requires [`uv`](https://docs.astral.sh/uv/) (provides `uvx`). From the repo (or anywhere):

```sh
claude mcp add qdrant \
  -e QDRANT_URL=http://localhost:6333 \
  -e COLLECTION_NAME=markdown \
  -e EMBEDDING_MODEL=BAAI/bge-small-en-v1.5 \
  -e QDRANT_READ_ONLY=true \
  -- uvx mcp-server-qdrant
```

- `QDRANT_READ_ONLY=true` exposes only `qdrant-find` (search), not `qdrant-store` — drop it
  if you also want Claude to write memories into the collection.
- Add `-e QDRANT_API_KEY=...` if your Qdrant requires auth.
- Add `-s project` to write the config to a shared `.mcp.json` instead of your user scope.

Equivalent `.mcp.json` (project scope):

```json
{
  "mcpServers": {
    "qdrant": {
      "command": "uvx",
      "args": ["mcp-server-qdrant"],
      "env": {
        "QDRANT_URL": "http://localhost:6333",
        "COLLECTION_NAME": "markdown",
        "EMBEDDING_MODEL": "BAAI/bge-small-en-v1.5",
        "QDRANT_READ_ONLY": "true"
      }
    }
  }
}
```

### 2. Confirm it is connected

```sh
claude mcp list          # should show "qdrant: ... - ✓ Connected"
```

Inside a Claude Code session, `/mcp` lists the server and its `qdrant-find` tool.

### 3. Ask questions against your markdown

Claude calls `qdrant-find` automatically when a prompt needs the indexed docs. Examples:

```
> Search my notes for what Qdrant is used for.
> Using the qdrant-find tool, what does the markdown say about Go 1.26?
```

The tool returns the matching `document` text plus its `metadata.path`, which Claude then
uses to answer — a minimal RAG loop over your `*.md` files.

> First call downloads the model on the Python side too (FastEmbed pulls
> `bge-small-en-v1.5`), so the first `qdrant-find` may take a few seconds.

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
