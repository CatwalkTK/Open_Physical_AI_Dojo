package service

import (
	"strings"
	"sync"
	"time"

	"open-physical-ai-dojo/backend/internal/domain"
)

// lessonStore is the persistence boundary the lesson service needs. The
// JSONL store satisfies it today; a SQL store can replace it later.
type lessonStore interface {
	SaveLessonProgress(progress domain.LessonProgress) error
	ListLessonProgress() ([]domain.LessonProgress, error)
	ListEpisodes(limit int) ([]domain.Task, error)
}

// lessonCatalog mirrors the 教材計画 in docs/implementation_plan_dogzilla.md.
var lessonCatalog = []domain.Lesson{
	{
		ID:          "lesson_1_vision",
		Order:       1,
		Title:       "カメラ画像と物体検出",
		Description: "Vision Viewerで画像認識を実行し、検出枠の見方を学びます。",
		Goal:        "物体を1つ以上検出する",
		Steps: []string{
			"Vision Viewerでサンプル画像または手持ちの画像を選ぶ",
			"Visionボタンを押して認識を実行する",
			"検出枠とラベル、信頼度を確認する",
		},
	},
	{
		ID:          "lesson_2_instruction",
		Order:       2,
		Title:       "日本語指示のJSON化",
		Description: "日本語指示から対象物と動作が抽出される様子を学びます。",
		Goal:        "対象物を含む指示から計画を生成する",
		Steps: []string{
			"「赤いブロックの近くまで移動して止まって」のような対象つき指示を入力する",
			"Planボタンを押して構造化結果を確認する",
			"Goalに対象物が含まれていることを確認する",
		},
	},
	{
		ID:          "lesson_3_plan",
		Order:       3,
		Title:       "行動計画",
		Description: "stand/move/turn/stopで構成される行動計画を作ります。",
		Goal:        "stand・move・stopを含む計画を生成する",
		Steps: []string{
			"移動を含む指示でPlanを実行する",
			"計画のステップ一覧を確認する",
			"開始のstandと終了のstopが入っていることを確認する",
		},
	},
	{
		ID:          "lesson_4_simulator",
		Order:       4,
		Title:       "シミュレータ実行",
		Description: "安全な仮想環境で行動計画を実行します。",
		Goal:        "シミュレータでタスクを成功させる",
		Steps: []string{
			"実行環境にSimulatorを選ぶ",
			"Planを作成してRunで実行する",
			"3Dビューアで軌跡と成功を確認する",
		},
	},
	{
		ID:          "lesson_5_dogzilla",
		Order:       5,
		Title:       "Dogzilla実機実行",
		Description: "Safety Guardを通して実機(またはmock)でタスクを実行します。",
		Goal:        "Dogzilla環境でタスクを成功させる",
		Steps: []string{
			"Dogzilla Statusで接続を確認する",
			"実行環境にDogzillaを選んでRunする",
			"Safety Limitsの範囲で動作することを確認する",
		},
	},
	{
		ID:          "lesson_6_analysis",
		Order:       6,
		Title:       "Episode分析",
		Description: "失敗ログを振り返り、指示や計画を改善します。",
		Goal:        "失敗したEpisodeの後に成功させる",
		Steps: []string{
			"HistoryのEpisodesで失敗または停止したログを見つける",
			"失敗理由を確認して指示を見直す",
			"改善した指示で再実行して成功させる",
		},
	},
}

type LessonService struct {
	mu    sync.Mutex
	store lessonStore

	// In-process signals that are not persisted as episodes.
	perceptionDetected bool
	targetPlanned      bool
	motionPlanned      bool
}

func NewLessonService(store lessonStore) *LessonService {
	return &LessonService{store: store}
}

// RecordPerception marks lesson 1 progress when a perception run detected
// at least one object.
func (s *LessonService) RecordPerception(result domain.PerceptionResult) {
	if len(result.Objects) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perceptionDetected = true
}

// RecordTaskCreated marks lesson 2 and 3 progress from the generated plan.
func (s *LessonService) RecordTaskCreated(task *domain.Task) {
	if task == nil || task.Plan == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if planReferencesTarget(task.Plan.Goal) {
		s.targetPlanned = true
	}
	if planHasMotionSequence(*task.Plan) {
		s.motionPlanned = true
	}
}

// Lessons evaluates all lesson criteria, persists newly completed lessons,
// and returns the catalog with completion status.
func (s *LessonService) Lessons() ([]domain.LessonWithStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	persisted, err := s.store.ListLessonProgress()
	if err != nil {
		return nil, err
	}
	completedAt := map[string]time.Time{}
	for _, progress := range persisted {
		if _, ok := completedAt[progress.LessonID]; !ok {
			completedAt[progress.LessonID] = progress.CompletedAt
		}
	}

	episodes, err := s.store.ListEpisodes(0)
	if err != nil {
		return nil, err
	}

	live := map[string]bool{
		"lesson_1_vision":      s.perceptionDetected,
		"lesson_2_instruction": s.targetPlanned,
		"lesson_3_plan":        s.motionPlanned,
		"lesson_4_simulator":   hasSucceededEpisode(episodes, domain.EnvironmentSimulator),
		"lesson_5_dogzilla":    hasSucceededEpisode(episodes, domain.EnvironmentDogzilla),
		"lesson_6_analysis":    hasRecoveryAfterFailure(episodes),
	}

	now := time.Now().UTC()
	lessons := make([]domain.LessonWithStatus, 0, len(lessonCatalog))
	for _, lesson := range lessonCatalog {
		doneAt, done := completedAt[lesson.ID]
		if !done && live[lesson.ID] {
			doneAt = now
			done = true
			if err := s.store.SaveLessonProgress(domain.LessonProgress{
				LessonID:    lesson.ID,
				CompletedAt: doneAt,
			}); err != nil {
				return nil, err
			}
		}
		status := domain.LessonWithStatus{Lesson: lesson, Completed: done}
		if done {
			completed := doneAt
			status.CompletedAt = &completed
		}
		lessons = append(lessons, status)
	}
	return lessons, nil
}

func planReferencesTarget(goal string) bool {
	for _, prefix := range []string{"approach_", "face_", "return_to_"} {
		if strings.Contains(goal, prefix) {
			return true
		}
	}
	return false
}

func planHasMotionSequence(plan domain.ActionPlan) bool {
	hasStand, hasMove, hasStop := false, false, false
	for _, step := range plan.Steps {
		switch step.Type {
		case "stand":
			hasStand = true
		case "move":
			hasMove = true
		case "stop":
			hasStop = true
		}
	}
	return hasStand && hasMove && hasStop
}

func hasSucceededEpisode(episodes []domain.Task, env domain.Environment) bool {
	for _, episode := range episodes {
		if episode.Environment == env && episode.Status == domain.TaskSucceeded {
			return true
		}
	}
	return false
}

// hasRecoveryAfterFailure detects the lesson 6 pattern: a failed or stopped
// episode followed later by a succeeded one.
func hasRecoveryAfterFailure(episodes []domain.Task) bool {
	var firstFailure *time.Time
	for _, episode := range episodes {
		if episode.Status == domain.TaskFailed || episode.Status == domain.TaskStopped {
			if firstFailure == nil || episode.UpdatedAt.Before(*firstFailure) {
				failedAt := episode.UpdatedAt
				firstFailure = &failedAt
			}
		}
	}
	if firstFailure == nil {
		return false
	}
	for _, episode := range episodes {
		if episode.Status == domain.TaskSucceeded && episode.UpdatedAt.After(*firstFailure) {
			return true
		}
	}
	return false
}
