package domain

import "testing"

func TestDecodeMusicRecipeJSON(t *testing.T) {
	t.Parallel()

	recipe, err := DecodeMusicRecipeJSON(`{"title":"Song","sections":[{"name":"Intro","duration_seconds":8,"prompt":"bright intro"}]}`)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if recipe.Title != "Song" {
		t.Fatalf("expected title Song, got %q", recipe.Title)
	}
	if len(recipe.Sections) != 1 {
		t.Fatalf("expected one section, got %d", len(recipe.Sections))
	}
}

func TestDecodeMusicRecipeJSONRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "invalid json", raw: "{not-json"},
		{name: "empty recipe", raw: "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := DecodeMusicRecipeJSON(tt.raw); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}
