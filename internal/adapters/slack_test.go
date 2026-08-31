package adapters

import (
	"strings"
	"testing"

	"github.com/shouni/go-notify/notify"

	"github.com/shouni/ap-music-poc/internal/domain"
)

// 記法の変換（Markdown から Slack mrkdwn へ）は go-notify 側の責務なので、
// ここでは ap-music-poc が本文へ載せる項目だけを確かめます。
func TestSlackContentIncludesCommand(t *testing.T) {
	t.Parallel()

	adapter := &SlackAdapter{serviceURL: "https://example.com"}
	content := adapter.buildSlackContent(&domain.PublishResult{
		JobID:      "job-1",
		StorageURI: "gs://bucket/job-1.mp3",
	}, domain.NotificationRequest{
		Command: string(domain.TaskCommandGenerateFromRecipe),
	})

	if !strings.Contains(content, "**Command:** `generate_from_recipe`") {
		t.Fatalf("expected command in slack content, got %q", content)
	}
}

func TestSlackMetadataIncludesCommand(t *testing.T) {
	t.Parallel()

	body := notify.NewBody()
	writeSlackRequestMetadata(body, domain.NotificationRequest{
		Command: string(domain.TaskCommandCompose),
		Title:   "Midnight Recipe",
		Mode:    "rave",
	})

	got := body.String()
	if !strings.Contains(got, "**Command:** `compose`") {
		t.Fatalf("expected command in slack metadata, got %q", got)
	}
	if !strings.Contains(got, "**Title:** Midnight Recipe") {
		t.Fatalf("expected title in slack metadata, got %q", got)
	}
	if !strings.Contains(got, "**Mode:** `rave`") {
		t.Fatalf("expected mode in slack metadata, got %q", got)
	}
}
