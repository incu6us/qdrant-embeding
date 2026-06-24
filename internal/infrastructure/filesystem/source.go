package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/incu6us/qdrant-embeding/internal/domain"
)

type MarkdownSource struct {
	root string
}

func NewMarkdownSource(root string) *MarkdownSource {
	return &MarkdownSource{root: root}
}

func (s *MarkdownSource) Load(ctx context.Context) ([]domain.Document, error) {
	var documents []domain.Document
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		if strings.TrimSpace(string(content)) == "" {
			return nil
		}

		document, docErr := domain.NewDocument(path, string(content))
		if docErr != nil {
			return fmt.Errorf("build document %s: %w", path, docErr)
		}
		documents = append(documents, document)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", s.root, err)
	}
	return documents, nil
}
