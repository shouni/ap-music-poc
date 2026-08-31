package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"

	"github.com/shouni/gcp-kit/tasks"
	"github.com/shouni/go-remote-io/remoteio"

	"github.com/shouni/ap-music-poc/assets"
	"github.com/shouni/ap-music-poc/internal/config"
	"github.com/shouni/ap-music-poc/internal/domain"
)

const titleSuffix = " - AP Music Poc"

// Handler はWeb画面、ジョブ投入、履歴操作のHTTPハンドラーをまとめます。
type Handler struct {
	cfg           *config.Config
	templateCache map[string]*template.Template
	taskEnqueuer  *tasks.Enqueuer[domain.Task]
	composeModes  []string
	taskFactory   *taskFactory
	store         remoteio.Store
	musicRepo     domain.MusicRepository
}

// NewHandler は指定された構成に基づいて新しいハンドラーを初期化します。
func NewHandler(
	cfg *config.Config,
	taskEnqueuer *tasks.Enqueuer[domain.Task],
	store remoteio.Store,
	musicRepo domain.MusicRepository,
) (*Handler, error) {
	cache := make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	entries, err := fs.ReadDir(assets.Templates, "templates")
	if err != nil {
		return nil, fmt.Errorf("テンプレートディレクトリの読み込み失敗: %w", err)
	}

	layoutPath := "templates/layout.html"
	if _, err := fs.Stat(assets.Templates, layoutPath); err != nil {
		return nil, fmt.Errorf("レイアウトテンプレートが見つかりません: %s", layoutPath)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "layout.html" {
			continue
		}

		pageName := entry.Name()
		pagePath := "templates/" + pageName

		tmpl, err := template.New(pageName).
			Funcs(funcMap).
			ParseFS(assets.Templates, layoutPath, pagePath)

		if err != nil {
			return nil, fmt.Errorf("テンプレート %s の解析失敗: %w", pageName, err)
		}
		cache[pageName] = tmpl
	}

	composePrompts, err := assets.LoadComposeFiles()
	if err != nil {
		return nil, fmt.Errorf("composeプロンプトの読み込み失敗: %w", err)
	}

	modes := make([]string, 0, len(composePrompts))
	for k := range composePrompts {
		modes = append(modes, k)
	}
	sort.Strings(modes)

	return &Handler{
		cfg:           cfg,
		templateCache: cache,
		taskEnqueuer:  taskEnqueuer,
		composeModes:  modes,
		taskFactory:   newTaskFactory(modes),
		store:         store,
		musicRepo:     musicRepo,
	}, nil
}

// Home はトップ画面を表示します。
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := struct {
		ComposeModes []string
	}{
		ComposeModes: h.composeModes,
	}
	h.render(w, r, http.StatusOK, "compose_form.html", "Compose", data)
}

// GenerateFromRecipe はMusicRecipe JSONから生成する画面を表示します。
func (h *Handler) GenerateFromRecipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.render(w, r, http.StatusOK, "generate_recipe_form.html", "Generate from Recipe", nil)
}

// render は HTML テンプレートをレンダリングし、レスポンスを書き込みます。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, pageName string, title string, data any) {
	tmpl, ok := h.templateCache[pageName]
	if !ok {
		slog.Error("キャッシュ内にテンプレートが見つかりません", "page", pageName)
		http.Error(w, "システムエラーが発生しました", http.StatusInternalServerError)
		return
	}

	renderData := struct {
		Title     string
		Data      any
		CSRFToken string
	}{
		Title:     title + titleSuffix,
		Data:      data,
		CSRFToken: csrfTokenFromContext(r.Context()),
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html", renderData); err != nil {
		slog.Error("テンプレートのレンダリングに失敗しました", "page", pageName, "error", err)
		http.Error(w, "画面の表示中にエラーが発生しました", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		slog.Error("レスポンスの書き込みに失敗しました", "error", err)
	}
}
