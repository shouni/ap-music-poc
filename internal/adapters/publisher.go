package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/shouni/go-remote-io/remoteio"

	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
)

const (
	recipeJSONContentType = "application/json; charset=utf-8"
	defaultCacheControl   = "public, max-age=1800"
)

// PublisherAdapter は成果物保存を行うアダプターです。
type PublisherAdapter struct {
	store      remoteio.Store
	Bucket     string
	Expiration time.Duration
}

// NewPublisherAdapter は成果物保存のためのアダプターを生成します。
func NewPublisherAdapter(cfg *config.Config, store remoteio.Store) (*PublisherAdapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	return &PublisherAdapter{
		store:      store,
		Bucket:     cfg.GCSBucket,
		Expiration: config.SignedURLExpiration,
	}, nil
}

// Publish は成果物をストレージに保存し、その結果（署名付きURL等）を返します。
func (a *PublisherAdapter) Publish(ctx context.Context, task domain.Task, recipe *domain.MusicRecipe, audioData []byte) (*domain.PublishResult, error) {
	if recipe == nil {
		return nil, fmt.Errorf("recipe cannot be nil")
	}
	if task.JobID == "" {
		return nil, fmt.Errorf("job id is required")
	}
	if len(audioData) == 0 {
		return nil, fmt.Errorf("output file is empty")
	}

	storageURI := remoteio.BuildURI(remoteio.SchemeGCS, a.Bucket, fmt.Sprintf("%s%s", task.JobID, domain.AudioFileExtension))
	recipeStorageURI := remoteio.BuildURI(remoteio.SchemeGCS, a.Bucket, fmt.Sprintf("%s.json", task.JobID))

	// 1. 音声データの書き込み（Cache-Control を適用）
	contentReader := bytes.NewReader(audioData)
	if err := a.store.Write(ctx, storageURI, contentReader,
		remoteio.WithContentType(domain.AudioContentType),
		remoteio.WithInline(),
		remoteio.WithCacheControl(defaultCacheControl),
	); err != nil {
		return nil, fmt.Errorf("failed to write audio to storage: %w", err)
	}

	// 2. レシピJSONの生成
	recipeData, err := json.Marshal(recipe)
	if err != nil {
		a.cleanupOnFailure(ctx, storageURI)
		return nil, fmt.Errorf("failed to marshal recipe json: %w", err)
	}

	// 3. レシピJSONの書き込み（Cache-Control を適用）
	recipeReader := bytes.NewReader(recipeData)
	if err := a.store.Write(ctx, recipeStorageURI, recipeReader,
		remoteio.WithContentType(recipeJSONContentType),
		remoteio.WithCacheControl(defaultCacheControl),
	); err != nil {
		a.cleanupOnFailure(ctx, recipeStorageURI, storageURI)
		return nil, fmt.Errorf("failed to write recipe to storage (audio file %s was already written): %w", storageURI, err)
	}

	// 4. 各種署名付きURLの生成
	signedURL, err := a.generateSignedResultURL(ctx, storageURI)
	if err != nil {
		a.cleanupOnFailure(ctx, recipeStorageURI, storageURI)
		return nil, fmt.Errorf("failed to generate audio signed URL: %w", err)
	}
	recipeSignedURL, err := a.generateSignedResultURL(ctx, recipeStorageURI)
	if err != nil {
		a.cleanupOnFailure(ctx, recipeStorageURI, storageURI)
		return nil, fmt.Errorf("failed to generate recipe signed URL: %w", err)
	}

	return &domain.PublishResult{
		JobID:            task.JobID,
		StorageURI:       storageURI,
		SignedURL:        signedURL,
		RecipeStorageURI: recipeStorageURI,
		RecipeSignedURL:  recipeSignedURL,
	}, nil
}

// generateSignedResultURL は StorageURI から署名付きURLを作るヘルパーです。
func (a *PublisherAdapter) generateSignedResultURL(ctx context.Context, storageURI string) (string, error) {
	return a.store.SignURL(ctx, storageURI, http.MethodGet, a.Expiration)
}

// cleanupOnFailure は失敗時にアーティファクトをクリーンアップするヘルパーです。
func (a *PublisherAdapter) cleanupOnFailure(ctx context.Context, uris ...string) {
	if a.store == nil {
		return
	}

	for _, uri := range uris {
		if uri == "" {
			continue
		}
		if err := a.store.Delete(ctx, uri); err != nil {
			slog.WarnContext(ctx, "failed to cleanup partial artifact", "uri", uri, "error", err)
		}
	}
}
