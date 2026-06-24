package fastembed

import (
	"context"
	"fmt"

	lib "github.com/anush008/fastembed-go"

	"github.com/incu6us/qdrant-embeding/internal/domain"
)

type Embedder struct {
	model     *lib.FlagEmbedding
	dimension int
	batchSize int
}

func New(cacheDir string, maxLength, batchSize int) (*Embedder, error) {
	if batchSize < 1 {
		batchSize = 1
	}
	model, err := lib.NewFlagEmbedding(&lib.InitOptions{
		Model:     lib.BGESmallENV15,
		CacheDir:  cacheDir,
		MaxLength: maxLength,
	})
	if err != nil {
		return nil, fmt.Errorf("init fastembed model: %w", err)
	}

	dimension, err := probeDimension(model, batchSize)
	if err != nil {
		model.Destroy()
		return nil, err
	}

	return &Embedder{model: model, dimension: dimension, batchSize: batchSize}, nil
}

func probeDimension(model *lib.FlagEmbedding, batchSize int) (int, error) {
	vectors, err := model.Embed([]string{"dimension probe"}, batchSize)
	if err != nil {
		return 0, fmt.Errorf("probe embedding dimension: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return 0, fmt.Errorf("probe embedding returned no vector")
	}
	return len(vectors[0]), nil
}

func (e *Embedder) Embed(ctx context.Context, contents []string) ([]domain.Embedding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	vectors, err := e.model.Embed(contents, e.batchSize)
	if err != nil {
		return nil, fmt.Errorf("embed %d documents: %w", len(contents), err)
	}

	embeddings := make([]domain.Embedding, len(vectors))
	for i, vector := range vectors {
		embedding, embErr := domain.NewEmbedding(vector)
		if embErr != nil {
			return nil, embErr
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (e *Embedder) Dimension() int { return e.dimension }

func (e *Embedder) Close() error {
	e.model.Destroy()
	return nil
}
