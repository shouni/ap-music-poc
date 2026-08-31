package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"

	"github.com/shouni/go-remote-io/remoteio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStore は書き込み・削除・署名の記録を取るストレージのスタブです。
// PublisherAdapter が 1 つの Store を受け取る形になったため、
// 以前あった testURLSigner の役割もここが兼ねます。
type stubStore struct {
	remoteio.Store

	writes       []string
	deletes      []string
	dataByURI    map[string][]byte
	contentTypes map[string]string
	failOn       map[string]error
	// signCalls は署名を要求された URI の記録です。
	signCalls []string
	// signFailOn は署名の失敗を注入します。
	signFailOn map[string]error
}

// SignURL は署名付き URL を組み立てたことにします。
func (w *stubStore) SignURL(_ context.Context, uri, _ string, _ time.Duration) (string, error) {
	if err, ok := w.signFailOn[uri]; ok {
		return "", err
	}
	w.signCalls = append(w.signCalls, uri)
	return "https://signed.example/" + uri, nil
}

// Write は remoteio.Store の書き込みを模します。
func (w *stubStore) Write(_ context.Context, uri string, contentReader io.Reader, _ ...remoteio.WriteOption) error {
	if err, ok := w.failOn[uri]; ok {
		return err
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, contentReader); err != nil {
		return err
	}

	w.writes = append(w.writes, uri)
	if w.dataByURI == nil {
		w.dataByURI = make(map[string][]byte)
	}
	w.dataByURI[uri] = buf.Bytes()

	if w.contentTypes == nil {
		w.contentTypes = make(map[string]string)
	}

	if strings.HasSuffix(uri, ".json") {
		w.contentTypes[uri] = recipeJSONContentType
	} else if strings.HasSuffix(uri, domain.AudioFileExtension) {
		w.contentTypes[uri] = domain.AudioContentType
	}

	return nil
}

func (w *stubStore) Delete(_ context.Context, uri string) error {
	w.deletes = append(w.deletes, uri)
	return nil
}

// --- Test 関数群 ---

func TestPublisherPublishCleansUpOnRecipeWriteFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{GCSBucket: "bucket"}
	audioURI := "gs://bucket/job-1.mp3"
	recipeURI := "gs://bucket/job-1.json"

	store := &stubStore{failOn: map[string]error{
		recipeURI: errors.New("recipe write failed"),
	}}

	publisher, err := NewPublisherAdapter(cfg, store)
	require.NoError(t, err)

	_, err = publisher.Publish(context.Background(), domain.Task{JobID: "job-1"}, &domain.MusicRecipe{Title: "x"}, []byte("mp3"))
	assert.Error(t, err)

	expectedDeletes := []string{recipeURI, audioURI}
	assert.Equal(t, expectedDeletes, store.deletes)
}

func TestPublisherPublishCleansUpOnSignedURLFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{GCSBucket: "bucket"}
	audioURI := "gs://bucket/job-2.mp3"
	recipeURI := "gs://bucket/job-2.json"

	store := &stubStore{signFailOn: map[string]error{
		audioURI: errors.New("sign failed"),
	}}

	publisher, err := NewPublisherAdapter(cfg, store)
	require.NoError(t, err)

	_, err = publisher.Publish(context.Background(), domain.Task{JobID: "job-2"}, &domain.MusicRecipe{Title: "x"}, []byte("mp3"))
	assert.Error(t, err)

	expectedDeletes := []string{recipeURI, audioURI}
	assert.Equal(t, expectedDeletes, store.deletes)
}

func TestPublisherPublishWritesRecipeJSONAsUTF8(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{GCSBucket: "bucket"}
	recipeURI := "gs://bucket/job-utf8.json"

	store := &stubStore{}

	publisher, err := NewPublisherAdapter(cfg, store)
	require.NoError(t, err)

	recipe := &domain.MusicRecipe{
		Title: "朝焼けのレシピ",
		Theme: "日本語ボーカル",
	}
	_, err = publisher.Publish(context.Background(), domain.Task{JobID: "job-utf8"}, recipe, []byte("mp3"))
	require.NoError(t, err)

	recipeData := store.dataByURI[recipeURI]

	assert.Equal(t, recipeJSONContentType, store.contentTypes[recipeURI])
	assert.True(t, utf8.Valid(recipeData))
	assert.True(t, json.Valid(recipeData))
	assert.Contains(t, string(recipeData), "朝焼けのレシピ")
}
