package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/shouni/ap-music-poc/internal/domain"
)

// jobIDRegex は JobID のバリデーション用正規表現です。
// 英数字とハイフンのみを許可し、パストラバーサル等を防ぎます。
var jobIDRegex = regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)

// ServeHistory は楽曲生成履歴の一覧画面を表示するのだ。
func (h *Handler) ServeHistory(w http.ResponseWriter, r *http.Request) {
	// TODO: 認証機能実装後、セッションから userID を取得するように修正する
	userID := "me"
	histories, err := h.musicRepo.ListHistory(r.Context(), userID)
	if err != nil {
		http.Error(w, "履歴の取得に失敗したのだ", http.StatusInternalServerError)
		return
	}

	h.render(w, r, http.StatusOK, "history.html", "History", histories)
}

// ServeDetails は特定の楽曲の詳細画面を表示するのだ。
func (h *Handler) ServeDetails(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	// バリデーション：セキュリティとパスの安全性を確保
	if !jobIDRegex.MatchString(jobID) {
		http.Error(w, "Invalid JobID format", http.StatusBadRequest)
		return
	}

	recipe, err := h.musicRepo.GetRecipe(r.Context(), jobID)
	if err != nil {
		http.Error(w, "レシピが見つからないのだ", http.StatusNotFound)
		return
	}

	// テンプレートの JSON タブに表示するための整形済み JSON を作成
	recipeJSON, err := formatRecipeJSONForDisplay(recipe)
	if err != nil {
		slog.ErrorContext(r.Context(), "JSONの整形に失敗したのだ", "jobID", jobID, "error", err)
		recipeJSON = "{}"
	}

	audioURL := fmt.Sprintf("/web/audio/%s", jobID)

	data := struct {
		ID         string
		Recipe     *domain.MusicRecipe
		RecipeJSON string
		AudioURL   string
	}{
		ID:         jobID,
		Recipe:     recipe,
		RecipeJSON: recipeJSON,
		AudioURL:   audioURL,
	}

	h.render(w, r, http.StatusOK, "music_view.html", recipe.Title, data)
}

func formatRecipeJSONForDisplay(recipe *domain.MusicRecipe) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(recipe); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// ServeAudio は、GCSの署名付きURLへリダイレクトさせることで音声を配信します。
func (h *Handler) ServeAudio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := chi.URLParam(r, "jobID")

	// バリデーション：共通の正規表現を使用
	if !jobIDRegex.MatchString(jobID) {
		http.Error(w, "Invalid JobID format", http.StatusBadRequest)
		return
	}

	// ファイル名の組み立て
	fileName := fmt.Sprintf("%s%s", jobID, domain.AudioFileExtension)
	gcsURL := h.cfg.GetGCSObjectURL(fileName)

	// 署名付きURLの生成（直接GCSから配信させてサーバー負荷を軽減）
	signedURL, err := h.store.SignURL(
		ctx,
		gcsURL,
		http.MethodGet,
		1*time.Hour,
	)
	if err != nil {
		slog.Error("Failed to generate signed URL", "jobID", jobID, "error", err)
		http.Error(w, "Audio access error", http.StatusInternalServerError)
		return
	}

	// クライアントを直接 GCS へ飛ばす
	http.Redirect(w, r, signedURL, http.StatusFound)
}

// DeleteHistory は、指定されたジョブIDの履歴とファイルを削除します。
func (h *Handler) DeleteHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	jobID := chi.URLParam(r, "jobID")

	// バリデーション：既存の正規表現を使用してパスの安全性を確保
	if !jobIDRegex.MatchString(jobID) {
		http.Error(w, "Invalid JobID format", http.StatusBadRequest)
		return
	}

	// リポジトリ経由で GCS 上の JSON と音声ファイルを削除
	if err := h.musicRepo.DeleteHistory(ctx, jobID); err != nil {
		slog.ErrorContext(ctx, "履歴の削除に失敗したのだ", "jobID", jobID, "error", err)
		http.Error(w, "Failed to delete history", http.StatusInternalServerError)
		return
	}

	// 削除成功時は 204 No Content を返す（Fetch API での呼び出しを想定）
	w.WriteHeader(http.StatusNoContent)
}
