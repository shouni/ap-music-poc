package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/shouni/ap-music-poc/internal/domain"
)

// EnqueueTask は通常の作曲フォーム入力をジョブ化してキューに積みます。
func (h *Handler) EnqueueTask(w http.ResponseWriter, r *http.Request) {
	// POSTメソッド以外は許可しない
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// フォームデータのパースを実行
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	// タスクの構築
	task := h.taskFactory.BuildCompose(r.Form)
	// バリデーションチェック
	if err := task.ValidateSubmission(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.enqueueAndRespond(w, r, task)
}

// EnqueueGenerateFromRecipe はMusicRecipe JSONからPhase 4/5専用ジョブをキューに積みます。
func (h *Handler) EnqueueGenerateFromRecipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	task, err := h.taskFactory.BuildGenerateFromRecipe(r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := task.ValidateSubmission(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.enqueueAndRespond(w, r, task)
}

func (h *Handler) enqueueAndRespond(w http.ResponseWriter, r *http.Request, task domain.Task) {
	// Cloud Tasks 等へのエンキュー実行
	if err := h.taskEnqueuer.Enqueue(r.Context(), task); err != nil {
		slog.Error("failed to enqueue task", "job_id", task.JobID, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Acceptヘッダーを確認し、JSONを要求している場合はJSONでレスポンス
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "queued",
			"job_id": task.JobID,
		})
		return
	}

	// HTML レンダリング
	h.render(w, r, http.StatusAccepted, "accepted.html", "タスク受付完了", task)
}
