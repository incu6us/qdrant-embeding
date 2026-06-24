package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/incu6us/qdrant-embeding/internal/application"
	"github.com/incu6us/qdrant-embeding/internal/infrastructure/fastembed"
	"github.com/incu6us/qdrant-embeding/internal/infrastructure/filesystem"
	"github.com/incu6us/qdrant-embeding/internal/infrastructure/qdrant"
)

func main() {
	dir := flag.String("dir", ".", "directory to scan recursively for .md files")
	qdrantURL := flag.String("qdrant", envOr("QDRANT_URL", "http://localhost:6333"), "Qdrant REST base URL")
	collection := flag.String("collection", "markdown", "Qdrant collection name")
	cacheDir := flag.String("cache", "model_cache", "directory to cache the embedding model")
	maxLength := flag.Int("max-length", 512, "max token length per document (longer text is truncated)")
	batchSize := flag.Int("batch", 16, "embedding and upsert batch size")
	flag.Parse()

	if err := run(*dir, *qdrantURL, *collection, *cacheDir, *maxLength, *batchSize); err != nil {
		log.Fatalf("ingest failed: %v", err)
	}
}

func run(dir, qdrantURL, collection, cacheDir string, maxLength, batchSize int) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Printf("loading embedding model (cache=%s)...", cacheDir)
	embedder, err := fastembed.New(cacheDir, maxLength, batchSize)
	if err != nil {
		return err
	}
	defer embedder.Close()
	log.Printf("model ready, vector dimension=%d", embedder.Dimension())

	source := filesystem.NewMarkdownSource(dir)
	repo := qdrant.NewRepository(qdrantURL, os.Getenv("QDRANT_API_KEY"), collection)
	service := application.NewIngestService(source, embedder, repo, batchSize)

	log.Printf("ingesting *.md from %q into collection %q at %s", dir, collection, qdrantURL)
	result, err := service.Ingest(ctx)
	if err != nil {
		return err
	}
	log.Printf("done: ingested %d markdown file(s)", result.Ingested)
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
