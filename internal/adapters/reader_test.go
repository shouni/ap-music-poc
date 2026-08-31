package adapters

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/shouni/ap-music-poc/internal/domain"
)

type fakeContentReader struct {
	content string
}

func (r fakeContentReader) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(r.content)), nil
}

func TestReaderAdapterCollectBuildsPromptFromInputParts(t *testing.T) {
	adapter := &ReaderAdapter{
		contentReader: fakeContentReader{content: "source body"},
	}

	got, err := adapter.Collect(context.Background(), domain.Task{
		RequestURL: "gs://bucket/source.txt",
		InputText:  "user notes",
	})

	require.NoError(t, err)
	require.Equal(t, "[Source URL]\ngs://bucket/source.txt\n\n[Source Content]\nsource body\n\n[User Input]\nuser notes", got.Prompt)
}
