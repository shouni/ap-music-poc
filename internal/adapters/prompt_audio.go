package adapters

import (
	"fmt"
	"strings"

	"github.com/shouni/go-gemini-client/lyria"
)

type lyriaAudioPromptBuilder struct{}

// NewDefaultLyriaAudioPromptBuilder returns the public showcase audio prompt builder.
func NewDefaultLyriaAudioPromptBuilder() lyria.AudioPromptBuilder {
	return lyriaAudioPromptBuilder{}
}

// BuildFullSong は、MusicRecipe 全体を 1 回の Lyria 呼び出しで生成するためのプロンプトを組み立てます。
func (lyriaAudioPromptBuilder) BuildFullSong(recipe *lyria.MusicRecipe) string {
	var pb strings.Builder
	pb.WriteString("Task: Generate a full song from the provided music recipe.\n")
	pb.WriteString(buildLyriaSongContext(recipe))

	if len(recipe.Sections) > 0 {
		pb.WriteString("Song Structure:\n")
		for _, sec := range recipe.Sections {
			direction := buildLyriaSectionDirection(sec.Name)
			fmt.Fprintf(&pb, "- [%s] (%d sec): %s %s\n", sec.Name, sec.Duration, direction, sec.Prompt)
		}
		pb.WriteString("\n")
	}

	pb.WriteString("[Generation Guidelines]\n")
	pb.WriteString("- Follow the provided title, mood, tempo, instruments, lyrics, and section structure.\n")
	pb.WriteString("- Keep transitions natural between sections.\n")
	pb.WriteString("- Preserve the intended musical direction from each section prompt.\n")
	pb.WriteString("- Avoid unintended long pauses, abrupt endings, or silent gaps between sections.\n")
	pb.WriteString("- Ensure clear vocal performance and proper enunciation throughout the track.")

	return pb.String()
}

// buildLyriaSongContext は、全体生成とセクション生成で共有する曲全体の文脈を組み立てます。
func buildLyriaSongContext(recipe *lyria.MusicRecipe) string {
	var pb strings.Builder
	fmt.Fprintf(&pb, "Title: '%s'.\n", recipe.Title)
	fmt.Fprintf(&pb, "Style & Mood: %s\n", recipe.Mood)
	fmt.Fprintf(&pb, "Tempo: %d BPM. Instruments: %s.\n\n", recipe.Tempo, strings.Join(recipe.Instruments, ", "))

	if recipe.Lyrics != nil && recipe.Lyrics.Lyrics != "" {
		pb.WriteString("Lyrics:\n")
		pb.WriteString(recipe.Lyrics.Lyrics)
		pb.WriteString("\n\n")
	}

	return pb.String()
}

// buildLyriaSectionDirection は、セクション名に応じた汎用的な生成方針を返します。
func buildLyriaSectionDirection(sectionName string) string {
	return fmt.Sprintf("Section Direction: Use this section as the [%s] part of the full arrangement.", sectionName)
}
