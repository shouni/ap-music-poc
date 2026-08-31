package handlers

import (
	"fmt"
	"math"
	"math/rand/v2"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shouni/ap-music-poc/internal/domain"
)

type taskFactory struct {
	now          func() time.Time
	newSeed      func() int64
	newJobID     func(time.Time) string
	allowedModes []string
}

// newTaskFactory は許可されたモードリストを受け取って初期化するように変更
func newTaskFactory(allowedModes []string) *taskFactory {
	return &taskFactory{
		now: func() time.Time { return time.Now().UTC() },
		newSeed: func() int64 {
			return rand.Int64N(math.MaxInt32 + 1)
		},
		newJobID: func(now time.Time) string {
			return fmt.Sprintf("%s-%s", now.Format("20060102150405"), uuid.New().String()[:8])
		},
		allowedModes: allowedModes,
	}
}

// BuildCompose はフォームデータから通常の作曲タスクを構築します。
func (f *taskFactory) BuildCompose(form url.Values) domain.Task {
	createdAt := f.now()

	rawMode := strings.TrimSpace(form.Get("compose_mode"))
	validatedMode := ""
	if slices.Contains(f.allowedModes, rawMode) {
		validatedMode = rawMode
	}

	task := domain.Task{
		Command:    domain.TaskCommandCompose,
		JobID:      strings.TrimSpace(form.Get("job_id")),
		RequestURL: strings.TrimSpace(form.Get("url")),
		InputText:  strings.TrimSpace(form.Get("text")),
		ImageURL:   strings.TrimSpace(form.Get("image")),
		CreatedAt:  createdAt,
		AIModels: domain.AIModels{
			TextModel:   strings.TrimSpace(form.Get("lyrics_model")),
			AudioModel:  strings.TrimSpace(form.Get("compose_model")),
			ComposeMode: validatedMode, // バリデーション済みの値をセット
			Seed:        parseSeed(form.Get("seed"), f.newSeed),
		},
	}

	f.ensureJobID(&task, createdAt)

	return task
}

// BuildGenerateFromRecipe はMusicRecipe JSONフォームからPhase 4/5専用タスクを構築します。
func (f *taskFactory) BuildGenerateFromRecipe(form url.Values) (domain.Task, error) {
	createdAt := f.now()
	recipe, err := domain.DecodeMusicRecipeJSON(form.Get("recipe_json"))
	if err != nil {
		return domain.Task{}, err
	}

	seed, err := parseOptionalSeed(form.Get("seed"))
	if err != nil {
		return domain.Task{}, err
	}
	if seed == nil && recipe.Seed == nil {
		seed = new(f.newSeed())
	}

	task := domain.Task{
		Command:   domain.TaskCommandGenerateFromRecipe,
		JobID:     strings.TrimSpace(form.Get("job_id")),
		Recipe:    recipe,
		CreatedAt: createdAt,
		AIModels: domain.AIModels{
			AudioModel: strings.TrimSpace(form.Get("compose_model")),
			Seed:       seed,
		},
	}

	f.ensureJobID(&task, createdAt)

	return task, nil
}

func (f *taskFactory) ensureJobID(task *domain.Task, createdAt time.Time) {
	if task.JobID == "" {
		task.JobID = f.newJobID(createdAt)
	}
}

func parseOptionalSeed(raw string) (*int64, error) {
	seedText := strings.TrimSpace(raw)
	if seedText == "" {
		return nil, nil
	}
	if val, err := strconv.ParseInt(seedText, 10, 64); err == nil {
		val &= math.MaxInt32
		return &val, nil
	}
	return nil, fmt.Errorf("invalid seed: %s", seedText)
}

// parseSeed 指定された生のシード文字列を解析してint64ポインタに変換します。解析に失敗した場合は、フォールバック関数を使用します。
// 解析された値がint32の最大値を超える場合は、int32の制限内に収まるように値をラップします。
func parseSeed(raw string, fallback func() int64) *int64 {
	seedText := strings.TrimSpace(raw)
	if seedText != "" {
		if val, err := strconv.ParseInt(seedText, 10, 64); err == nil {
			val &= math.MaxInt32
			return &val
		}
	}

	return new(fallback())
}
