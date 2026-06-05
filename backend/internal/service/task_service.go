package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"open-physical-ai-dojo/backend/internal/domain"
	"open-physical-ai-dojo/backend/internal/integration/perception"
	"open-physical-ai-dojo/backend/internal/integration/robot"
	"open-physical-ai-dojo/backend/internal/repository"
	"open-physical-ai-dojo/backend/internal/safety"
	"open-physical-ai-dojo/backend/internal/stream"
)

type TaskService struct {
	mu       sync.Mutex
	nextID   int
	tasks    map[string]*domain.Task
	dogzilla *robot.DogzillaClient
	vision   *perception.Client
	store    *repository.JSONLStore
	guard    safety.Guard
	broker   *stream.Broker
}

func NewTaskService(dogzilla *robot.DogzillaClient, vision *perception.Client, store *repository.JSONLStore) *TaskService {
	return &TaskService{
		tasks:    map[string]*domain.Task{},
		dogzilla: dogzilla,
		vision:   vision,
		store:    store,
		guard:    safety.NewGuard(),
		broker:   stream.NewBroker(),
	}
}

func (s *TaskService) Stream() *stream.Broker {
	return s.broker
}

func (s *TaskService) CreateTask(req domain.CreateTaskRequest) (*domain.Task, error) {
	if strings.TrimSpace(req.Instruction) == "" {
		return nil, errors.New("instruction is required")
	}
	if req.Environment == "" {
		req.Environment = domain.EnvironmentSimulator
	}
	if req.Environment != domain.EnvironmentSimulator && req.Environment != domain.EnvironmentDogzilla {
		return nil, errors.New("environment must be simulator or dogzilla")
	}

	plan := s.GeneratePlan(domain.PlanRequest{Instruction: req.Instruction, Environment: req.Environment})
	if err := s.guard.ValidatePlan(req.Environment, plan); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	task := &domain.Task{
		ID:          fmt.Sprintf("task_%04d", s.nextID),
		Instruction: req.Instruction,
		Environment: req.Environment,
		Status:      domain.TaskQueued,
		Plan:        &plan,
		Simulator:   initialSimulatorState(req.Environment, now),
		Events: []domain.TaskEvent{{
			Time:    now,
			Type:    "created",
			Message: "task created and initial plan generated",
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.tasks[task.ID] = task
	return cloneTask(task), nil
}

func (s *TaskService) GetTask(id string) (*domain.Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	return cloneTask(task), true
}

func (s *TaskService) ListTasks() []*domain.Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]*domain.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, cloneTask(task))
	}
	return tasks
}

func (s *TaskService) ListPersistedEpisodes(limit int) ([]domain.Task, error) {
	return s.store.ListEpisodes(limit)
}

func (s *TaskService) ListPersistedEvaluations(limit int) ([]domain.EvaluationResult, error) {
	return s.store.ListEvaluations(limit)
}

func (s *TaskService) GeneratePlan(req domain.PlanRequest) domain.ActionPlan {
	instruction := req.Instruction
	goal := "approach_target"
	steps := []domain.ActionStep{{Type: "stand"}}

	if strings.Contains(instruction, "右") {
		steps = append(steps, domain.ActionStep{Type: "move", YawDeg: 15, DurationMS: 600})
	}
	if strings.Contains(instruction, "左") {
		steps = append(steps, domain.ActionStep{Type: "move", YawDeg: -15, DurationMS: 600})
	}
	if strings.Contains(instruction, "止") || strings.Contains(strings.ToLower(instruction), "stop") {
		goal = "approach_and_stop"
	}

	steps = append(steps,
		domain.ActionStep{Type: "move", LinearX: 0.08, DurationMS: 1200},
		domain.ActionStep{Type: "stop"},
	)

	return domain.ActionPlan{
		Goal:      goal,
		Steps:     steps,
		RiskLevel: "low",
	}
}

func (s *TaskService) RunPerception(req domain.PerceptionRequest) domain.PerceptionResult {
	result, err := s.vision.Run(req)
	if err != nil {
		return domain.PerceptionResult{
			Source:  req.Source,
			Summary: "perception service error: " + err.Error(),
		}
	}
	return result
}

func (s *TaskService) PerceptionStatus() domain.PerceptionServiceStatus {
	status := domain.PerceptionServiceStatus{
		Connected:   false,
		ServiceURL:  s.vision.BaseURL(),
		LastChecked: time.Now().UTC(),
	}
	if err := s.vision.Health(); err != nil {
		status.Error = err.Error()
		return status
	}
	status.Connected = true
	return status
}

