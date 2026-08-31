// Package handlers は、Web UI（フォーム表示・履歴閲覧等）のHTTPハンドラーを提供します。
package handlers

import (
	"context"

	"github.com/shouni/gcp-kit/auth/session"
)

// csrfTokenFromContext は、コンテキストに保存されたCSRFトークンを取得します。
//
// 格納するのは gcp-kit/auth/session の Authenticate なので、取得も同じキーを見ます。
func csrfTokenFromContext(ctx context.Context) string {
	return session.CSRFTokenFromContext(ctx)
}
