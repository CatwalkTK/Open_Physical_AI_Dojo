package service

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"open-physical-ai-dojo/backend/internal/domain"
	"open-physical-ai-dojo/backend/internal/integration/robot"
	"open-physical-ai-dojo/backend/internal/repository"
)

func newTestService(t *testing.T, dogzilla *robot.DogzillaClient) *TaskService {
	t.Helper()
	return NewTaskService(dogzilla, nil, nil)
}

func TestCreateTaskValidation(t *testing.T) {
	s := newTestService(t, nil)

	if _, err := s.CreateTask(domain.CreateTaskRequest{Instruction: "   "}); err == nil {
		t.Fatal("expected empty instruction to be rejected")
	}
	if _, err := s.CreateTask(domain.CreateTaskRequest{
		Instruction: "進んで",
		Environment: "mars_rover",
	}); err == nil {
		t.Fatal("expected unknown environment to be rejected")
	}

	task, err := s.CreateTask(domain.CreateTaskRequest{Instruction: "進んで"})
	if err != nil {
		t.Fatalf("expected task creation to succeed, got: %v", err)
	}
	if task.Environment != domain.EnvironmentSimulator {
		t.Fatalf("expected default environment simulator, got %q", task.Environment)
	}
	if task.Status != domain.TaskQueued {
		t.Fatalf("expected queued status, got %q", task.Status)
	}
	if task.Plan == nil || len(task.Plan.Steps) == 0 {
		t.Fatal("expected an initial plan with steps")
	}
	if task.Simulator == nil {
		t.Fatal("expected simulator state for simulator task")
	}
}

func TestListTasksSortedNewestFirst(t *testing.T) {
	s := newTestService(t, nil)
	first, _ := s.CreateTask(domain.CreateTaskRequest{Instruction: "一つ目"})
	second, _ := s.CreateTask(domain.CreateTaskRequest{Instruction: "二つ目"})

	tasks := s.ListTasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != second.ID || tasks[1].ID != first.ID {
		t.Fatalf("expected newest first ordering, got %s, %s", tasks[0].ID, tasks[1].ID)
	}
}

func TestGeneratePlanApproachesTarget(t *testing.T) {
	s := newTestService(t, nil)

	plan := s.GeneratePlan(domain.PlanRequest{Instruction: "赤いブロックの近くまで移動して止まって"})
	if plan.Goal != "approach_red_block_and_stop" {
		t.Fatalf("expected approach_red_block_and_stop goal, got %q", plan.Goal)
	}
	if plan.Steps[0].Type != "stand" {
		t.Fatalf("expected plan to start with stand, got %q", plan.Steps[0].Type)
	}
	if plan.Steps[len(plan.Steps)-1].Type != "stop" {
		t.Fatalf("expected plan to end with stop, got %q", plan.Steps[len(plan.Steps)-1].Type)
	}
	if plan.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", plan.RiskLevel)
	}
	// Red block is at (0.42, 0.12): expect a left turn (~16 deg) then forward motion.
	if !planHasTurnDirection(plan, +1) {
		t.Fatal("expected a left (positive yaw) turn toward the red block")
	}
	if !planHasForward(plan) {
		t.Fatal("expected forward motion toward the red block")
	}
}

func TestGeneratePlanFaceTargetOnly(t *testing.T) {
	s := newTestService(t, nil)

	// Blue marker is at (0.34, -0.24): expect right (negative) turn, no forward.
	plan := s.GeneratePlan(domain.PlanRequest{Instruction: "青い目印の方向を向いて"})
	if plan.Goal != "face_blue_marker" {
		t.Fatalf("expected face_blue_marker goal, got %q", plan.Goal)
	}
	if !planHasTurnDirection(plan, -1) {
		t.Fatal("expected a right (negative yaw) turn toward the blue marker")
	}
	if planHasForward(plan) {
		t.Fatal("face-only plan must not move forward")
	}
}

func TestGeneratePlanRelativeTurns(t *testing.T) {
	s := newTestService(t, nil)

	// Positive yaw = left turn (ROS REP-103).
	right := s.GeneratePlan(domain.PlanRequest{Instruction: "右を向いて"})
	if right.Goal != "turn_right" || !planHasTurnDirection(right, -1) {
		t.Fatalf("expected turn_right with negative yaw, got %q", right.Goal)
	}
	if planHasForward(right) {
		t.Fatal("turn-only plan must not move forward")
	}
	left := s.GeneratePlan(domain.PlanRequest{Instruction: "左を向いて"})
	if left.Goal != "turn_left" || !planHasTurnDirection(left, +1) {
		t.Fatalf("expected turn_left with positive yaw, got %q", left.Goal)
	}
	around := s.GeneratePlan(domain.PlanRequest{Instruction: "後ろを向いて"})
	if around.Goal != "turn_around" {
		t.Fatalf("expected turn_around, got %q", around.Goal)
	}
}

