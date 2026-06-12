package service

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
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
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return tasks
}

func (s *TaskService) ListPersistedEpisodes(limit int) ([]domain.Task, error) {
	return s.store.ListEpisodes(limit)
}

func (s *TaskService) ListPersistedEvaluations(limit int) ([]domain.EvaluationResult, error) {
	return s.store.ListEvaluations(limit)
}

// Planner motion parameters. Yaw follows ROS REP-103: positive = left turn.
const (
	planLinearSpeed   = 0.08 // m/s
	planTurnStepMS    = 600
	planMaxStepMS     = 1800
	planMaxStepYawDeg = 30.0
	planStopClearance = 0.12 // stop this far before the target edge
)

// GeneratePlan parses the Japanese instruction and produces motion steps
// against the known workbench scene, so different instructions actually
// lead to different trajectories. Sequential phrases (〜した後 / 〜してから)
// are planned segment by segment.
func (s *TaskService) GeneratePlan(req domain.PlanRequest) domain.ActionPlan {
	instruction := req.Instruction
	steps := []domain.ActionStep{{Type: "stand"}}
	goals := []string{}
	pose := &planPose{}
	var lastTarget *domain.SimulatorObject

	for _, segment := range splitInstruction(instruction) {
		segmentSteps, segmentGoal, usedTarget := planSegmentWithPose(segment, pose, lastTarget)
		if segmentGoal == "" {
			continue
		}
		steps = append(steps, segmentSteps...)
		goals = append(goals, segmentGoal)
		if usedTarget != nil {
			lastTarget = usedTarget
		}
	}
	if len(goals) == 0 {
		steps = append(steps, forwardSteps(planLinearSpeed*1.2)...)
		goals = append(goals, "move_forward")
	}

	goal := strings.Join(goals, "_then_")
	if wantsStop(instruction) {
		goal += "_and_stop"
	}
	steps = append(steps, domain.ActionStep{Type: "stop"})

	plan := domain.ActionPlan{
		Goal:      goal,
		Steps:     steps,
		RiskLevel: "low",
	}
	trimPlanToSafety(&plan, req.Environment)
	return plan
}

func wantsStop(instruction string) bool {
	lower := strings.ToLower(instruction)
	return strings.Contains(instruction, "止") ||
		strings.Contains(instruction, "ストップ") ||
		strings.Contains(instruction, "停止") ||
		strings.Contains(lower, "stop")
}

func trimPlanToSafety(plan *domain.ActionPlan, env domain.Environment) {
	maxMS := safety.MaxTotalDurationMS
	maxDist := safety.MaxTotalDistanceM
	if env == domain.EnvironmentSimulator {
		// Simulator education demos may chain multiple approach/return legs.
		maxMS = 18000
		maxDist = 1.5
	}
	if len(plan.Steps) == 0 {
		return
	}
	trimmed := []domain.ActionStep{plan.Steps[0]}
	totalMS := 0
	totalDist := 0.0
	for _, step := range plan.Steps[1:] {
		if step.Type == "stop" {
			break
		}
		ms := stepDurationMS(step)
		dist := stepDistanceM(step)
		if totalMS+ms > maxMS || totalDist+dist > maxDist {
			break
		}
		trimmed = append(trimmed, step)
		totalMS += ms
		totalDist += dist
	}
	trimmed = append(trimmed, domain.ActionStep{Type: "stop"})
	plan.Steps = trimmed
}

func stepDurationMS(step domain.ActionStep) int {
	if step.Type != "move" {
		return 300
	}
	if step.DurationMS < safety.MinEffectiveDurationMS {
		return safety.MinEffectiveDurationMS
	}
	return step.DurationMS
}

func stepDistanceM(step domain.ActionStep) float64 {
	if step.Type != "move" {
		return 0
	}
	ms := stepDurationMS(step)
	return math.Abs(step.LinearX) * float64(ms) / 1000
}

// splitInstruction breaks a compound instruction into sequential segments.
func splitInstruction(instruction string) []string {
	normalized := instruction
	for _, separator := range []string{"した後", "したあと", "してから", "それから", "、", ","} {
		normalized = strings.ReplaceAll(normalized, separator, "\n")
	}
	parts := strings.Split(normalized, "\n")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	if len(segments) == 0 {
		return []string{instruction}
	}
	return segments
}

