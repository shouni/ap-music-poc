// Package app は、アプリケーションの依存関係を組み立てて保持する DI コンテナを提供します。
package app

import (
	"log/slog"

	"github.com/jellydator/ttlcache/v3"
	"github.com/shouni/gcp-kit/tasks"
	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio"

	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
)

// Container はアプリケーションの依存関係（DIコンテナ）を保持します。
type Container struct {
	Config *config.Config
	// I/O and Storage
	// Storage は GCS クライアントの寿命を持ちます。go-web-reader のように
	// ファクトリそのものを要求する相手へ渡すために保持しています。
	Storage remoteio.Factory
	// Store は Storage から取り出した読み書き・署名の窓口です。
	Store remoteio.Store
	// Asynchronous Task
	TaskEnqueuer *tasks.Enqueuer[domain.Task]
	// Business Logic
	Pipeline domain.Pipeline
	// External Adapters
	HTTPClient httpkit.Requester
	Notifier   domain.Notifier
	// Data Access
	MusicRepo    domain.MusicRepository
	HistoryCache *ttlcache.Cache[string, domain.MusicHistory]
}

// Close は、Container が保持するすべての外部接続リソースを安全に解放します。
func (c *Container) Close() {
	// ストレージクライアントの解放
	if c.Storage != nil {
		if err := c.Storage.Close(); err != nil {
			slog.Error("failed to close storage factory", "error", err)
		}
	}

	// TaskEnqueuer のリソース解放
	if c.TaskEnqueuer != nil {
		if err := c.TaskEnqueuer.Close(); err != nil {
			slog.Error("failed to close task enqueuer", "error", err)
		}
	}

	if c.HistoryCache != nil {
		c.HistoryCache.Stop()
	}
}
