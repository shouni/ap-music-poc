package domain

import "context"

// Pipeline は、デコードされたペイロードを受け取って実際の処理を行うインターフェースです。
type Pipeline interface {
	Execute(ctx context.Context, payload Task) (err error)
}

// Collector は入力コンテキスト収集を行います。
type Collector interface {
	Collect(ctx context.Context, task Task) (*CollectedContent, error)
}

// MusicRunner は音楽生成のコアプロセス（作詞〜音声生成）を一括で行うインターフェースです。
type MusicRunner interface {
	// Run はコンテキストを受け取り、最終的な音声バイナリとレシピ（メタデータ）を返します。
	Run(ctx context.Context, task Task, input *CollectedContent) (*MusicRecipe, []byte, error)
}

// Lyricist は歌詞生成を担う役割です。
type Lyricist interface {
	GenerateLyrics(ctx context.Context, input *CollectedContent, model, mode string) (*LyricsDraft, error)
}

// Composer は楽曲の設計（レシピ構築）を担う役割です。
type Composer interface {
	Compose(ctx context.Context, lyrics *LyricsDraft, model, mode string) (*MusicRecipe, error)
}

// AudioGenerator は MusicRecipe から音声バイナリを生成します。
type AudioGenerator interface {
	GenerateAudio(ctx context.Context, recipe *MusicRecipe, images []ImagePayload) ([]byte, error)
}
