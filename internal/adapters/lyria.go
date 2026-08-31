package adapters

import (
	"context"
	"errors"

	"github.com/shouni/go-gemini-client/gemini"
	"github.com/shouni/go-gemini-client/lyria"

	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
)

// LyriaAdapter は domain と lyria の境界アダプターです。
//
// 楽曲設計図の型は domain 側が go-gemini-client の music パッケージを別名にしているため、
// レシピ・歌詞・AIModels の変換は要りません。残る変換は収集コンテンツだけです。
type LyriaAdapter struct {
	core *lyria.Workflow
}

// LyriaAdapterOption configures the Lyria adapter boundary.
type LyriaAdapterOption = lyria.Option

// NewLyriaAdapter は既存の adapters API を維持し、Lyria 実装を lyria パッケージへ委譲します。
func NewLyriaAdapter(cfg *config.Config, aiClient gemini.Generator, promptGen domain.PromptGenerator, adapterOptions ...LyriaAdapterOption) (*LyriaAdapter, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	options := []lyria.Option{
		lyria.WithGeminiModel(cfg.GeminiModel),
		lyria.WithLyriaModel(cfg.LyriaModel),
		lyria.WithRateInterval(cfg.RateInterval),
	}
	options = append(options, adapterOptions...)

	core, err := lyria.New(aiClient, promptGen, NewDefaultLyriaAudioPromptBuilder(), options...)
	if err != nil {
		return nil, err
	}

	return &LyriaAdapter{core: core}, nil
}

// Run は入力コンテンツから楽曲レシピと音声データを生成します。
//
// lyria.Workflow が一括実行の Run を持たなくなったため、作詞・作曲・音声生成の
// 3 段を当アダプターで順に呼びます。
func (a *LyriaAdapter) Run(ctx context.Context, task domain.Task, input *domain.CollectedContent) (*domain.MusicRecipe, []byte, error) {
	content := toLyriaCollectedContent(input)

	lyrics, err := a.core.GenerateLyrics(ctx, task.AIModels, content)
	if err != nil {
		return nil, nil, err
	}

	recipe, err := a.core.Compose(ctx, task.AIModels, lyrics)
	if err != nil {
		return nil, nil, err
	}

	var images []lyria.ImagePayload
	if content != nil {
		images = content.Images
	}

	audio, err := a.core.GenerateAudio(ctx, recipe, images)
	if err != nil {
		return nil, nil, err
	}

	return recipe, audio, nil
}

// GenerateLyrics は収集済みコンテンツから歌詞ドラフトを生成します。
func (a *LyriaAdapter) GenerateLyrics(ctx context.Context, ai domain.AIModels, input *domain.CollectedContent) (*domain.LyricsDraft, error) {
	return a.core.GenerateLyrics(ctx, ai, toLyriaCollectedContent(input))
}

// Compose は歌詞ドラフトから Lyria 向けの MusicRecipe を生成します。
func (a *LyriaAdapter) Compose(ctx context.Context, ai domain.AIModels, lyrics *domain.LyricsDraft) (*domain.MusicRecipe, error) {
	return a.core.Compose(ctx, ai, lyrics)
}

// GenerateAudio は MusicRecipe から音声データを生成します。
func (a *LyriaAdapter) GenerateAudio(ctx context.Context, recipe *domain.MusicRecipe, images []domain.ImagePayload) ([]byte, error) {
	return a.core.GenerateAudio(ctx, recipe, toLyriaImagePayloads(images))
}

func toLyriaCollectedContent(input *domain.CollectedContent) *lyria.CollectedContent {
	if input == nil {
		return nil
	}
	return &lyria.CollectedContent{
		Prompt: input.Prompt,
		Images: toLyriaImagePayloads(input.Images),
	}
}

func toLyriaImagePayloads(images []domain.ImagePayload) []lyria.ImagePayload {
	if images == nil {
		return nil
	}
	out := make([]lyria.ImagePayload, len(images))
	for i, image := range images {
		out[i] = lyria.ImagePayload{
			Data:     append([]byte(nil), image.Data...),
			MIMEType: image.MIMEType,
		}
	}
	return out
}
