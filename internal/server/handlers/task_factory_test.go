package handlers

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/shouni/ap-music-poc/internal/domain"
)

func TestTaskFactoryBuildUsesProvidedValues(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	allowed := []string{"rave", "jazz", "techno"}

	factory := &taskFactory{
		now:          func() time.Time { return now },
		newSeed:      func() int64 { return 99 },
		newJobID:     func(time.Time) string { return "generated-job" },
		allowedModes: allowed,
	}

	task := factory.BuildCompose(url.Values{
		"job_id":        {"job-123"},
		"url":           {" https://example.com "},
		"text":          {" hello "},
		"image":         {" https://example.com/image.png "},
		"lyrics_model":  {" gemini-text "},
		"compose_model": {" lyria-audio "},
		"compose_mode":  {" rave "}, // 許可リストにある値を指定
		"seed":          {"42"},
	})

	expected := domain.Task{
		Command:    domain.TaskCommandCompose,
		JobID:      "job-123",
		RequestURL: "https://example.com",
		InputText:  "hello",
		ImageURL:   "https://example.com/image.png",
		CreatedAt:  now,
		AIModels: domain.AIModels{
			TextModel:   "gemini-text",
			AudioModel:  "lyria-audio",
			ComposeMode: "rave",
			Seed:        new(int64(42)),
		},
	}

	assert.Equal(t, expected, task)
}

func TestTaskFactoryBuildWithInvalidMode(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	// 許可リストに "rave" はない状態
	factory := &taskFactory{
		now:          func() time.Time { return now },
		newSeed:      func() int64 { return 99 },
		newJobID:     func(time.Time) string { return "generated-job" },
		allowedModes: []string{"jazz"},
	}

	task := factory.BuildCompose(url.Values{
		"compose_mode": {"rave"},
	})

	assert.Equal(t, "", task.ComposeMode)
}

func TestTaskFactoryBuildGeneratesFallbacks(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	factory := &taskFactory{
		now:      func() time.Time { return now },
		newSeed:  func() int64 { return 1234 },
		newJobID: func(time.Time) string { return "generated-job" },
		// allowedModes を指定しない場合、全ての入力は空文字として扱われる
	}

	task := factory.BuildCompose(url.Values{
		"seed": {"not-a-number"},
	})

	if assert.NotNil(t, task.Seed) {
		assert.Equal(t, int64(1234), *task.Seed)
	}
	assert.Equal(t, "generated-job", task.JobID)
	assert.Equal(t, now, task.CreatedAt)
}

func TestTaskFactoryBuildGenerateFromRecipe(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	factory := &taskFactory{
		now:      func() time.Time { return now },
		newSeed:  func() int64 { return 99 },
		newJobID: func(time.Time) string { return "generated-job" },
	}

	task, err := factory.BuildGenerateFromRecipe(url.Values{
		"recipe_json":   {` {"title":"x"} `},
		"compose_model": {" lyria-audio "},
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.TaskCommandGenerateFromRecipe, task.Command)
	assert.Equal(t, "generated-job", task.JobID)
	if assert.NotNil(t, task.Recipe) {
		assert.Equal(t, "x", task.Recipe.Title)
	}
	assert.Equal(t, "lyria-audio", task.AudioModel)
	if assert.NotNil(t, task.Seed) {
		assert.Equal(t, int64(99), *task.Seed)
	}
}

func TestTaskFactoryBuildGenerateFromRecipeKeepsRecipeSeedWhenFormSeedIsEmpty(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	factory := &taskFactory{
		now:      func() time.Time { return now },
		newSeed:  func() int64 { return 99 },
		newJobID: func(time.Time) string { return "generated-job" },
	}

	task, err := factory.BuildGenerateFromRecipe(url.Values{
		"recipe_json": {`{"title":"x","seed":42}`},
	})

	assert.NoError(t, err)
	assert.Nil(t, task.Seed)
	if assert.NotNil(t, task.Recipe) && assert.NotNil(t, task.Recipe.Seed) {
		assert.Equal(t, int64(42), *task.Recipe.Seed)
	}
}

func TestTaskFactoryBuildGenerateFromRecipeRejectsInvalidRecipeJSON(t *testing.T) {
	factory := &taskFactory{
		now:      func() time.Time { return time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC) },
		newSeed:  func() int64 { return 99 },
		newJobID: func(time.Time) string { return "generated-job" },
	}

	_, err := factory.BuildGenerateFromRecipe(url.Values{
		"recipe_json": {"{not-json"},
	})

	assert.Error(t, err)
}

func TestTaskFactoryBuildGenerateFromRecipeRejectsInvalidSeed(t *testing.T) {
	factory := &taskFactory{
		now:      func() time.Time { return time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC) },
		newSeed:  func() int64 { return 99 },
		newJobID: func(time.Time) string { return "generated-job" },
	}

	_, err := factory.BuildGenerateFromRecipe(url.Values{
		"recipe_json": {`{"title":"x"}`},
		"seed":        {"abc"},
	})

	assert.Error(t, err)
}
