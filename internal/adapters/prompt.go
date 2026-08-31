package adapters

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shouni/go-prompt-kit/prompts"

	"github.com/shouni/ap-music-poc/assets"
	"github.com/shouni/ap-music-poc/internal/domain"
)

// lyricsPromptData は歌詞プロンプトのテンプレートに渡すデータ構造です。
type lyricsPromptData struct {
	InputText    string
	OutputSchema string
}

// recipePromptData はレシピプロンプトのテンプレートに渡すデータ構造です。
type recipePromptData struct {
	LyricsContent string
	OutputSchema  string
}

// promptBuilder は、フォーマット済みのプロンプトを作成するためのインターフェース
type promptBuilder interface {
	Build(mode string, data any) (string, error)
}

// PromptAdapter は、さまざまなモードとデータに基づいてプロンプトを生成する役割を担います。
type PromptAdapter struct {
	lyrics  promptBuilder
	compose promptBuilder
}

// NewPromptAdapter は動的に読み込んだテンプレートを使用して Builder を構築します。
func NewPromptAdapter() (*PromptAdapter, error) {
	// 1. テンプレートの読み込み
	lyricsTemplates, err := assets.LoadLyricsFiles()
	if err != nil {
		return nil, fmt.Errorf("歌詞テンプレートの読み込みに失敗: %w", err)
	}

	// 2. ビルダーの構築
	lyrics, err := prompts.NewBuilder(lyricsTemplates)
	if err != nil {
		return nil, fmt.Errorf("歌詞ビルダーの構築に失敗: %w", err)
	}

	// 3. テンプレートの読み込み
	composeTemplates, err := assets.LoadComposeFiles()
	if err != nil {
		return nil, fmt.Errorf("作曲テンプレートの読み込みに失敗: %w", err)
	}

	// 4. ビルダーの構築
	compose, err := prompts.NewBuilder(composeTemplates)
	if err != nil {
		return nil, fmt.Errorf("作曲ビルダーの構築に失敗: %w", err)
	}

	return &PromptAdapter{
		lyrics:  lyrics,
		compose: compose,
	}, nil
}

// GenerateLyrics は歌詞生成用プロンプトを返します。
func (pa *PromptAdapter) GenerateLyrics(mode string, content string) (string, error) {
	draft := domain.LyricsDraft{
		Title:     "楽曲のタイトル",
		Theme:     "世界観の核",
		Hook:      "印象的なフレーズ",
		Lyrics:    "[Verse]\n...\n[Chorus]\n...",
		Keywords:  []string{"キーワード1", "キーワード2"},
		Mood:      "雰囲気",
		Narrative: "背景物語",
	}

	schemaBytes, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return "", fmt.Errorf("歌詞出力スキーマの生成に失敗: %w", err)
	}
	outputSchema := string(schemaBytes)
	enrichedContent := content
	// lyrics_modelやcompose_modeの指定に応じて、プロンプトにモード特有の制約を付与
	// assets.ModeLyrics ("lyrics") は標準モードのため、それ以外が指定された場合のみ処理を行う
	if mode != "" && mode != assets.ModeLyrics {
		// 特殊モード（jazz, techno-futurism等）の追加プロンプトを構築...
		enrichedContent = fmt.Sprintf("【楽曲ジャンル: %s 】\n%s", mode, content)
	}

	data := lyricsPromptData{
		InputText:    enrichedContent,
		OutputSchema: outputSchema,
	}

	prompt, err := pa.lyrics.Build(assets.ModeLyrics, data)
	if err != nil {
		return "", fmt.Errorf("歌詞テンプレートの実行に失敗: %w", err)
	}
	return prompt, nil
}

// GenerateRecipe はレシピ生成用プロンプトを返します。
func (pa *PromptAdapter) GenerateRecipe(mode string, lyrics *domain.LyricsDraft) (string, error) {
	recipeTemplate := domain.MusicRecipe{
		Title:       "楽曲のタイトル",
		Theme:       "楽曲のコンセプト",
		Mood:        "Euphoric High-Energy (英語)",
		Tempo:       160,
		Instruments: []string{"Synthesizer", "Drum Machine"},
		Sections: []domain.MusicSection{
			{
				Name:     "Main",
				Duration: 30,
				Prompt:   "Lyria 3用の詳細な英文プロンプト...",
			},
		},
	}

	lyricsContent := buildLyricsContent(lyrics)
	schemaBytes, err := json.MarshalIndent(recipeTemplate, "", "  ")
	if err != nil {
		return "", fmt.Errorf("レシピ出力スキーマの生成に失敗: %w", err)
	}
	data := recipePromptData{
		LyricsContent: lyricsContent,
		OutputSchema:  string(schemaBytes),
	}

	prompt, err := pa.compose.Build(mode, data)
	if err != nil {
		return "", fmt.Errorf("レシピテンプレートの実行に失敗: %w", err)
	}
	return prompt, nil
}

// buildLyricsContent は、プロンプトに埋め込むための「歌詞案」セクションを構築します。
func buildLyricsContent(ld *domain.LyricsDraft) string {
	if ld == nil {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Title: %s\n", ld.Title)
	fmt.Fprintf(&sb, "Theme: %s\n", ld.Theme)
	fmt.Fprintf(&sb, "Hook: %s\n", ld.Hook)
	fmt.Fprintf(&sb, "Mood: %s\n", ld.Mood)
	fmt.Fprintf(&sb, "Narrative: %s\n", ld.Narrative)

	if len(ld.Keywords) > 0 {
		sb.WriteString("Keywords: ")
		sb.WriteString(strings.Join(ld.Keywords, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("Lyrics:\n")
	sb.WriteString(ld.Lyrics)

	return sb.String()
}
