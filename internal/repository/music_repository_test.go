package repository

import (
	"context"
	"io"
	"slices"
	"sync"
	"testing"

	"github.com/shouni/go-remote-io/remoteio"
	"github.com/shouni/go-remote-io/remoteio/memio"

	"github.com/shouni/ap-music-poc/internal/config"
)

// fakeStore は memio を包んだストレージのフェイクです。
//
// 一覧の畳み込み・不在の返し方・削除の単位といったストレージの振る舞いは memio が
// 受け持ちます（本物のハンドラと同じ適合性スイートを通っています）。ここに残しているのは
// 呼び出しの記録だけです。読み書きが 1 つの Store に畳まれたので、以前あった
// fakeHistoryReader と fakeHistoryWriter もこの型が兼ねます。
type fakeStore struct {
	remoteio.Store
	h *memio.Handler

	mu        sync.Mutex
	openCount map[string]int
	deleted   []string
}

// newFakeStore は、objects を置いたフェイクを返します。
// 値は内容で、空文字なら「一覧には出るが中身は要らない」オブジェクトです。
func newFakeStore(objects map[string]string) *fakeStore {
	s := &fakeStore{
		h:         memio.New(memio.WithScheme(remoteio.SchemeGCS)),
		openCount: map[string]int{},
	}
	s.Store = remoteio.NewStore(s.h)
	for uri, body := range objects {
		s.put(uri, body)
	}
	return s
}

// put は前提となるオブジェクトを置きます。
// 構築後の差し替えにもこれを使います。
func (s *fakeStore) put(uri, body string) {
	if err := s.h.Seed(uri, []byte(body)); err != nil {
		panic(err)
	}
}

func (s *fakeStore) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	s.mu.Lock()
	s.openCount[name]++
	s.mu.Unlock()

	return s.Store.Open(ctx, name)
}

func (s *fakeStore) Delete(ctx context.Context, name string) error {
	s.mu.Lock()
	s.deleted = append(s.deleted, name)
	s.mu.Unlock()

	return s.Store.Delete(ctx, name)
}

func (s *fakeStore) countOpen(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.openCount[name]
}

func (s *fakeStore) deletedPaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.deleted)
}

func TestListHistoryLoadsRecipeMetadata(t *testing.T) {
	t.Parallel()

	store := newFakeStore(map[string]string{
		"gs://music/20260501123456-abcd1234.json": `{
             "title": "テスト曲",
             "mood": "透明感",
             "tempo": 132,
             "compose_mode": "rave",
             "seed": 42
          }`,
		"gs://music/ignore.mp3": "",
	})
	repo := NewGCSMusicRepository(&config.Config{GCSBucket: "music"}, store, NewHistoryCache())

	histories, err := repo.ListHistory(context.Background(), "me")
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if len(histories) != 1 {
		t.Fatalf("len(histories) = %d, want 1", len(histories))
	}

	got := histories[0]
	if got.JobID != "20260501123456-abcd1234" {
		t.Fatalf("JobID = %q", got.JobID)
	}
	if got.Title != "テスト曲" {
		t.Fatalf("Title = %q", got.Title)
	}
	if got.Mood != "透明感" {
		t.Fatalf("Mood = %q", got.Mood)
	}
	if got.Tempo != 132 {
		t.Fatalf("Tempo = %d", got.Tempo)
	}
	if got.ComposeMode != "rave" {
		t.Fatalf("ComposeMode = %q", got.ComposeMode)
	}
	if got.Seed != "42" {
		t.Fatalf("Seed = %q", got.Seed)
	}
	if got.CreatedAt != "2026-05-01 21:34 JST" {
		t.Fatalf("CreatedAt = %q", got.CreatedAt)
	}
}

func TestListHistoryInitializesMissingCache(t *testing.T) {
	t.Parallel()

	const objectPath = "gs://music/20260501123456-abcd1234.json"
	store := newFakeStore(map[string]string{
		objectPath: `{"title":"キャッシュ未指定","tempo":132}`,
	})
	repo := NewGCSMusicRepository(&config.Config{GCSBucket: "music"}, store, nil)

	histories, err := repo.ListHistory(context.Background(), "me")
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if len(histories) != 1 {
		t.Fatalf("len(histories) = %d, want 1", len(histories))
	}
	if got := histories[0].Title; got != "キャッシュ未指定" {
		t.Fatalf("Title = %q, want キャッシュ未指定", got)
	}
}