func (s *TaskService) DogzillaStatus() domain.DogzillaRuntimeStatus {
	status := domain.DogzillaRuntimeStatus{
		Connected:  false,
		RuntimeURL: s.dogzilla.BaseURL(),
		Safety: domain.DogzillaSafety{
			EmergencyStopAvailable: true,
			MaxLinearX:             0.12,
			MaxLinearY:             0.08,
			MaxYawDeg:              30,
			MaxDurationMS:          1800,
		},
		LastChecked: time.Now().UTC(),
	}

	if err := s.dogzilla.Health(); err != nil {
		status.Error = err.Error()
		return status
	}
	state, err := s.dogzilla.State()
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Connected = true
	status.State = state
	return status
}

func (s *TaskService) RunEvaluation(req domain.EvaluationRequest) domain.EvaluationResult {
	suite := strings.TrimSpace(req.Suite)
	if suite == "" {
		suite = "simulator_basic"
	}

	scenarios := []struct {
		id             string
		instruction    string
		expectedObject string
	}{
		{id: "case_001", instruction: "赤いブロックの近くまで移動して止まって", expectedObject: "red_block"},
		{id: "case_002", instruction: "左の赤いブロックの方向を向いて止まって", expectedObject: "red_block"},
		{id: "case_003", instruction: "青い目印の方向に少し進んで止まって", expectedObject: "blue_marker"},
	}

	cases := make([]domain.EvaluationCase, 0, len(scenarios))
	totalConfidence := 0.0
	totalFinalX := 0.0
	passed := 0

	for _, scenario := range scenarios {
		perception := s.RunPerception(domain.PerceptionRequest{
			Source:      "sample_workbench",
			Instruction: scenario.instruction,
		})
		plan := s.GeneratePlan(domain.PlanRequest{
			Instruction: scenario.instruction,
			Environment: domain.EnvironmentSimulator,
		})
		labels := make([]string, 0, len(perception.Objects))
		confidence := 0.0
		detectedExpected := false
		for _, object := range perception.Objects {
			labels = append(labels, object.Label)
			confidence += object.Confidence
			if object.Label == scenario.expectedObject {
				detectedExpected = true
			}
		}
		if len(perception.Objects) > 0 {
			confidence = confidence / float64(len(perception.Objects))
		}

		finalX := estimateFinalX(plan)
		hasStop := planHasStep(plan, "stop")
		casePassed := detectedExpected && hasStop && plan.RiskLevel == "low" && finalX >= 0.05
		failureReason := ""
		if !casePassed {
			failureReason = evaluationFailureReason(detectedExpected, hasStop, plan.RiskLevel, finalX)
		}
		if casePassed {
			passed++
		}
		totalConfidence += confidence
		totalFinalX += finalX
		cases = append(cases, domain.EvaluationCase{
			ID:                   scenario.id,
			Instruction:          scenario.instruction,
			ExpectedObject:       scenario.expectedObject,
			DetectedObjectLabels: labels,
			PlanGoal:             plan.Goal,
			FinalX:               finalX,
			Passed:               casePassed,
			FailureReason:        failureReason,
		})
	}

	totalCases := len(cases)
	result := domain.EvaluationResult{
		ID:    fmt.Sprintf("eval_%d", time.Now().UTC().Unix()),
		Suite: suite,
		Summary: domain.EvaluationSummary{
			TotalCases:        totalCases,
			PassedCases:       passed,
			SuccessRate:       float64(passed) / float64(totalCases),
			AverageConfidence: totalConfidence / float64(totalCases),
			AverageFinalX:     totalFinalX / float64(totalCases),
		},
		Cases:     cases,
		CreatedAt: time.Now().UTC(),
	}
	_ = s.store.SaveEvaluation(result)
	return result
}

func (s *TaskService) EmergencyStopDogzilla() domain.DogzillaRuntimeStatus {
	status := s.DogzillaStatus()
	if err := s.dogzilla.Stop(); err != nil {
		status.Connected = false
		status.Error = err.Error()
		status.LastChecked = time.Now().UTC()
		return status
	}
	return s.DogzillaStatus()
}

