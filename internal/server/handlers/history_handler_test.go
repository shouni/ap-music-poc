package handlers

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/shouni/ap-music-poc/internal/domain"
)

type stubMusicRepository struct {
	histories    []domain.MusicHistory
	recipe       *domain.MusicRecipe
	deletedJobID string
}

func (r *stubMusicRepository) ListHistory(context.Context, string) ([]domain.MusicHistory, error) {
	return r.histories, nil
}

func (r *stubMusicRepository) GetRecipe(context.Context, string) (*domain.MusicRecipe, error) {
	return r.recipe, nil
}

func (r *stubMusicRepository) DeleteHistory(_ context.Context, jobID string) error {
	r.deletedJobID = jobID
	return nil
}

func TestServeHistoryRendersDeleteControls(t *testing.T) {
	t.Parallel()

	h, err := NewHandler(nil, nil, nil, &stubMusicRepository{
		histories: []domain.MusicHistory{
			{
				JobID:       "job-list-1",
				Title:       "一覧の曲",
				Mood:        "透明感",
				Tempo:       132,
				CreatedAt:   "2026-05-01",
				ComposeMode: "rave",
				Seed:        "42",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/web/history/", nil)
	rec := httptest.NewRecorder()

	h.ServeHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`onclick="deleteHistory('job-list-1', event)"`,
		`onclick="togglePlaylist(event)"`,
		`id="playlist-toggle-btn"`,
		`一括再生`,
		`id="history-item-job-list-1"`,
		`.history-playing`,
		`function setPlayingItem(jobID)`,
		`data-job-id="job-list-1"`,
		`function playNextPlaylistAudio()`,
		"`/web/history/${jobID}`",
		`'X-CSRF-Token': csrfToken`,
		`id="csrf_token"`,
		`透明感`,
		`132 BPM`,
		`rave`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("response body does not contain %q", want)
		}
	}
}

func TestServeDetailsRendersRecipeJSONAsUTF8(t *testing.T) {
	t.Parallel()

	recipe := &domain.MusicRecipe{
		Title: "朝焼け<テスト>&構成",
		Theme: "日本語ボーカル",
		Mood:  "透明感",
		Sections: []domain.MusicSection{
			{Name: "サビ", Duration: 30, Prompt: "高揚感のある歌声"},
		},
		Lyrics: &domain.LyricsDraft{
			Lyrics: "きみの声が\n朝に溶ける",
		},
	}

	h, err := NewHandler(nil, nil, nil, &stubMusicRepository{recipe: recipe})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/web/history/job-utf8", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("jobID", "job-utf8")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()

	h.ServeDetails(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "朝焼け") || !strings.Contains(body, "高揚感のある歌声") {
		t.Fatalf("response body does not contain expected Japanese text: %s", body)
	}
	for _, want := range []string{
		`onclick="deleteHistory('job-utf8')"`,
		"`/web/history/${jobID}`",
		`'X-CSRF-Token': csrfToken`,
		`id="csrf_token"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("response body does not contain %q", want)
		}
	}

	if strings.Contains(body, `\u003c`) || strings.Contains(body, `\u003e`) || strings.Contains(body, `\u0026`) {
		t.Fatalf("display JSON contains avoidable HTML unicode escapes: %s", body)
	}

	jsonText := extractCodeText(t, body)
	var rendered domain.MusicRecipe
	if err := json.Unmarshal([]byte(jsonText), &rendered); err != nil {
		t.Fatalf("rendered JSON is invalid: %v\n%s", err, jsonText)
	}
	if rendered.Title != recipe.Title {
		t.Fatalf("rendered title = %q, want %q", rendered.Title, recipe.Title)
	}
}

func extractCodeText(t *testing.T, body string) string {
	t.Helper()

	preStart := strings.Index(body, `<pre id="json-raw"`)
	if preStart < 0 {
		t.Fatalf("json pre block not found")
	}
	codeStart := strings.Index(body[preStart:], "<code>")
	if codeStart < 0 {
		t.Fatalf("json code block not found")
	}
	codeStart += preStart + len("<code>")
	codeEnd := strings.Index(body[codeStart:], "</code>")
	if codeEnd < 0 {
		t.Fatalf("json code block end not found")
	}
	return html.UnescapeString(body[codeStart : codeStart+codeEnd])
}

func TestDeleteHistory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		jobID          string
		expectedStatus int
	}{
		{
			name:           "Valid JobID should return No Content",
			method:         http.MethodDelete,
			jobID:          "valid-job-123",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "Invalid JobID (path traversal) should return Bad Request",
			method:         http.MethodDelete,
			jobID:          "../forbidden",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JobID (special characters) should return Bad Request",
			method:         http.MethodDelete,
			jobID:          "job@id!",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Wrong method should return Method Not Allowed",
			method:         http.MethodGet,
			jobID:          "valid-job-123",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubMusicRepository{}
			h, _ := NewHandler(nil, nil, nil, repo)

			req := httptest.NewRequest(tt.method, "/web/history/"+tt.jobID, nil)
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("jobID", tt.jobID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

			rec := httptest.NewRecorder()
			h.DeleteHistory(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			if tt.expectedStatus == http.StatusNoContent && repo.deletedJobID != tt.jobID {
				t.Errorf("deleted jobID = %q, want %q", repo.deletedJobID, tt.jobID)
			}
		})
	}
}