// planPose tracks cumulative robot pose while building a multi-segment plan.
type planPose struct {
	x, y, yaw float64
}

func (p *planPose) applySteps(steps []domain.ActionStep) {
	for _, step := range steps {
		if step.Type != "move" {
			continue
		}
		p.yaw = normalizeYawForPlan(p.yaw + step.YawDeg)
		if step.LinearX != 0 {
			ms := step.DurationMS
			if ms < safety.MinEffectiveDurationMS {
				ms = safety.MinEffectiveDurationMS
			}
			d := step.LinearX * float64(ms) / 1000
			rad := p.yaw * math.Pi / 180
			p.x += d * math.Cos(rad)
			p.y += d * math.Sin(rad)
		}
	}
}

func (p *planPose) stepsToApproachObject(obj domain.SimulatorObject) []domain.ActionStep {
	dx := obj.X - p.x
	dy := obj.Y - p.y
	fullDist := math.Hypot(dx, dy)
	if fullDist < 0.01 {
		return nil
	}
	approachDist := fullDist - obj.Radius - planStopClearance
	if approachDist < 0.01 {
		return nil
	}
	bearing := math.Atan2(dy, dx) * 180 / math.Pi
	yawDelta := normalizeYawForPlan(bearing - p.yaw)
	steps := turnStepsUnlimited(yawDelta)
	steps = append(steps, forwardSteps(approachDist)...)
	p.applySteps(steps)
	return steps
}

func (p *planPose) stepsToFaceObject(obj domain.SimulatorObject) []domain.ActionStep {
	dx := obj.X - p.x
	dy := obj.Y - p.y
	if math.Hypot(dx, dy) < 0.01 {
		return nil
	}
	bearing := math.Atan2(dy, dx) * 180 / math.Pi
	yawDelta := normalizeYawForPlan(bearing - p.yaw)
	steps := turnStepsUnlimited(yawDelta)
	p.applySteps(steps)
	return steps
}

func (p *planPose) stepsToPoint(tx, ty float64) []domain.ActionStep {
	dx := tx - p.x
	dy := ty - p.y
	dist := math.Hypot(dx, dy)
	if dist < 0.01 {
		return nil
	}
	bearing := math.Atan2(dy, dx) * 180 / math.Pi
	yawDelta := normalizeYawForPlan(bearing - p.yaw)
	steps := turnStepsUnlimited(yawDelta)
	steps = append(steps, forwardSteps(dist)...)
	p.applySteps(steps)
	return steps
}

func normalizeYawForPlan(yaw float64) float64 {
	for yaw > 180 {
		yaw -= 360
	}
	for yaw <= -180 {
		yaw += 360
	}
	return yaw
}

func wantsReturn(segment string) bool {
	return containsAny(segment, "戻って", "戻る", "返って", "返る", "戻れ", "戻し", "に戻")
}

