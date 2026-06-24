package application

import (
	"context"
	"fmt"

	"github.com/incu6us/qdrant-embeding/internal/domain"
)

type IngestResult struct {
	Ingested int
}

type IngestService struct {
	source    domain.DocumentSource
	embedder  domain.Embedder
	repo      domain.VectorRepository
	batchSize int
}

func NewIngestService(
	source domain.DocumentSource,
	embedder domain.Embedder,
	repo domain.VectorRepository,
	batchSize int,
) *IngestService {
	if batchSize < 1 {
		batchSize = 1
	}
	return &IngestService{
		source:    source,
		embedder:  embedder,
		repo:      repo,
		batchSize: batchSize,
	}
}

func (s *IngestService) Ingest(ctx context.Context) (IngestResult, error) {
	if err := s.repo.Prepare(ctx, s.embedder.Dimension()); err != nil {
		return IngestResult{}, err
	}

	documents, err := s.source.Load(ctx)
	if err != nil {
		return IngestResult{}, err
	}

	total := 0
	for start := 0; start < len(documents); start += s.batchSize {
		end := min(start+s.batchSize, len(documents))
		saved, err := s.ingestBatch(ctx, documents[start:end])
		if err != nil {
			return IngestResult{Ingested: total}, err
		}
		total += saved
	}
	return IngestResult{Ingested: total}, nil
}

func (s *IngestService) ingestBatch(ctx context.Context, documents []domain.Document) (int, error) {
	contents := make([]string, len(documents))
	for i, d := range documents {
		contents[i] = d.Content()
	}

	embeddings, err := s.embedder.Embed(ctx, contents)
	if err != nil {
		return 0, err
	}
	if len(embeddings) != len(documents) {
		return 0, fmt.Errorf("embedding count mismatch: got %d for %d documents", len(embeddings), len(documents))
	}

	embedded := make([]domain.EmbeddedDocument, len(documents))
	for i := range documents {
		embedded[i] = domain.EmbeddedDocument{Document: documents[i], Embedding: embeddings[i]}
	}

	if err := s.repo.Save(ctx, embedded); err != nil {
		return 0, err
	}
	return len(documents), nil
}