func TestListHistoryUsesCachedMetadata(t *testing.T) {
	t.Parallel()

	const objectPath = "gs://music/20260501123456-abcd1234.json"
	store := newFakeStore(map[string]string{
		objectPath: `{"title":"初回タイトル","tempo":132}`,
	})
	repo := NewGCSMusicRepository(&config.Config{GCSBucket: "music"}, store, NewHistoryCache())

	if _, err := repo.ListHistory(context.Background(), "me"); err != nil {
		t.Fatalf("first ListHistory() error = %v", err)
	}
	store.put(objectPath, `{"title":"更新後タイトル","tempo":140}`)
	histories, err := repo.ListHistory(context.Background(), "me")
	if err != nil {
		t.Fatalf("second ListHistory() error = %v", err)
	}

	if got := store.countOpen(objectPath); got != 1 {
		t.Fatalf("Open count = %d, want 1", got)
	}
	if got := histories[0].Title; got != "初回タイトル" {
		t.Fatalf("cached Title = %q, want 初回タイトル", got)
	}
}

func TestDeleteHistoryInvalidatesCachedMetadata(t *testing.T) {
	t.Parallel()

	const objectPath = "gs://music/20260501123456-abcd1234.json"
	store := newFakeStore(map[string]string{
		objectPath: `{"title":"削除前タイトル","tempo":132}`,
	})
	repo := NewGCSMusicRepository(&config.Config{GCSBucket: "music"}, store, NewHistoryCache())

	if _, err := repo.ListHistory(context.Background(), "me"); err != nil {
		t.Fatalf("first ListHistory() error = %v", err)
	}
	if err := repo.DeleteHistory(context.Background(), "20260501123456-abcd1234"); err != nil {
		t.Fatalf("DeleteHistory() error = %v", err)
	}

	store.put(objectPath, `{"title":"削除後タイトル","tempo":140}`)
	histories, err := repo.ListHistory(context.Background(), "me")
	if err != nil {
		t.Fatalf("second ListHistory() error = %v", err)
	}

	if got := store.countOpen(objectPath); got != 2 {
		t.Fatalf("Open count = %d, want 2", got)
	}
	if got := histories[0].Title; got != "削除後タイトル" {
		t.Fatalf("Title after cache invalidation = %q, want 削除後タイトル", got)
	}
}

func TestDeleteHistoryInvalidatesCachedMetadataWithSanitizedJobID(t *testing.T) {
	t.Parallel()

	const (
		jobID      = "20260501123456-abcd1234"
		objectPath = "gs://music/20260501123456-abcd1234.json"
	)
	store := newFakeStore(map[string]string{
		objectPath: `{"title":"削除前タイトル","tempo":132}`,
	})
	repo := NewGCSMusicRepository(&config.Config{GCSBucket: "music"}, store, NewHistoryCache())

	if _, err := repo.ListHistory(context.Background(), "me"); err != nil {
		t.Fatalf("first ListHistory() error = %v", err)
	}
	if err := repo.DeleteHistory(context.Background(), "nested/"+jobID); err != nil {
		t.Fatalf("DeleteHistory() error = %v", err)
	}

	store.put(objectPath, `{"title":"削除後タイトル","tempo":140}`)
	histories, err := repo.ListHistory(context.Background(), "me")
	if err != nil {
		t.Fatalf("second ListHistory() error = %v", err)
	}

	if got := store.countOpen(objectPath); got != 2 {
		t.Fatalf("Open count = %d, want 2", got)
	}
	if got := histories[0].Title; got != "削除後タイトル" {
		t.Fatalf("Title after sanitized cache invalidation = %q, want 削除後タイトル", got)
	}
}

func TestDeleteHistoryDeletesRecipeAndAudioFiles(t *testing.T) {
	t.Parallel()

	store := newFakeStore(nil)
	repo := NewGCSMusicRepository(&config.Config{GCSBucket: "music"}, store, NewHistoryCache())

	if err := repo.DeleteHistory(context.Background(), "20260501123456-abcd1234"); err != nil {
		t.Fatalf("DeleteHistory() error = %v", err)
	}

	wantDeleted := []string{
		"gs://music/20260501123456-abcd1234.json",
		"gs://music/20260501123456-abcd1234.mp3",
	}
	deleted := store.deletedPaths()
	for _, want := range wantDeleted {
		if !slices.Contains(deleted, want) {
			t.Fatalf("deleted paths = %#v, missing %q", deleted, want)
		}
	}
}
