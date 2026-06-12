package service

import (
	"testing"
	"time"

	"open-physical-ai-dojo/backend/internal/domain"
	"open-physical-ai-dojo/backend/internal/repository"
)

func newTestLessonService(t *testing.T, dataDir string) *LessonService {
	t.Helper()
	store, err := repository.NewJSONLStore(dataDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return NewLessonService(store)
}

func lessonByID(t *testing.T, lessons []domain.LessonWithStatus, id string) domain.LessonWithStatus {
	t.Helper()
	for _, lesson := range lessons {
		if lesson.ID == id {
			return lesson
		}
	}
	t.Fatalf("lesson %q not found", id)
	return domain.LessonWithStatus{}
}

func TestLessonsInitiallyIncomplete(t *testing.T) {
	s := newTestLessonService(t, t.TempDir())

	lessons, err := s.Lessons()
	if err != nil {
		t.Fatalf("list lessons: %v", err)
	}
	if len(lessons) != 6 {
		t.Fatalf("expected 6 lessons, got %d", len(lessons))
	}
	for i, lesson := range lessons {
		if lesson.Completed {
			t.Errorf("lesson %s should start incomplete", lesson.ID)
		}
		if lesson.Order != i+1 {
			t.Errorf("expected lessons ordered 1..6, got order %d at index %d", lesson.Order, i)
		}
	}
}

func TestPerceptionDetectionCompletesLesson1(t *testing.T) {
	s := newTestLessonService(t, t.TempDir())

	s.RecordPerception(domain.PerceptionResult{Objects: nil})
	lessons, _ := s.Lessons()
	if lessonByID(t, lessons, "lesson_1_vision").Completed {
		t.Fatal("perception without detections must not complete lesson 1")
	}

	s.RecordPerception(domain.PerceptionResult{Objects: []domain.DetectedObject{{Label: "red_block"}}})
	lessons, _ = s.Lessons()
	if !lessonByID(t, lessons, "lesson_1_vision").Completed {
		t.Fatal("perception with a detection must complete lesson 1")
	}
}

func TestTaskCreationCompletesLessons2And3(t *testing.T) {
	s := newTestLessonService(t, t.TempDir())

	// A fallback plan with no recognized target completes neither lesson 2 nor 3.
	s.RecordTaskCreated(&domain.Task{Plan: &domain.ActionPlan{
		Goal:  "move_forward",
		Steps: []domain.ActionStep{{Type: "stand"}, {Type: "stop"}},
	}})
	lessons, _ := s.Lessons()
	if lessonByID(t, lessons, "lesson_2_instruction").Completed {
		t.Fatal("plan without a recognized target must not complete lesson 2")
	}
	if lessonByID(t, lessons, "lesson_3_plan").Completed {
		t.Fatal("plan without movement must not complete lesson 3")
	}

	// A target-directed plan with stand/move/stop completes both.
	s.RecordTaskCreated(&domain.Task{Plan: &domain.ActionPlan{
		Goal: "approach_red_block_and_stop",
		Steps: []domain.ActionStep{
			{Type: "stand"},
			{Type: "move", LinearX: 0.08, DurationMS: 1200},
			{Type: "stop"},
		},
	}})
	lessons, _ = s.Lessons()
	if !lessonByID(t, lessons, "lesson_2_instruction").Completed {
		t.Fatal("target-directed plan must complete lesson 2")
	}
	if !lessonByID(t, lessons, "lesson_3_plan").Completed {
		t.Fatal("stand/move/stop plan must complete lesson 3")
	}
}

func TestEpisodesCompleteExecutionLessons(t *testing.T) {
	dataDir := t.TempDir()
	store, err := repository.NewJSONLStore(dataDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	s := NewLessonService(store)

	now := time.Now().UTC()
	mustSave := func(task domain.Task) {
		t.Helper()
		if err := store.SaveEpisode(&task); err != nil {
			t.Fatalf("save episode: %v", err)
		}
	}

	mustSave(domain.Task{ID: "task_0001", Environment: domain.EnvironmentSimulator, Status: domain.TaskSucceeded, UpdatedAt: now})
	lessons, _ := s.Lessons()
	if !lessonByID(t, lessons, "lesson_4_simulator").Completed {
		t.Fatal("succeeded simulator episode must complete lesson 4")
	}
	if lessonByID(t, lessons, "lesson_5_dogzilla").Completed {
		t.Fatal("lesson 5 needs a dogzilla episode")
	}

	mustSave(domain.Task{ID: "task_0002", Environment: domain.EnvironmentDogzilla, Status: domain.TaskSucceeded, UpdatedAt: now.Add(time.Minute)})
	lessons, _ = s.Lessons()
	if !lessonByID(t, lessons, "lesson_5_dogzilla").Completed {
		t.Fatal("succeeded dogzilla episode must complete lesson 5")
	}
}

func TestFailureThenSuccessCompletesLesson6(t *testing.T) {
	dataDir := t.TempDir()
	store, err := repository.NewJSONLStore(dataDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	s := NewLessonService(store)

	now := time.Now().UTC()
	failed := domain.Task{ID: "task_0001", Environment: domain.EnvironmentSimulator, Status: domain.TaskFailed, UpdatedAt: now}
	if err := store.SaveEpisode(&failed); err != nil {
		t.Fatalf("save episode: %v", err)
	}
	lessons, _ := s.Lessons()
	if lessonByID(t, lessons, "lesson_6_analysis").Completed {
		t.Fatal("a failure alone must not complete lesson 6")
	}

	succeeded := domain.Task{ID: "task_0002", Environment: domain.EnvironmentSimulator, Status: domain.TaskSucceeded, UpdatedAt: now.Add(time.Minute)}
	if err := store.SaveEpisode(&succeeded); err != nil {
		t.Fatalf("save episode: %v", err)
	}
	lessons, _ = s.Lessons()
	if !lessonByID(t, lessons, "lesson_6_analysis").Completed {
		t.Fatal("a success after a failure must complete lesson 6")
	}
}

func TestLessonProgressPersistsAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()

	first := newTestLessonService(t, dataDir)
	first.RecordPerception(domain.PerceptionResult{Objects: []domain.DetectedObject{{Label: "red_block"}}})
	if _, err := first.Lessons(); err != nil {
		t.Fatalf("list lessons: %v", err)
	}

	// In-memory signals are gone after a restart; persisted progress must remain.
	second := newTestLessonService(t, dataDir)
	lessons, err := second.Lessons()
	if err != nil {
		t.Fatalf("list lessons after restart: %v", err)
	}
	if !lessonByID(t, lessons, "lesson_1_vision").Completed {
		t.Fatal("lesson 1 completion must survive a restart")
	}
}