func TestGeneratePlanCompoundInstruction(t *testing.T) {
	s := newTestService(t, nil)

	plan := s.GeneratePlan(domain.PlanRequest{Instruction: "10歩バックした後、赤いブロックの近くまで移動して止まって"})
	if plan.Goal != "move_backward_then_approach_red_block_and_stop" {
		t.Fatalf("unexpected goal: %q", plan.Goal)
	}
	backwardIndex, forwardIndex := -1, -1
	for i, step := range plan.Steps {
		if step.Type != "move" {
			continue
		}
		if step.LinearX < 0 && backwardIndex == -1 {
			backwardIndex = i
		}
		if step.LinearX > 0 && forwardIndex == -1 {
			forwardIndex = i
		}
	}
	if backwardIndex == -1 || forwardIndex == -1 || backwardIndex > forwardIndex {
		t.Fatalf("expected backward motion before forward motion, got backward=%d forward=%d", backwardIndex, forwardIndex)
	}

	comma := s.GeneratePlan(domain.PlanRequest{Instruction: "左の赤いブロックの方を向いて、近くまで移動して止まって"})
	if comma.Goal != "face_red_block_then_approach_red_block_and_stop" {
		t.Fatalf("unexpected comma-split goal: %q", comma.Goal)
	}
	if !planHasTurnDirection(comma, +1) {
		t.Fatal("expected turn toward red block")
	}
	if !planHasForward(comma) {
		t.Fatal("expected forward motion after facing target")
	}

	spin := s.GeneratePlan(domain.PlanRequest{Instruction: "一回転した後、青い目印の方向を向いて"})
	if spin.Goal != "spin_around_then_face_blue_marker" {
		t.Fatalf("unexpected goal: %q", spin.Goal)
	}
	if err := s.guard.ValidatePlan(domain.EnvironmentDogzilla, spin); err != nil {
		t.Fatalf("spin plan should respect safety limits: %v", err)
	}
}

func TestGeneratePlanBlueThenReturnRed(t *testing.T) {
	s := newTestService(t, nil)
	instruction := "青いブロックまで移動した後赤物の位置に戻って、赤いブロックに移動してストップ"
	plan := s.GeneratePlan(domain.PlanRequest{
		Instruction: instruction,
		Environment: domain.EnvironmentSimulator,
	})
	wantGoal := "approach_blue_marker_then_return_to_red_block_then_approach_red_block_and_stop"
	if plan.Goal != wantGoal {
		t.Fatalf("unexpected goal: %q want %q", plan.Goal, wantGoal)
	}
	if plan.Steps[len(plan.Steps)-1].Type != "stop" {
		t.Fatal("expected plan to end with stop")
	}
	// Must include motion toward blue, then toward red (return leg), and still finish.
	if !planHasForward(plan) {
		t.Fatal("expected forward motion in multi-leg plan")
	}
	turnCount := 0
	for _, step := range plan.Steps {
		if step.Type == "move" && step.YawDeg != 0 {
			turnCount++
		}
	}
	if turnCount < 2 {
		t.Fatalf("expected at least 2 turn phases (blue + return-to-red), got %d", turnCount)
	}
}

func TestGeneratePlanRespectsSafetyLimits(t *testing.T) {
	s := newTestService(t, nil)
	instructions := []string{
		"赤いブロックの近くまで移動して止まって",
		"青い目印の方向に進んで止まって",
		"机の端に近づいたら止まって",
		"後ろを向いて",
		"前に進んで",
		"一回転した後、青い目印の方向を向いて",
		"10歩バックした後、赤いブロックの近くまで移動して止まって",
		"左の赤いブロックの方を向いて、近くまで移動して止まって",
	}
	for _, instruction := range instructions {
		plan := s.GeneratePlan(domain.PlanRequest{Instruction: instruction, Environment: domain.EnvironmentDogzilla})
		if err := s.guard.ValidatePlan(domain.EnvironmentDogzilla, plan); err != nil {
			t.Errorf("plan for %q violates safety limits: %v", instruction, err)
		}
	}
}

func planHasTurnDirection(plan domain.ActionPlan, sign float64) bool {
	for _, step := range plan.Steps {
		if step.Type == "move" && step.YawDeg*sign > 0 {
			return true
		}
	}
	return false
}

func planHasForward(plan domain.ActionPlan) bool {
	for _, step := range plan.Steps {
		if step.Type == "move" && step.LinearX > 0 {
			return true
		}
	}
	return false
}

