package domain

import "context"

type DocumentSource interface {
	Load(ctx context.Context) ([]Document, error)
}

type Embedder interface {
	Embed(ctx context.Context, contents []string) ([]Embedding, error)
	Dimension() int
}

type VectorRepository interface {
	Prepare(ctx context.Context, dimension int) error
	Save(ctx context.Context, documents []EmbeddedDocument) error
}
