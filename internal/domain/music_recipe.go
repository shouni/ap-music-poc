package domain

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shouni/go-gemini-client/music"
)

// 楽曲設計図の型は go-gemini-client の葉パッケージ music が持ちます。
// 独自定義を持っていた頃は、JSON タグまで一致する同型を 2 つ抱え、その間を
// アダプターの手写しマッパで往復していました。フィールドが片方にだけ増えると
// 黙って落ちる形だったので、別名にして変換そのものを無くしています。
type (
	// LyricsDraft は作詞フェーズの出力です。
	LyricsDraft = music.LyricsDraft
	// MusicRecipe は楽曲設計図です。
	MusicRecipe = music.Recipe
	// MusicSection は曲内セクションです。
	MusicSection = music.Section
	// AIModels は各生成フェーズで使うモデルと seed を保持します。
	AIModels = music.AIModels
)

// DecodeMusicRecipeJSON parses user-submitted JSON into a MusicRecipe.
func DecodeMusicRecipeJSON(raw string) (*MusicRecipe, error) {
	recipeText := strings.TrimSpace(raw)
	if recipeText == "" {
		return nil, fmt.Errorf("music recipe json is required")
	}

	var recipe MusicRecipe
	if err := json.Unmarshal([]byte(recipeText), &recipe); err != nil {
		return nil, fmt.Errorf("invalid music recipe json: %w", err)
	}
	if err := ValidateRecipeForGeneration(&recipe); err != nil {
		return nil, err
	}

	return &recipe, nil
}

// ValidateRecipeForGeneration checks that the recipe has enough musical direction to
// send to the audio generator.
//
// メソッドではなく自由関数なのは、MusicRecipe が music.Recipe の別名になり、
// 別パッケージの型にメソッドを足せなくなったためです。
func ValidateRecipeForGeneration(r *MusicRecipe) error {
	if strings.TrimSpace(r.Title) == "" &&
		strings.TrimSpace(r.Theme) == "" &&
		len(r.Instruments) == 0 &&
		len(r.Sections) == 0 &&
		r.Lyrics == nil {
		return fmt.Errorf("music recipe must include title, theme, instruments, sections, or lyrics")
	}

	return nil
}

// MusicHistory は一覧画面の表示
type MusicHistory struct {
	JobID       string `json:"job_id"`
	Title       string `json:"title"`
	Mood        string `json:"mood,omitempty"`
	Tempo       int    `json:"tempo,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	ComposeMode string `json:"compose_mode,omitempty"`
	Seed        string `json:"seed,omitempty"`
}
