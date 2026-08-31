// Package builder は、設定値から各サービスクライアント・ハンドラーの
// 依存関係を組み立てるファクトリ関数を提供します。
package builder

import (
	"context"
	"fmt"
	"io"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio/gcs"

	"github.com/shouni/ap-music-poc/internal/adapters"
	"github.com/shouni/ap-music-poc/internal/app"
	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/repository"
)

// BuildContainer は外部サービスとの接続を確立し、依存関係を組み立てた app.Container を返します。
func BuildContainer(ctx context.Context, cfg *config.Config) (container *app.Container, err error) {
	var resources []io.Closer
	defer func() {
		if err != nil {
			for _, r := range resources {
				if r != nil {
					_ = r.Close()
				}
			}
		}
	}()

	// 1. I/O Infrastructure (GCS)
	storage, err := gcs.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS factory: %w", err)
	}
	resources = append(resources, storage)

	store, err := storage.Store()
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// 2. Task Enqueuer
	enqueuer, err := buildTaskEnqueuer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task enqueuer: %w", err)
	}
	resources = append(resources, enqueuer)

	httpClient := httpkit.New(config.DefaultHTTPTimeout)
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL, cfg.ServiceURL)
	if err != nil {
		return nil, err
	}

	aiClient, err := adapters.NewGeminiAIAdapter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AI adapter: %w", err)
	}

	// 3. Prompt Generator
	promptGen, err := adapters.NewPromptAdapter()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize prompt adapter: %w", err)
	}

	// 4. Music Generator
	lyriaAdapter, err := adapters.NewLyriaAdapter(cfg, aiClient, promptGen)
	if err != nil {
		return nil, err
	}

	reader := adapters.NewReaderAdapter(storage, httpClient)

	publisher, err := adapters.NewPublisherAdapter(cfg, store)
	if err != nil {
		return nil, err
	}

	// 5. Pipeline (Core Logic)
	pipeline, err := buildPipeline(reader, lyriaAdapter, lyriaAdapter, publisher, slack)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize music pipeline: %w", err)
	}

	// 6. Repositories (Data Access)
	historyCache := repository.NewHistoryCache()
	go historyCache.Start()
	musicRepo := repository.NewGCSMusicRepository(cfg, store, historyCache)

	appCtx := &app.Container{
		Config:       cfg,
		Storage:      storage,
		Store:        store,
		TaskEnqueuer: enqueuer,
		Pipeline:     pipeline,
		HTTPClient:   httpClient,
		Notifier:     slack,
		MusicRepo:    musicRepo,
		HistoryCache: historyCache,
	}

	return appCtx, nil
}
