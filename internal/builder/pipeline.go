package builder

import (
	"github.com/shouni/ap-music-poc/internal/domain"
	"github.com/shouni/ap-music-poc/internal/pipeline"
)

// buildPipeline は、提供された設定と各コンポーネントを使用して新しいパイプラインを初期化して返します。
func buildPipeline(
	collector domain.Collector,
	musicGenerator domain.MusicRunner,
	audioGenerator domain.AudioGenerator,
	publisher domain.Publisher,
	notifier domain.Notifier,
) (domain.Pipeline, error) {
	return pipeline.MusicPipeline{
		Collector:      collector,
		MusicGenerator: musicGenerator,
		AudioGenerator: audioGenerator,
		Publisher:      publisher,
		Notifier:       notifier,
	}, nil
}
