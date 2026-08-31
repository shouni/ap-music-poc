package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-notify/notify"
	"github.com/shouni/go-notify/slack"

	"github.com/shouni/ap-music-poc/internal/domain"
)

const (
	slackSuccessTitle = "🎼 処理が完了しました！"
	slackErrorTitle   = "❌ 処理中にエラーが発生しました"
	slackErrorLabel   = "エラー内容"
)

// SlackAdapter は、Slack APIと連携し、Webhookを介してメッセージを投稿するためのアダプタを表します。
type SlackAdapter struct {
	notifier   notify.Notifier
	serviceURL string
}

// NewSlackAdapter は新しいアダプターインスタンスを作成します。
// webhookURL が空の場合、go-notify 側が無効な Notifier を返すため送信は行われません。
func NewSlackAdapter(httpClient httpkit.Requester, webhookURL, serviceURL string) (*SlackAdapter, error) {
	notifier, err := slack.NewNotifier(httpClient, webhookURL)
	if err != nil {
		return nil, fmt.Errorf("slackクライアントの初期化に失敗しました: %w", err)
	}

	return &SlackAdapter{
		notifier:   notifier,
		serviceURL: serviceURL,
	}, nil
}

// Notify は処理完了時の標準的なSlack通知を送信します。
func (s *SlackAdapter) Notify(ctx context.Context, result *domain.PublishResult) error {
	return s.NotifyWithRequest(ctx, result, domain.NotificationRequest{})
}

// NotifyWithRequest は詳細情報（NotificationRequest）付きでSlack通知を送信します。
func (s *SlackAdapter) NotifyWithRequest(ctx context.Context, result *domain.PublishResult, req domain.NotificationRequest) error {
	if result == nil {
		return fmt.Errorf("publish result is nil")
	}

	if !notify.Enabled(s.notifier) {
		slog.InfoContext(ctx, "Slack通知が無効化されているためスキップします。", "storage_uri", result.StorageURI)
		return nil
	}

	msg := notify.Message{
		Title: slackSuccessTitle,
		Body:  s.buildSlackContent(result, req),
		Level: notify.LevelSuccess,
	}

	if err := s.notifier.Notify(ctx, msg); err != nil {
		return fmt.Errorf("slackへの投稿に失敗しました: %w", err)
	}

	slog.Info("Slack に完了通知を送信しました。",
		"public_url", result.SignedURL,
		"recipe_url", result.RecipeSignedURL,
		"job_id", result.JobID,
	)
	return nil
}

// NotifyError エラー詳細と実行メタデータを含むSlackエラー通知の送信。
func (s *SlackAdapter) NotifyError(ctx context.Context, errDetail error, req domain.NotificationRequest) error {
	if !notify.Enabled(s.notifier) {
		slog.InfoContext(ctx, "Slack通知が無効化されているため、エラー通知をスキップします。", "error", errDetail)
		return nil
	}

	body := notify.NewBody()
	writeSlackRequestMetadata(body, req)
	body.Error(slackErrorLabel, errDetail)

	msg := notify.Message{
		Title: slackErrorTitle,
		Body:  body.String(),
		Level: notify.LevelFailure,
	}

	if err := s.notifier.Notify(ctx, msg); err != nil {
		return fmt.Errorf("slackへのエラー通知に失敗しました: %w", err)
	}

	slog.Info("Slack にエラー通知を送信しました。", "error", errDetail)
	return nil
}

// buildSlackContent 指定された結果とリクエストに基づき、Slack メッセージの内容を生成します。
func (s *SlackAdapter) buildSlackContent(result *domain.PublishResult, req domain.NotificationRequest) string {
	body := notify.NewBody()

	writeSlackRequestMetadata(body, req)

	body.LinkOrField("History Detail", s.historyDetailURL(result.JobID), result.JobID)

	// 音楽ファイルのリンク（署名付き URL が無ければ URI をそのまま載せます）
	body.LinkOrField("WAV File", result.SignedURL, result.StorageURI)

	// レシピ JSON のリンク
	body.LinkOrField("Recipe JSON", result.RecipeSignedURL, result.RecipeStorageURI)

	if body.Empty() {
		body.Text(domain.NotAvailable)
	}

	return body.String()
}

// writeSlackRequestMetadata は通知本文の先頭へ、依頼内容のメタデータを書き出します。
//
// 記法は notify.Body が出力する標準 Markdown です。Slack mrkdwn を直接書くと
// Slack 以外のチャネルへ送れなくなるため、組み立ては Body に任せます。
func writeSlackRequestMetadata(body *notify.Body, req domain.NotificationRequest) {
	body.Code("Command", req.Command)
	body.Field("Title", req.Title)
	body.Field("Source", req.SourceURL)
	body.Code("Mode", req.Mode)

	if req.Seed != nil {
		body.Field("Seed", notify.CodeSpan(strconv.FormatInt(*req.Seed, 10))+" 🎲")
	}
}

func (s *SlackAdapter) historyDetailURL(jobID string) string {
	if s.serviceURL == "" || jobID == "" {
		return ""
	}

	return notify.JoinURL(s.serviceURL, "web", "history", jobID)
}
