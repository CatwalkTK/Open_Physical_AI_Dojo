package repository

import "testing"

// Empty stores must return empty slices, not nil: nil marshals to JSON null,
// which breaks API clients that expect arrays.
func TestListReturnsEmptySlicesWhenFilesMissing(t *testing.T) {
	store, err := NewJSONLStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	episodes, err := store.ListEpisodes(0)
	if err != nil {
		t.Fatalf("list episodes: %v", err)
	}
	if episodes == nil {
		t.Error("ListEpisodes must return an empty slice, not nil")
	}

	evaluations, err := store.ListEvaluations(0)
	if err != nil {
		t.Fatalf("list evaluations: %v", err)
	}
	if evaluations == nil {
		t.Error("ListEvaluations must return an empty slice, not nil")
	}

	progress, err := store.ListLessonProgress()
	if err != nil {
		t.Fatalf("list lesson progress: %v", err)
	}
	if progress == nil {
		t.Error("ListLessonProgress must return an empty slice, not nil")
	}
}
