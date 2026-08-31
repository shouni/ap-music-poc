package pipeline

import (
	"context"
	"testing"

	"github.com/shouni/ap-music-poc/internal/domain"
)

type panicCollector struct{}

func (panicCollector) Collect(context.Context, domain.Task) (*domain.CollectedContent, error) {
	panic("collector should not be called")
}

type panicMusicGenerator struct{}

func (panicMusicGenerator) Run(context.Context, domain.Task, *domain.CollectedContent) (*domain.MusicRecipe, []byte, error) {
	panic("music generator should not be called")
}

type recordingAudioGenerator struct {
	recipe *domain.MusicRecipe
}

func (g *recordingAudioGenerator) GenerateAudio(_ context.Context, recipe *domain.MusicRecipe, _ []domain.ImagePayload) ([]byte, error) {
	g.recipe = recipe
	return []byte("mp3"), nil
}

type recordingPublisher struct {
	task   domain.Task
	recipe *domain.MusicRecipe
	audio  []byte
}

func (p *recordingPublisher) Publish(_ context.Context, task domain.Task, recipe *domain.MusicRecipe, audioData []byte) (*domain.PublishResult, error) {
	p.task = task
	p.recipe = recipe
	p.audio = audioData
	return &domain.PublishResult{JobID: task.JobID, StorageURI: "gs://bucket/job.mp3"}, nil
}

type recordingNotifier struct {
	notified      bool
	errorNotified bool
	req           domain.NotificationRequest
}

func (n *recordingNotifier) Notify(context.Context, *domain.PublishResult) error {
	n.notified = true
	return nil
}

func (n *recordingNotifier) NotifyWithRequest(_ context.Context, _ *domain.PublishResult, req domain.NotificationRequest) error {
	n.notified = true
	n.req = req
	return nil
}

func (n *recordingNotifier) NotifyError(_ context.Context, _ error, req domain.NotificationRequest) error {
	n.errorNotified = true
	n.req = req
	return nil
}

func TestMusicPipelineGenerateFromRecipeRunsPhase4And5Only(t *testing.T) {
	t.Parallel()

	audio := &recordingAudioGenerator{}
	publisher := &recordingPublisher{}
	notifier := &recordingNotifier{}
	p := MusicPipeline{
		Collector:      panicCollector{},
		MusicGenerator: panicMusicGenerator{},
		AudioGenerator: audio,
		Publisher:      publisher,
		Notifier:       notifier,
	}
	seed := int64(123)
	task := domain.Task{
		Command: domain.TaskCommandGenerateFromRecipe,
		JobID:   "job-1",
		Recipe: &domain.MusicRecipe{
			Title: "Recipe Song",
			AIModels: domain.AIModels{
				AudioModel: "recipe-model",
				Seed:       new(int64),
			},
		},
		AIModels: domain.AIModels{
			AudioModel: "task-model",
			Seed:       &seed,
		},
	}

	if err := p.Execute(context.Background(), task); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if audio.recipe == nil || audio.recipe.Title != "Recipe Song" {
		t.Fatalf("audio generator did not receive decoded recipe: %#v", audio.recipe)
	}
	if audio.recipe.AudioModel != "task-model" {
		t.Fatalf("expected task audio model override, got %q", audio.recipe.AudioModel)
	}
	if audio.recipe.Seed == nil || *audio.recipe.Seed != seed {
		t.Fatalf("expected task seed override, got %#v", audio.recipe.Seed)
	}
	if publisher.recipe != audio.recipe {
		t.Fatal("publisher did not receive generated recipe")
	}
	if string(publisher.audio) != "mp3" {
		t.Fatalf("expected mp3 payload, got %q", string(publisher.audio))
	}
	if !notifier.notified {
		t.Fatal("expected success notification")
	}
	if notifier.req.Command != string(domain.TaskCommandGenerateFromRecipe) {
		t.Fatalf("expected command in success notification, got %q", notifier.req.Command)
	}
	if notifier.req.Title != "Recipe Song" {
		t.Fatalf("expected title in success notification, got %q", notifier.req.Title)
	}
	if notifier.req.Seed == nil || *notifier.req.Seed != seed {
		t.Fatalf("expected task seed in success notification, got %#v", notifier.req.Seed)
	}
	if notifier.req.Mode != "" {
		t.Fatalf("expected empty mode for recipe generation, got %q", notifier.req.Mode)
	}
	if notifier.errorNotified {
		t.Fatal("did not expect error notification")
	}
}

func TestMusicPipelineComposeNotificationUsesComposeMode(t *testing.T) {
	t.Parallel()

	notifReq := domain.NotificationRequest{
		Command: string(domain.TaskCommandCompose),
		Mode: notificationMode(domain.Task{
			Command: domain.TaskCommandCompose,
			AIModels: domain.AIModels{
				ComposeMode: "rave",
			},
		}),
	}

	if notifReq.Mode != "rave" {
		t.Fatalf("expected compose mode, got %q", notifReq.Mode)
	}
}

func TestMusicPipelineGenerateFromRecipeNotificationUsesRecipeSeed(t *testing.T) {
	t.Parallel()

	audio := &recordingAudioGenerator{}
	notifier := &recordingNotifier{}
	p := MusicPipeline{
		Collector:      panicCollector{},
		MusicGenerator: panicMusicGenerator{},
		AudioGenerator: audio,
		Publisher:      &recordingPublisher{},
		Notifier:       notifier,
	}
	recipeSeed := int64(42)

	err := p.Execute(context.Background(), domain.Task{
		Command: domain.TaskCommandGenerateFromRecipe,
		JobID:   "job-1",
		Recipe: &domain.MusicRecipe{
			Title: "Recipe Seed Song",
			AIModels: domain.AIModels{
				Seed: &recipeSeed,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if audio.recipe == nil || audio.recipe.Seed == nil || *audio.recipe.Seed != recipeSeed {
		t.Fatalf("expected recipe seed to be used for generation, got %#v", audio.recipe)
	}
	if notifier.req.Seed == nil || *notifier.req.Seed != recipeSeed {
		t.Fatalf("expected recipe seed in success notification, got %#v", notifier.req.Seed)
	}
}

func TestMusicPipelineGenerateFromRecipeNotifiesOnError(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	p := MusicPipeline{
		Collector:      panicCollector{},
		MusicGenerator: panicMusicGenerator{},
		AudioGenerator: &recordingAudioGenerator{},
		Publisher:      &recordingPublisher{},
		Notifier:       notifier,
	}

	err := p.Execute(context.Background(), domain.Task{
		Command: domain.TaskCommandGenerateFromRecipe,
		JobID:   "job-1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !notifier.errorNotified {
		t.Fatal("expected error notification")
	}
	if notifier.req.Command != string(domain.TaskCommandGenerateFromRecipe) {
		t.Fatalf("expected command in error notification, got %q", notifier.req.Command)
	}
}
