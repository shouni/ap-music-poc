// Package server は、HTTPルーティングとミドルウェアを構成します。
package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shouni/gcp-kit/auth"

	"github.com/shouni/ap-music-poc/internal/builder"
)

// NewRouter は、ミドルウェアとルーティングを統合した http.Handler を構築します。
func NewRouter(h *builder.AppHandlers) http.Handler {
	r := chi.NewRouter()
	setupCommonMiddleware(r)
	setupRoutes(r, h)

	return r
}

// setupCommonMiddleware は、標準的なミドルウェアを構成します。
func setupCommonMiddleware(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
}

// setupRoutes は、各コンポーネントのハンドラーをルーティングに登録します。
func setupRoutes(r chi.Router, h *builder.AppHandlers) {
	// --- 1. 公開ルート (ヘルスチェック) ---
	// "/healthz" は Cloud Run のデフォルトドメイン (*.run.app) 側で予約パス的に扱われ、
	// コンテナまでリクエストが届かず GFE の汎用 404 に置き換えられるため使わない。
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if h == nil {
		slog.Warn("AppHandlers is nil, skipping application routes registration")
		return
	}

	// --- 2. 認証関連エンドポイント (OAuth2 フロー) ---
	if h.Auth != nil {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", h.Auth.Login)
			r.Get("/callback", h.Auth.Callback)
		})
	}

	// --- 3. 認証が必要なルート (Web UI 用) ---
	r.Group(func(r chi.Router) {
		if h.Auth == nil {
			if h.Web != nil {
				slog.Error("Auth handler is nil, skipping protected web routes")
			}
			return
		}

		// ログインチェック・CSRF 検証・CSRF トークンの発行はすべて
		// session.Handler.Authenticate が行います。
		r.Use(auth.Require(h.Auth))

		if h.Web != nil {
			r.Get("/", h.Web.Home)
			r.Post("/web/compose", h.Web.EnqueueTask)
			r.Get("/web/generate-from-recipe", h.Web.GenerateFromRecipe)
			r.Post("/web/generate-from-recipe", h.Web.EnqueueGenerateFromRecipe)
			r.Route("/web/history", func(r chi.Router) {
				r.Get("/", h.Web.ServeHistory)
				r.Get("/{jobID}", h.Web.ServeDetails)
				r.Delete("/{jobID}", h.Web.DeleteHistory)
			})
			r.Get("/web/audio/{jobID}", h.Web.ServeAudio)
		}
	})

	// --- 4. Cloud Tasks 専用ルート (Worker 用) ---
	r.Group(func(r chi.Router) {
		if h.TaskAuth == nil {
			if h.Worker != nil {
				slog.Error("Task OIDC verifier is nil, skipping worker routes")
			}
			return
		}

		// Cloud Tasks からの OIDC トークンを検証 (セッション不要)
		r.Use(auth.Require(h.TaskAuth))

		if h.Worker != nil {
			r.Post("/tasks/generate", h.Worker.ProcessTask)
		}
	})
}
