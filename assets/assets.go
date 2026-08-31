// Package assets は、プロンプトテンプレートを埋め込みリソースとして提供します。
package assets

import (
	"embed"

	"github.com/shouni/go-prompt-kit/resource"
)

const (
	promptDir = "prompts"

	lyricsPrefix  = "lyrics_"
	composePrefix = "compose_"

	// ModeLyrics represents the default template key for lyrics generation.
	ModeLyrics = "default"
	// ModeCompose represents the default template key for music composition.
	ModeCompose = "default"
)

var (
	// lyricsFiles はプロンプトテンプレートです。
	//go:embed prompts/lyrics_*.md
	lyricsFiles embed.FS

	// composeFiles はプロンプトテンプレートです。
	//go:embed prompts/compose_*.md
	composeFiles embed.FS

	// Templates は、すべてのHTMLテンプレートを保持します。
	//go:embed templates/*.html
	Templates embed.FS
)

// LoadLyricsFiles は埋め込まれたプロンプトファイルを読み込みます。
func LoadLyricsFiles() (map[string]string, error) {
	return resource.Load(lyricsFiles, promptDir, resource.WithPrefix(lyricsPrefix))
}

// LoadComposeFiles は埋め込まれたプロンプトファイルを読み込みます。
func LoadComposeFiles() (map[string]string, error) {
	return resource.Load(composeFiles, promptDir, resource.WithPrefix(composePrefix))
}