// planSegmentWithPose plans one instruction segment from the current pose.
// lastTarget carries the most recent scene object for follow-on clauses like
// "近くまで移動して".
func planSegmentWithPose(segment string, pose *planPose, lastTarget *domain.SimulatorObject) ([]domain.ActionStep, string, *domain.SimulatorObject) {
	target := matchSceneTarget(segment)
	wantsMove := wantsMovement(segment)
	turnOnly := strings.Contains(segment, "向") && !wantsMove

	if wantsReturn(segment) {
		if target != nil {
			steps := pose.stepsToApproachObject(*target)
			return steps, "return_to_" + target.ID, target
		}
		steps := pose.stepsToPoint(0, 0)
		return steps, "return_to_origin", nil
	}

	switch {
	case target != nil:
		if turnOnly {
			steps := pose.stepsToFaceObject(*target)
			return steps, "face_" + target.ID, target
		}
		steps := pose.stepsToApproachObject(*target)
		return steps, "approach_" + target.ID, target

	case lastTarget != nil && wantsMove && containsAny(segment, "近"):
		steps := pose.stepsToApproachObject(*lastTarget)
		return steps, "approach_" + lastTarget.ID, lastTarget

	case containsAny(segment, "後ろ", "下が", "バック", "ばっく"):
		if strings.Contains(segment, "向") {
			steps := turnStepsUnlimited(180)
			pose.applySteps(steps)
			return steps, "turn_around", nil
		}
		steps := backwardSteps(parseStepCount(segment))
		pose.applySteps(steps)
		return steps, "move_backward", nil

	case strings.Contains(segment, "右"):
		steps := []domain.ActionStep{{Type: "move", YawDeg: -15, DurationMS: planTurnStepMS}}
		if wantsMove {
			steps = append(steps, forwardSteps(planLinearSpeed*float64(parseStepCount(segment))*0.15)...)
		}
		pose.applySteps(steps)
		return steps, "turn_right", nil

	case strings.Contains(segment, "左"):
		steps := []domain.ActionStep{{Type: "move", YawDeg: 15, DurationMS: planTurnStepMS}}
		if wantsMove {
			steps = append(steps, forwardSteps(planLinearSpeed*float64(parseStepCount(segment))*0.15)...)
		}
		pose.applySteps(steps)
		return steps, "turn_left", nil

	case containsAny(segment, "一回転", "回転", "回って"):
		steps := turnStepsUnlimited(360)
		pose.applySteps(steps)
		return steps, "spin_around", nil

	case wantsMove || containsAny(segment, "前", "まっすぐ"):
		steps := forwardSteps(planLinearSpeed * float64(parseStepCount(segment)) * 0.15)
		pose.applySteps(steps)
		return steps, "move_forward", nil
	}
	return nil, "", nil
}

var stepCountPattern = regexp.MustCompile(`(\d+)\s*歩`)

func parseStepCount(segment string) int {
	match := stepCountPattern.FindStringSubmatch(segment)
	if len(match) < 2 {
		return 1
	}
	count, err := strconv.Atoi(match[1])
	if err != nil || count < 1 {
		return 1
	}
	if count > 10 {
		return 10
	}
	return count
}

func wantsMovement(segment string) bool {
	if containsAny(segment, "移動", "進", "行って", "向かって", "近づ", "近くまで", "近くに") {
		return true
	}
	// Bare "近" only counts when paired with motion intent.
	return strings.Contains(segment, "近") && containsAny(segment, "まで", "に", "して")
}

func backwardSteps(count int) []domain.ActionStep {
	steps := make([]domain.ActionStep, 0, count)
	for i := 0; i < count; i++ {
		steps = append(steps, domain.ActionStep{Type: "move", LinearX: -0.05, DurationMS: 300})
	}
	return steps
}

// matchSceneTarget maps color/landmark words to known workbench objects.
func matchSceneTarget(instruction string) *domain.SimulatorObject {
	lower := strings.ToLower(instruction)
	candidates := append(defaultSceneTargets(), defaultSceneObstacles()...)
	switch {
	case strings.Contains(instruction, "赤") || strings.Contains(lower, "red"):
		return findSceneObject(candidates, "red_block")
	case strings.Contains(instruction, "青") || strings.Contains(lower, "blue"):
		return findSceneObject(candidates, "blue_marker")
	case strings.Contains(instruction, "机") || strings.Contains(instruction, "テーブル"):
		return findSceneObject(candidates, "table_edge")
	}
	return nil
}

func findSceneObject(objects []domain.SimulatorObject, id string) *domain.SimulatorObject {
	for i := range objects {
		if objects[i].ID == id {
			return &objects[i]
		}
	}
	return nil
}