func (s *TaskService) Execute(taskID string) (*domain.Task, error) {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return nil, errors.New("task not found")
	}
	if task.Plan == nil {
		s.mu.Unlock()
		return nil, errors.New("task has no plan")
	}
	task.Status = domain.TaskRunning
	task.UpdatedAt = time.Now().UTC()
	s.addEventLocked(task, "running", "task execution started")
	s.mu.Unlock()
	s.publish(taskID)

	go s.runTask(taskID)

	updated, _ := s.GetTask(taskID)
	return updated, nil
}

func (s *TaskService) Stop(taskID string) (*domain.Task, error) {
	if err := s.dogzilla.Stop(); err != nil {
		s.appendEvent(taskID, "stop_error", err.Error())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, errors.New("task not found")
	}
	task.Status = domain.TaskStopped
	task.UpdatedAt = time.Now().UTC()
	s.addEventLocked(task, "stopped", "stop requested")
	s.broker.Publish(taskID, task)
	return cloneTask(task), nil
}

func (s *TaskService) runTask(taskID string) {
	task, ok := s.GetTask(taskID)
	if !ok || task.Plan == nil {
		return
	}

	if task.Environment == domain.EnvironmentDogzilla {
		if err := s.dogzilla.Health(); err != nil {
			s.fail(taskID, "dogzilla runtime is not healthy: "+err.Error())
			return
		}
	}

	for _, step := range task.Plan.Steps {
		if s.currentStatus(taskID) == domain.TaskStopped {
			return
		}
		s.appendEvent(taskID, "step", "executing "+step.Type)

		var err error
		if task.Environment == domain.EnvironmentDogzilla {
			err = s.executeDogzillaStep(step)
		} else {
			s.executeSimulatorStep(taskID, step)
		}
		if err != nil {
			s.fail(taskID, err.Error())
			return
		}
		if task.Environment == domain.EnvironmentDogzilla {
			if state, stateErr := s.dogzilla.State(); stateErr == nil {
				s.appendState(taskID, state)
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil && task.Status == domain.TaskRunning {
		task.Status = domain.TaskSucceeded
		task.UpdatedAt = time.Now().UTC()
		s.addEventLocked(task, "succeeded", "task execution completed")
		s.broker.Publish(taskID, task)
		_ = s.store.SaveEpisode(cloneTask(task))
	}
}

func (s *TaskService) executeSimulatorStep(taskID string, step domain.ActionStep) {
	duration := time.Duration(max(step.DurationMS, 300)) * time.Millisecond
	time.Sleep(duration)

	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.tasks[taskID]
	if task == nil {
		return
	}
	if task.Simulator == nil {
		task.Simulator = initialSimulatorState(domain.EnvironmentSimulator, time.Now().UTC())
	}

	state := *task.Simulator
	now := time.Now().UTC()
	switch step.Type {
	case "stand":
		state.Mode = "standing"
	case "move":
		state.Mode = "moving"
		state.YawDeg = normalizeYaw(state.YawDeg + step.YawDeg)
		distance := step.LinearX * float64(max(step.DurationMS, 300)) / 1000
		state.X += distance * cosDeg(state.YawDeg)
		state.Y += distance * sinDeg(state.YawDeg)
	case "stop":
		state.Mode = "stopped"
	case "sit":
		state.Mode = "sitting"
	default:
		state.Mode = step.Type
	}
	state.UpdatedAt = now
	state.Path = append(state.Path, domain.SimulatorPose{
		X:      state.X,
		Y:      state.Y,
		YawDeg: state.YawDeg,
		Time:   now,
	})
	task.Simulator = &state
	task.UpdatedAt = now
	s.broker.Publish(taskID, task)
}

func (s *TaskService) executeDogzillaStep(step domain.ActionStep) error {
	switch step.Type {
	case "stand":
		return s.dogzilla.Stand()
	case "sit":
		return s.dogzilla.Sit()
	case "move":
		return s.dogzilla.Move(step)
	case "stop":
		return s.dogzilla.Stop()
	default:
		return fmt.Errorf("unsupported Dogzilla step: %s", step.Type)
	}
}

func (s *TaskService) currentStatus(taskID string) domain.TaskStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		return task.Status
	}
	return domain.TaskFailed
}

func (s *TaskService) appendEvent(taskID, eventType, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.UpdatedAt = time.Now().UTC()
		s.addEventLocked(task, eventType, message)
		s.broker.Publish(taskID, task)
	}
}

