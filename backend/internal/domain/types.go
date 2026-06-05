package domain

import "time"

type Environment string

const (
	EnvironmentSimulator Environment = "simulator"
	EnvironmentDogzilla  Environment = "dogzilla"
)

type TaskStatus string

const (
	TaskQueued    TaskStatus = "queued"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskFailed    TaskStatus = "failed"
	TaskStopped   TaskStatus = "stopped"
)

type Task struct {
	ID            string          `json:"id"`
	Instruction   string          `json:"instruction"`
	Environment   Environment     `json:"environment"`
	Status        TaskStatus      `json:"status"`
	Plan          *ActionPlan     `json:"plan,omitempty"`
	Simulator     *SimulatorState `json:"simulator_state,omitempty"`
	Events        []TaskEvent     `json:"events"`
	RobotStates   []DogzillaState `json:"robot_states,omitempty"`
	FailureReason string          `json:"failure_reason,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type TaskEvent struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
}

type CreateTaskRequest struct {
	Instruction string      `json:"instruction"`
	Environment Environment `json:"environment"`
}

type PlanRequest struct {
	Instruction string      `json:"instruction"`
	Environment Environment `json:"environment"`
}

type PerceptionRequest struct {
	Source      string `json:"source"`
	Instruction string `json:"instruction,omitempty"`
}

type PerceptionResult struct {
	Source    string           `json:"source"`
	ImageSize ImageSize        `json:"image_size"`
	Objects   []DetectedObject `json:"objects"`
	Summary   string           `json:"summary"`
}

type PerceptionServiceStatus struct {
	Connected   bool      `json:"connected"`
	ServiceURL  string    `json:"service_url"`
	Error       string    `json:"error,omitempty"`
	LastChecked time.Time `json:"last_checked"`
}

type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type DetectedObject struct {
	Label        string    `json:"label"`
	DisplayName  string    `json:"display_name"`
	Confidence   float64   `json:"confidence"`
	BBox         []float64 `json:"bbox"`
	PositionHint string    `json:"position_hint"`
}

type ExecuteActionRequest struct {
	TaskID string `json:"task_id"`
}

type StopActionRequest struct {
	TaskID string `json:"task_id"`
}

type EvaluationRequest struct {
	Suite string `json:"suite"`
}

type EvaluationResult struct {
	ID        string            `json:"id"`
	Suite     string            `json:"suite"`
	Summary   EvaluationSummary `json:"summary"`
	Cases     []EvaluationCase  `json:"cases"`
	CreatedAt time.Time         `json:"created_at"`
}

type EvaluationSummary struct {
	TotalCases        int     `json:"total_cases"`
	PassedCases       int     `json:"passed_cases"`
	SuccessRate       float64 `json:"success_rate"`
	AverageConfidence float64 `json:"average_confidence"`
	AverageFinalX     float64 `json:"average_final_x"`
}

type EvaluationCase struct {
	ID                   string   `json:"id"`
	Instruction          string   `json:"instruction"`
	ExpectedObject       string   `json:"expected_object"`
	DetectedObjectLabels []string `json:"detected_object_labels"`
	PlanGoal             string   `json:"plan_goal"`
	FinalX               float64  `json:"final_x"`
	Passed               bool     `json:"passed"`
	FailureReason        string   `json:"failure_reason,omitempty"`
}

type ActionPlan struct {
	Goal      string       `json:"goal"`
	Steps     []ActionStep `json:"steps"`
	RiskLevel string       `json:"risk_level"`
}

type ActionStep struct {
	Type       string  `json:"type"`
	LinearX    float64 `json:"linear_x,omitempty"`
	LinearY    float64 `json:"linear_y,omitempty"`
	YawDeg     float64 `json:"yaw_deg,omitempty"`
	DurationMS int     `json:"duration_ms,omitempty"`
}

type SimulatorState struct {
	Mode      string            `json:"mode"`
	X         float64           `json:"x"`
	Y         float64           `json:"y"`
	YawDeg    float64           `json:"yaw_deg"`
	Path      []SimulatorPose   `json:"path"`
	Obstacles []SimulatorObject `json:"obstacles"`
	Targets   []SimulatorObject `json:"targets"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type SimulatorPose struct {
	X      float64   `json:"x"`
	Y      float64   `json:"y"`
	YawDeg float64   `json:"yaw_deg"`
	Time   time.Time `json:"time"`
}

type SimulatorObject struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
}

type DogzillaState struct {
	Mode        string    `json:"mode"`
	Battery     float64   `json:"battery"`
	Roll        float64   `json:"roll"`
	Pitch       float64   `json:"pitch"`
	Yaw         float64   `json:"yaw"`
	ServoAngles []float64 `json:"servo_angles"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DogzillaRuntimeStatus struct {
	Connected   bool           `json:"connected"`
	RuntimeURL  string         `json:"runtime_url"`
	State       DogzillaState  `json:"state,omitempty"`
	Safety      DogzillaSafety `json:"safety"`
	Error       string         `json:"error,omitempty"`
	LastChecked time.Time      `json:"last_checked"`
}

type DogzillaSafety struct {
	EmergencyStopAvailable bool    `json:"emergency_stop_available"`
	MaxLinearX             float64 `json:"max_linear_x"`
	MaxLinearY             float64 `json:"max_linear_y"`
	MaxYawDeg              float64 `json:"max_yaw_deg"`
	MaxDurationMS          int     `json:"max_duration_ms"`
}
