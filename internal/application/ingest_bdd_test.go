package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"

	"github.com/incu6us/qdrant-embeding/internal/application"
	"github.com/incu6us/qdrant-embeding/internal/domain"
)

type fakeSource struct {
	documents []domain.Document
}

func (f *fakeSource) Load(context.Context) ([]domain.Document, error) {
	return f.documents, nil
}

type fakeEmbedder struct {
	dimension int
}

func (f *fakeEmbedder) Dimension() int { return f.dimension }

func (f *fakeEmbedder) Embed(_ context.Context, contents []string) ([]domain.Embedding, error) {
	embeddings := make([]domain.Embedding, len(contents))
	for i := range contents {
		embedding, err := domain.NewEmbedding(make([]float32, f.dimension))
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

type fakeRepository struct {
	preparedDimension int
	saved             map[string]domain.EmbeddedDocument
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{saved: map[string]domain.EmbeddedDocument{}}
}

func (r *fakeRepository) Prepare(_ context.Context, dimension int) error {
	r.preparedDimension = dimension
	return nil
}

func (r *fakeRepository) Save(_ context.Context, documents []domain.EmbeddedDocument) error {
	for _, ed := range documents {
		r.saved[ed.Document.ID()] = ed
	}
	return nil
}

type ingestWorld struct {
	source   *fakeSource
	embedder *fakeEmbedder
	repo     *fakeRepository
	result   application.IngestResult
	err      error
}

func (w *ingestWorld) anEmbedderProducingDimensionalVectors(dimension int) error {
	w.embedder = &fakeEmbedder{dimension: dimension}
	return nil
}

func (w *ingestWorld) anEmptyVectorStore() error {
	w.repo = newFakeRepository()
	return nil
}

func (w *ingestWorld) aSourceContainingMarkdownDocuments(count int) error {
	documents := make([]domain.Document, 0, count)
	for i := 1; i <= count; i++ {
		document, err := domain.NewDocument(
			fmt.Sprintf("docs/doc-%d.md", i),
			fmt.Sprintf("# Document %d\nbody content %d", i, i),
		)
		if err != nil {
			return err
		}
		documents = append(documents, document)
	}
	w.source = &fakeSource{documents: documents}
	return nil
}

func (w *ingestWorld) iRunTheIngestion() error {
	service := application.NewIngestService(w.source, w.embedder, w.repo, 16)
	w.result, w.err = service.Ingest(context.Background())
	return w.err
}

func (w *ingestWorld) iRunTheIngestionAgain() error {
	return w.iRunTheIngestion()
}

func (w *ingestWorld) documentsShouldBeReportedAsIngested(count int) error {
	if w.result.Ingested != count {
		return fmt.Errorf("expected %d ingested, got %d", count, w.result.Ingested)
	}
	return nil
}

func (w *ingestWorld) theVectorStoreShouldContainEmbeddedDocuments(count int) error {
	if len(w.repo.saved) != count {
		return fmt.Errorf("expected %d stored documents, got %d", count, len(w.repo.saved))
	}
	return nil
}

func (w *ingestWorld) theVectorStoreShouldBePreparedWithDimension(dimension int) error {
	if w.repo.preparedDimension != dimension {
		return fmt.Errorf("expected prepared dimension %d, got %d", dimension, w.repo.preparedDimension)
	}
	return nil
}

func initializeScenario(ctx *godog.ScenarioContext) {
	world := &ingestWorld{}

	ctx.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		*world = ingestWorld{}
		return ctx, nil
	})

	ctx.Step(`^an embedder producing (\d+)-dimensional vectors$`, world.anEmbedderProducingDimensionalVectors)
	ctx.Step(`^an empty vector store$`, world.anEmptyVectorStore)
	ctx.Step(`^a source containing (\d+) markdown documents$`, world.aSourceContainingMarkdownDocuments)
	ctx.Step(`^I run the ingestion$`, world.iRunTheIngestion)
	ctx.Step(`^I run the ingestion again$`, world.iRunTheIngestionAgain)
	ctx.Step(`^(\d+) documents should be reported as ingested$`, world.documentsShouldBeReportedAsIngested)
	ctx.Step(`^the vector store should contain (\d+) embedded documents$`, world.theVectorStoreShouldContainEmbeddedDocuments)
	ctx.Step(`^the vector store should be prepared with dimension (\d+)$`, world.theVectorStoreShouldBePreparedWithDimension)
}

func TestIngestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../../features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("BDD feature scenarios failed")
	}
}
