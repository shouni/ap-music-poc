package domain

import "context"

// MusicRepository は、生成された楽曲の成果物（レシピや履歴）を管理するための定義です
// 永続化および取得操作を定義するインターフェースなのだ。
type MusicRepository interface {
	// ListHistory は、指定されたユーザーに関連付けられた楽曲生成履歴の一覧を取得します。
	ListHistory(ctx context.Context, userID string) ([]MusicHistory, error)
	// GetRecipe は、指定されたジョブIDに対応する詳細な楽曲設計図（MusicRecipe）を取得します。
	GetRecipe(ctx context.Context, jobID string) (*MusicRecipe, error)
	// DeleteHistory は、指定されたジョブIDの履歴を削除します。
	DeleteHistory(ctx context.Context, jobID string) error
}