func TestStopOnlyCallsDogzillaForRealRobotTasks(t *testing.T) {
	var stopCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stop" {
			stopCalls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	s := newTestService(t, robot.NewDogzillaClient(server.URL))

	simTask, err := s.CreateTask(domain.CreateTaskRequest{
		Instruction: "進んで止まって",
		Environment: domain.EnvironmentSimulator,
	})
	if err != nil {
		t.Fatalf("create simulator task: %v", err)
	}
	stopped, err := s.Stop(simTask.ID)
	if err != nil {
		t.Fatalf("stop simulator task: %v", err)
	}
	if stopped.Status != domain.TaskStopped {
		t.Fatalf("expected stopped status, got %q", stopped.Status)
	}
	if got := stopCalls.Load(); got != 0 {
		t.Fatalf("simulator stop must not call dogzilla runtime, got %d calls", got)
	}

	realTask, err := s.CreateTask(domain.CreateTaskRequest{
		Instruction: "進んで止まって",
		Environment: domain.EnvironmentDogzilla,
	})
	if err != nil {
		t.Fatalf("create dogzilla task: %v", err)
	}
	if _, err := s.Stop(realTask.ID); err != nil {
		t.Fatalf("stop dogzilla task: %v", err)
	}
	if got := stopCalls.Load(); got != 1 {
		t.Fatalf("dogzilla stop must call runtime exactly once, got %d calls", got)
	}
}

func TestStopWhileRunningPersistsStoppedEpisode(t *testing.T) {
	store, err := repository.NewJSONLStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	s := NewTaskService(nil, nil, store)

	task, err := s.CreateTask(domain.CreateTaskRequest{
		Instruction: "赤いブロックの近くまで移動して止まって",
		Environment: domain.EnvironmentSimulator,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := s.Execute(task.ID); err != nil {
		t.Fatalf("execute task: %v", err)
	}
	stopped, err := s.Stop(task.ID)
	if err != nil {
		t.Fatalf("stop task: %v", err)
	}
	if stopped.Status != domain.TaskStopped {
		t.Fatalf("expected stopped status, got %q", stopped.Status)
	}

	episodes, err := store.ListEpisodes(0)
	if err != nil {
		t.Fatalf("list episodes: %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("expected 1 persisted episode, got %d", len(episodes))
	}
	if episodes[0].ID != task.ID || episodes[0].Status != domain.TaskStopped {
		t.Fatalf("expected stopped episode for %s, got %s/%s", task.ID, episodes[0].ID, episodes[0].Status)
	}

	// Stopping a queued task is not an execution, so no episode is written.
	queued, err := s.CreateTask(domain.CreateTaskRequest{
		Instruction: "前に進んで",
		Environment: domain.EnvironmentSimulator,
	})
	if err != nil {
		t.Fatalf("create queued task: %v", err)
	}
	if _, err := s.Stop(queued.ID); err != nil {
		t.Fatalf("stop queued task: %v", err)
	}
	episodes, err = store.ListEpisodes(0)
	if err != nil {
		t.Fatalf("list episodes: %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("stopping a queued task must not persist an episode, got %d episodes", len(episodes))
	}
}

func TestStopUnknownTask(t *testing.T) {
	s := newTestService(t, nil)
	if _, err := s.Stop("task_9999"); err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestNormalizeYaw(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{180, 180},
		{190, -170},
		{-190, 170},
		{540, 180},
	}
	for _, tt := range tests {
		if got := normalizeYaw(tt.in); got != tt.want {
			t.Errorf("normalizeYaw(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestEstimateFinalX(t *testing.T) {
	plan := domain.ActionPlan{Steps: []domain.ActionStep{
		{Type: "stand"},
		{Type: "move", LinearX: 0.1, DurationMS: 1000},
		{Type: "stop"},
	}}
	got := estimateFinalX(plan)
	if got < 0.099 || got > 0.101 {
		t.Fatalf("expected ~0.1m forward, got %v", got)
	}

	// Zero-duration moves use the 300ms execution minimum.
	short := domain.ActionPlan{Steps: []domain.ActionStep{
		{Type: "move", LinearX: 0.1},
	}}
	got = estimateFinalX(short)
	if got < 0.029 || got > 0.031 {
		t.Fatalf("expected ~0.03m for zero-duration move, got %v", got)
	}
}

func TestEvaluationFailureReason(t *testing.T) {
	if reason := evaluationFailureReason(false, true, "low", true, 1, true); reason != "expected object was not detected" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if reason := evaluationFailureReason(true, false, "low", true, 1, true); reason != "plan did not include stop step" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if reason := evaluationFailureReason(true, true, "high", true, 1, true); reason != "plan risk level was not low" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if reason := evaluationFailureReason(true, true, "low", true, 0.01, true); reason != "simulated movement was too short" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if reason := evaluationFailureReason(true, true, "low", false, 0, false); reason != "plan did not include a turn toward the target" {
		t.Fatalf("unexpected reason: %q", reason)
	}
}
