package builder

import (
	"context"

	"github.com/shouni/gcp-kit/tasks"

	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
)

// buildTaskEnqueuer は、Cloud Tasks エンキューアを初期化します。
func buildTaskEnqueuer(ctx context.Context, cfg *config.Config) (*tasks.Enqueuer[domain.Task], error) {

	taskCfg := tasks.Config{
		ProjectID:           cfg.ProjectID,
		LocationID:          cfg.LocationID,
		QueueID:             cfg.QueueID,
		WorkerURL:           cfg.ServiceURL,
		WorkerPath:          "/tasks/generate",
		ServiceAccountEmail: cfg.ServiceAccountEmail,
		Audience:            cfg.TaskAudienceURL,
	}
	return tasks.NewEnqueuer[domain.Task](ctx, taskCfg)
}