func (s *TaskService) appendState(taskID string, state domain.DogzillaState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.RobotStates = append(task.RobotStates, state)
		task.UpdatedAt = time.Now().UTC()
		s.broker.Publish(taskID, task)
	}
}

func (s *TaskService) fail(taskID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.Status = domain.TaskFailed
		task.FailureReason = reason
		task.UpdatedAt = time.Now().UTC()
		s.addEventLocked(task, "failed", reason)
		s.broker.Publish(taskID, task)
		_ = s.store.SaveEpisode(cloneTask(task))
	}
}

func (s *TaskService) publish(taskID string) {
	if task, ok := s.GetTask(taskID); ok {
		s.broker.Publish(taskID, task)
	}
}

func (s *TaskService) addEventLocked(task *domain.Task, eventType, message string) {
	task.Events = append(task.Events, domain.TaskEvent{
		Time:    time.Now().UTC(),
		Type:    eventType,
		Message: message,
	})
}

func cloneTask(task *domain.Task) *domain.Task {
	copyTask := *task
	copyTask.Events = append([]domain.TaskEvent(nil), task.Events...)
	copyTask.RobotStates = append([]domain.DogzillaState(nil), task.RobotStates...)
	if task.Simulator != nil {
		simulator := *task.Simulator
		simulator.Path = append([]domain.SimulatorPose(nil), task.Simulator.Path...)
		simulator.Obstacles = append([]domain.SimulatorObject(nil), task.Simulator.Obstacles...)
		simulator.Targets = append([]domain.SimulatorObject(nil), task.Simulator.Targets...)
		copyTask.Simulator = &simulator
	}
	if task.Plan != nil {
		plan := *task.Plan
		plan.Steps = append([]domain.ActionStep(nil), task.Plan.Steps...)
		copyTask.Plan = &plan
	}
	return &copyTask
}

func initialSimulatorState(environment domain.Environment, now time.Time) *domain.SimulatorState {
	if environment != domain.EnvironmentSimulator {
		return nil
	}
	return &domain.SimulatorState{
		Mode:   "idle",
		X:      0,
		Y:      0,
		YawDeg: 0,
		Path: []domain.SimulatorPose{{
			X:      0,
			Y:      0,
			YawDeg: 0,
			Time:   now,
		}},
		Targets: []domain.SimulatorObject{
			{ID: "red_block", Label: "赤いブロック", X: 0.42, Y: 0.12, Radius: 0.08},
			{ID: "blue_marker", Label: "青い目印", X: 0.34, Y: -0.24, Radius: 0.07},
		},
		Obstacles: []domain.SimulatorObject{
			{ID: "table_edge", Label: "机の端", X: 0.58, Y: 0, Radius: 0.06},
		},
		UpdatedAt: now,
	}
}

func cosDeg(deg float64) float64 {
	switch normalizeYaw(deg) {
	case 0:
		return 1
	case 90:
		return 0
	case -90:
		return 0
	case 180, -180:
		return -1
	}
	// Small-angle approximation is enough for this educational mock.
	radians := deg * 3.141592653589793 / 180
	return 1 - radians*radians/2
}

func sinDeg(deg float64) float64 {
	radians := deg * 3.141592653589793 / 180
	return radians - radians*radians*radians/6
}

func normalizeYaw(yaw float64) float64 {
	for yaw > 180 {
		yaw -= 360
	}
	for yaw < -180 {
		yaw += 360
	}
	return yaw
}

func estimateFinalX(plan domain.ActionPlan) float64 {
	x := 0.0
	yaw := 0.0
	for _, step := range plan.Steps {
		if step.Type != "move" {
			continue
		}
		yaw = normalizeYaw(yaw + step.YawDeg)
		distance := step.LinearX * float64(max(step.DurationMS, 300)) / 1000
		x += distance * cosDeg(yaw)
	}
	return x
}

func planHasStep(plan domain.ActionPlan, stepType string) bool {
	for _, step := range plan.Steps {
		if step.Type == stepType {
			return true
		}
	}
	return false
}

func evaluationFailureReason(detectedExpected bool, hasStop bool, riskLevel string, finalX float64) string {
	if !detectedExpected {
		return "expected object was not detected"
	}
	if !hasStop {
		return "plan did not include stop step"
	}
	if riskLevel != "low" {
		return "plan risk level was not low"
	}
	if finalX < 0.05 {
		return "simulated movement was too short"
	}
	return "unknown evaluation failure"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