func containsAny(text string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

// turnStepsUnlimited splits a turn into chunks; trimPlanToSafety caps the full mission.
func turnStepsUnlimited(yawDeg float64) []domain.ActionStep {
	steps := []domain.ActionStep{}
	remaining := yawDeg
	for math.Abs(remaining) > 3 {
		chunk := remaining
		if chunk > planMaxStepYawDeg {
			chunk = planMaxStepYawDeg
		}
		if chunk < -planMaxStepYawDeg {
			chunk = -planMaxStepYawDeg
		}
		steps = append(steps, domain.ActionStep{
			Type:       "move",
			YawDeg:     math.Round(chunk*10) / 10,
			DurationMS: planTurnStepMS,
		})
		remaining -= chunk
	}
	return steps
}

// turnSteps splits a turn into chunks that respect per-step and total-duration limits.
func turnSteps(yawDeg float64) []domain.ActionStep {
	steps := []domain.ActionStep{}
	remaining := yawDeg
	totalMS := 0
	for math.Abs(remaining) > 3 && totalMS+planTurnStepMS <= safety.MaxTotalDurationMS {
		chunk := remaining
		if chunk > planMaxStepYawDeg {
			chunk = planMaxStepYawDeg
		}
		if chunk < -planMaxStepYawDeg {
			chunk = -planMaxStepYawDeg
		}
		steps = append(steps, domain.ActionStep{
			Type:       "move",
			YawDeg:     math.Round(chunk*10) / 10,
			DurationMS: planTurnStepMS,
		})
		totalMS += planTurnStepMS
		remaining -= chunk
	}
	return steps
}

// forwardSteps splits a straight run into chunks that respect the per-step
// duration limit.
func forwardSteps(distance float64) []domain.ActionStep {
	steps := []domain.ActionStep{}
	remaining := distance
	maxChunk := planLinearSpeed * float64(planMaxStepMS) / 1000
	for remaining > 0.005 {
		chunk := math.Min(remaining, maxChunk)
		steps = append(steps, domain.ActionStep{
			Type:       "move",
			LinearX:    planLinearSpeed,
			DurationMS: int(math.Round(chunk / planLinearSpeed * 1000)),
		})
		remaining -= chunk
	}
	return steps
}

// SourceDogzillaCamera asks the backend to grab the latest robot camera
// frame and run perception on it instead of an uploaded/sample image.
const SourceDogzillaCamera = "dogzilla_camera"

func (s *TaskService) RunPerception(req domain.PerceptionRequest) domain.PerceptionResult {
	if req.Source == SourceDogzillaCamera && req.ImageBase64 == "" {
		frame, err := s.dogzilla.CameraFrame()
		if err != nil {
			return domain.PerceptionResult{
				Source:  req.Source,
				Summary: "dogzilla camera frame error: " + err.Error(),
			}
		}
		req.ImageBase64 = frame.ImageBase64
	}

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
			MaxLinearX:             safety.MaxLinearX,
			MaxLinearY:             safety.MaxLinearY,
			MaxYawDeg:              safety.MaxYawDeg,
			MaxDurationMS:          safety.MaxStepDurationMS,
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
		expectMove     bool
	}{
		{id: "case_001", instruction: "赤いブロックの近くまで移動して止まって", expectedObject: "red_block", expectMove: true},
		{id: "case_002", instruction: "左の赤いブロックの方向を向いて止まって", expectedObject: "red_block", expectMove: false},
		{id: "case_003", instruction: "青い目印の方向に少し進んで止まって", expectedObject: "blue_marker", expectMove: true},
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
		hasTurn := planHasTurn(plan)
		motionOK := finalX >= 0.05
		if !scenario.expectMove {
			motionOK = hasTurn
		}
		casePassed := detectedExpected && hasStop && plan.RiskLevel == "low" && motionOK
		failureReason := ""
		if !casePassed {
			failureReason = evaluationFailureReason(detectedExpected, hasStop, plan.RiskLevel, scenario.expectMove, finalX, hasTurn)
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
	if task.Status == domain.TaskRunning {
		s.mu.Unlock()
		return nil, errors.New("task is already running")
	}
	now := time.Now().UTC()
	if task.Environment == domain.EnvironmentSimulator {
		task.Simulator = initialSimulatorState(task.Environment, now)
	}
	task.Status = domain.TaskRunning
	task.UpdatedAt = now
	s.addEventLocked(task, "running", "task execution started")
	s.mu.Unlock()
	s.publish(taskID)

	go s.runTask(taskID)

	updated, _ := s.GetTask(taskID)
	return updated, nil
}

func (s *TaskService) Stop(taskID string) (*domain.Task, error) {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return nil, errors.New("task not found")
	}
	environment := task.Environment
	s.mu.Unlock()

	// Only the real robot needs a hardware stop; simulator tasks stop in-process.
	if environment == domain.EnvironmentDogzilla {
		if err := s.dogzilla.Stop(); err != nil {
			s.appendEvent(taskID, "stop_error", err.Error())
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok = s.tasks[taskID]
	if !ok {
		return nil, errors.New("task not found")
	}
	task.Status = domain.TaskStopped
	task.UpdatedAt = time.Now().UTC()
	s.addEventLocked(task, "stopped", "stop requested")
	s.broker.Publish(taskID, cloneTask(task))
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
		s.broker.Publish(taskID, cloneTask(task))
		_ = s.store.SaveEpisode(cloneTask(task))
	}
}

// simulatorTickMS controls how often intermediate poses are published while
// a move step runs, so the UI can animate continuous motion.
const simulatorTickMS = 100

func (s *TaskService) executeSimulatorStep(taskID string, step domain.ActionStep) {
	durationMS := max(step.DurationMS, 300)

	if step.Type != "move" {
		time.Sleep(time.Duration(durationMS) * time.Millisecond)
		s.applySimulatorPose(taskID, step.Type, 0, 0)
		return
	}

	ticks := max(durationMS/simulatorTickMS, 1)
	yawPerTick := step.YawDeg / float64(ticks)
	distancePerTick := step.LinearX * float64(durationMS) / 1000 / float64(ticks)
	for i := 0; i < ticks; i++ {
		if s.currentStatus(taskID) != domain.TaskRunning {
			return
		}
		time.Sleep(simulatorTickMS * time.Millisecond)
		s.applySimulatorPose(taskID, "move", yawPerTick, distancePerTick)
	}
}

func (s *TaskService) applySimulatorPose(taskID, stepType string, yawDelta, distance float64) {
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
	switch stepType {
	case "stand":
		state.Mode = "standing"
	case "move":
		state.Mode = "moving"
		state.YawDeg = normalizeYaw(state.YawDeg + yawDelta)
		state.X += distance * cosDeg(state.YawDeg)
		state.Y += distance * sinDeg(state.YawDeg)
	case "stop":
		state.Mode = "stopped"
	case "sit":
		state.Mode = "sitting"
	default:
		state.Mode = stepType
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
	s.broker.Publish(taskID, cloneTask(task))
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
		s.broker.Publish(taskID, cloneTask(task))
	}
}

func (s *TaskService) appendState(taskID string, state domain.DogzillaState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task := s.tasks[taskID]; task != nil {
		task.RobotStates = append(task.RobotStates, state)
		task.UpdatedAt = time.Now().UTC()
		s.broker.Publish(taskID, cloneTask(task))
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
		s.broker.Publish(taskID, cloneTask(task))
		_ = s.store.SaveEpisode(cloneTask(task))
	}
}

func (s *TaskService) publish(taskID string) {
	if task, ok := s.GetTask(taskID); ok {
		s.broker.Publish(taskID, cloneTask(task))
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
		Targets:   defaultSceneTargets(),
		Obstacles: defaultSceneObstacles(),
		UpdatedAt: now,
	}
}

// The shared workbench scene: the planner and the simulator must agree on
// object positions so instructions resolve to correct trajectories.
func defaultSceneTargets() []domain.SimulatorObject {
	return []domain.SimulatorObject{
		{ID: "red_block", Label: "赤いブロック", X: 0.42, Y: 0.12, Radius: 0.08},
		{ID: "blue_marker", Label: "青い目印", X: 0.34, Y: -0.24, Radius: 0.07},
	}
}

func defaultSceneObstacles() []domain.SimulatorObject {
	return []domain.SimulatorObject{
		{ID: "table_edge", Label: "机の端", X: 0.58, Y: 0, Radius: 0.06},
	}
}

func cosDeg(deg float64) float64 {
	return math.Cos(deg * math.Pi / 180)
}

func sinDeg(deg float64) float64 {
	return math.Sin(deg * math.Pi / 180)
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

func planHasTurn(plan domain.ActionPlan) bool {
	for _, step := range plan.Steps {
		if step.Type == "move" && step.YawDeg != 0 {
			return true
		}
	}
	return false
}

func evaluationFailureReason(detectedExpected, hasStop bool, riskLevel string, expectMove bool, finalX float64, hasTurn bool) string {
	if !detectedExpected {
		return "expected object was not detected"
	}
	if !hasStop {
		return "plan did not include stop step"
	}
	if riskLevel != "low" {
		return "plan risk level was not low"
	}
	if expectMove && finalX < 0.05 {
		return "simulated movement was too short"
	}
	if !expectMove && !hasTurn {
		return "plan did not include a turn toward the target"
	}
	return "unknown evaluation failure"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
