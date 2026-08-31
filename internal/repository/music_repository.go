// Package repository は、生成済み楽曲・履歴の永続化と一覧取得を行います。
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/shouni/go-remote-io/remoteio"
	"golang.org/x/sync/errgroup"

	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
)

// MusicRepository はGCS上の生成履歴、レシピ、音声成果物を管理します。
type MusicRepository struct {
	cfg          *config.Config
	store        remoteio.Store
	historyCache *ttlcache.Cache[string, domain.MusicHistory]
}

const defaultHistoryCacheTTL = 10 * time.Minute

// NewHistoryCache creates the app-scoped cache injected into MusicRepository.
func NewHistoryCache() *ttlcache.Cache[string, domain.MusicHistory] {
	return ttlcache.New[string, domain.MusicHistory](
		ttlcache.WithTTL[string, domain.MusicHistory](defaultHistoryCacheTTL),
		ttlcache.WithDisableTouchOnHit[string, domain.MusicHistory](),
	)
}

// NewGCSMusicRepository はGCS互換の Store を使う履歴リポジトリを構築します。
func NewGCSMusicRepository(cfg *config.Config, store remoteio.Store, historyCache *ttlcache.Cache[string, domain.MusicHistory]) *MusicRepository {
	if historyCache == nil {
		historyCache = NewHistoryCache()
	}

	return &MusicRepository{
		cfg:          cfg,
		store:        store,
		historyCache: historyCache,
	}
}

// ListHistory は並行処理を用いて履歴一覧を高速に取得します。
func (r *MusicRepository) ListHistory(ctx context.Context, _ string) ([]domain.MusicHistory, error) {
	gcsURI := r.cfg.GetGCSObjectURL("")
	if !strings.HasSuffix(gcsURI, "/") {
		gcsURI += "/"
	}

	// 1. まずファイル一覧（JobID）を取得する
	var jobIDs []string
	for entry, err := range r.store.List(ctx, gcsURI) {
		if err != nil {
			return nil, fmt.Errorf("GCS履歴のリスト取得に失敗したのだ: %w", err)
		}
		if entry.IsPrefix || !strings.HasSuffix(entry.URI, ".json") {
			continue
		}
		jobID := strings.TrimSuffix(path.Base(entry.URI), ".json")
		if jobID != "" {
			jobIDs = append(jobIDs, jobID)
		}
	}

	// 2. 並行して詳細（Recipe）を取得する
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(10)

	histories := make([]domain.MusicHistory, len(jobIDs))
	var mu sync.Mutex

	for i, id := range jobIDs {
		eg.Go(func() error {
			history, err := r.buildHistory(ctx, id)
			if err != nil {
				slog.WarnContext(ctx, "failed to load recipe metadata for history list",
					"jobID", id,
					"error", err,
				)
				// 取得失敗時はフォールバックデータを生成
				history = domain.MusicHistory{
					JobID:     id,
					Title:     id,
					CreatedAt: formatHistoryCreatedAt(id),
				}
			}

			mu.Lock()
			histories[i] = history
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// 3. 最後にソート（新しい順）
	sort.Slice(histories, func(i, j int) bool {
		return histories[i].JobID > histories[j].JobID
	})

	return histories, nil
}

func (r *MusicRepository) buildHistory(ctx context.Context, jobID string) (domain.MusicHistory, error) {
	if history, ok := r.getCachedHistory(jobID); ok {
		return history, nil
	}

	recipe, err := r.GetRecipe(ctx, jobID)
	if err != nil {
		return domain.MusicHistory{}, err
	}

	title := strings.TrimSpace(recipe.Title)
	if title == "" {
		title = jobID
	}

	history := domain.MusicHistory{
		JobID:       jobID,
		Title:       title,
		Mood:        strings.TrimSpace(recipe.Mood),
		Tempo:       recipe.Tempo,
		CreatedAt:   formatHistoryCreatedAt(jobID),
		ComposeMode: strings.TrimSpace(recipe.ComposeMode),
	}
	if recipe.Seed != nil {
		history.Seed = fmt.Sprintf("%d", *recipe.Seed)
	}

	r.setCachedHistory(jobID, history)

	return history, nil
}

func (r *MusicRepository) getCachedHistory(jobID string) (domain.MusicHistory, bool) {
	item := r.historyCache.Get(historyCacheKey(jobID))
	if item == nil {
		return domain.MusicHistory{}, false
	}

	return item.Value(), true
}

func (r *MusicRepository) setCachedHistory(jobID string, history domain.MusicHistory) {
	r.historyCache.Set(historyCacheKey(jobID), history, ttlcache.DefaultTTL)
}

func (r *MusicRepository) deleteCachedHistory(jobID string) {
	r.historyCache.Delete(historyCacheKey(jobID))
}

func historyCacheKey(jobID string) string {
	return path.Base(jobID)
}

// formatHistoryCreatedAt は、JobIDから日付を安全にパースします。
func formatHistoryCreatedAt(jobID string) string {
	const (
		jobIDTimePrefixLen = 14 // "20060102150405"
		jobIDTimeLayout    = "20060102150405"
		displayTimeLayout  = "2006-01-02 15:04 MST"
	)
	jst := time.FixedZone("JST", 9*60*60)

	if len(jobID) < jobIDTimePrefixLen {
		return ""
	}

	prefix := jobID[:jobIDTimePrefixLen]
	for _, char := range prefix {
		if char < '0' || char > '9' {
			return ""
		}
	}

	createdAt, err := time.ParseInLocation(jobIDTimeLayout, prefix, time.UTC)
	if err != nil {
		return ""
	}

	return createdAt.In(jst).Format(displayTimeLayout)
}

// GetRecipe は、特定の JSON ファイルを読み込んで構造体にパースします。
func (r *MusicRepository) GetRecipe(ctx context.Context, jobID string) (*domain.MusicRecipe, error) {
	safeJobID := path.Base(jobID)
	objectPath := fmt.Sprintf("%s.json", safeJobID)
	gcsURI := r.cfg.GetGCSObjectURL(objectPath)
	rc, err := r.store.Open(ctx, gcsURI)
	if err != nil {
		return nil, fmt.Errorf("JSONオープン失敗 (%s): %w", gcsURI, err)
	}
	defer rc.Close()

	var recipe domain.MusicRecipe
	if err := json.NewDecoder(rc).Decode(&recipe); err != nil {
		return nil, fmt.Errorf("JSONデコード失敗: %w", err)
	}

	return &recipe, nil
}

// DeleteHistory は、関連ファイルを削除します。
func (r *MusicRepository) DeleteHistory(ctx context.Context, jobID string) error {
	safeJobID := path.Base(jobID)
	var errs []error

	jsonPath := fmt.Sprintf("%s.json", safeJobID)
	jsonURI := r.cfg.GetGCSObjectURL(jsonPath)
	if err := r.store.Delete(ctx, jsonURI); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete recipe JSON (%s): %w", jsonURI, err))
	}

	audioPath := fmt.Sprintf("%s%s", safeJobID, domain.AudioFileExtension)
	audioURI := r.cfg.GetGCSObjectURL(audioPath)
	if err := r.store.Delete(ctx, audioURI); err != nil {
		slog.WarnContext(ctx, "skipped or failed to delete audio file",
			"jobID", safeJobID,
			"uri", audioURI,
			"error", err,
		)
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	r.deleteCachedHistory(safeJobID)
	return nil
}
