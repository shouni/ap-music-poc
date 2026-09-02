package builder

import (
	"fmt"
	"net/url"

	"github.com/shouni/gcp-kit/auth/oidc"
	"github.com/shouni/gcp-kit/auth/session"
	"github.com/shouni/gcp-kit/worker"

	"github.com/shouni/ap-music-poc/internal/app"
	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
	"github.com/shouni/ap-music-poc/internal/server/handlers"
)

const defaultSessionName = "ap-music-poc-session"

// AppHandlers は生成されたすべての HTTP ハンドラーを保持する構造体です。
// server パッケージはこの構造体を受け取ってルーティングを行います。
type AppHandlers struct {
	Auth   *session.Handler
	Web    *handlers.Handler
	Worker *worker.Handler[domain.Task]
	// TaskAuth は Cloud Tasks からの OIDC を検証します。セッション認証とは別物なので、
	// OAuth 設定を持たない Worker 側でも組み立てられます。
	TaskAuth *oidc.Verifier
}

// BuildHandlers は各ハンドラーの依存関係をすべて組み立て、AppHandlers 構造体を返します。
func BuildHandlers(
	appCtx *app.Container,
) (*AppHandlers, error) {
	if appCtx.Config.ServiceURL == "" {
		return nil, fmt.Errorf("認証リダイレクトのために ServiceURL の設定が必要です")
	}

	// 1. 認証Handlerの初期化
	authHandler, err := createAuthHandler(appCtx.Config)
	if err != nil {
		return nil, fmt.Errorf("認証Handlerの初期化に失敗しました: %w", err)
	}

	// 2. Web UI 用Handlerの初期化
	webHandler, err := handlers.NewHandler(appCtx.Config, appCtx.TaskEnqueuer, appCtx.Store, appCtx.MusicRepo)
	if err != nil {
		return nil, fmt.Errorf("WebHandlerの初期化に失敗しました: %w", err)
	}

	// 3. 非同期ワーカー用Handlerの初期化
	// audience と許可する呼び出し元 SA が揃わないと検証は必ず失敗する（fail-closed）ため、
	// 構成の不足は起動時に落とします。
	taskAuth := oidc.New(appCtx.Config.TaskAudienceURL, []string{appCtx.Config.ServiceAccountEmail})
	if !taskAuth.Configured() {
		return nil, fmt.Errorf("cloud Tasks の OIDC 検証を構成できません: TASK_AUDIENCE_URL と SERVICE_ACCOUNT_EMAIL が必要です")
	}
	workerHandler := worker.NewHandler[domain.Task](appCtx.Pipeline)

	return &AppHandlers{
		Auth:     authHandler,
		Web:      webHandler,
		Worker:   workerHandler,
		TaskAuth: taskAuth,
	}, nil
}

// createAuthHandler は、認証ハンドラーを初期化して返します。
func createAuthHandler(cfg *config.Config) (*session.Handler, error) {
	redirectURL, err := url.JoinPath(cfg.ServiceURL, "/auth/callback")
	if err != nil {
		return nil, fmt.Errorf("リダイレクトURLの構築に失敗しました: %w", err)
	}

	return session.New(session.Config{
		ClientID:       cfg.GoogleClientID,
		ClientSecret:   cfg.GoogleClientSecret,
		RedirectURL:    redirectURL,
		SessionName:    defaultSessionName,
		Store:          session.NewMemoryStore(session.StoreConfig{Secure: cfg.IsSecureServiceURL()}),
		IsSecureCookie: cfg.IsSecureServiceURL(),
		AllowedEmails:  cfg.AllowedEmails,
		AllowedDomains: cfg.AllowedDomains,
	})
}
