package domain

import "context"

// Publisher は生成物の保存先を抽象化します。
type Publisher interface {
	Publish(ctx context.Context, task Task, recipe *MusicRecipe, audioData []byte) (*PublishResult, error)
}

// Notifier は完了通知を抽象化します。
type Notifier interface {
	Notify(ctx context.Context, result *PublishResult) error
	NotifyWithRequest(ctx context.Context, result *PublishResult, req NotificationRequest) error
	NotifyError(ctx context.Context, errDetail error, req NotificationRequest) error
}

// TaskQueue は非同期キューを抽象化します。
type TaskQueue interface {
	Enqueue(ctx context.Context, task Task) error
}

// PromptGenerator は、AIプロンプトを生成するインターフェースです。
type PromptGenerator interface {
	GenerateLyrics(mode string, content string) (string, error)
	GenerateRecipe(mode string, lyrics *LyricsDraft) (string, error)
}
