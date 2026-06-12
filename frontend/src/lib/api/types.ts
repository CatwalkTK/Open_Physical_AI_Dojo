export type Environment = "simulator" | "dogzilla";

export type TaskStatus = "queued" | "running" | "succeeded" | "failed" | "stopped";

export type ActionStep = {
  type: string;
  linear_x?: number;
  linear_y?: number;
  yaw_deg?: number;
  duration_ms?: number;
};

export type ActionPlan = {
  goal: string;
  steps: ActionStep[];
  risk_level: string;
};

export type TaskEvent = {
  time: string;
  type: string;
  message: string;
};

export type Task = {
  id: string;
  instruction: string;
  environment: Environment;
  status: TaskStatus;
  plan: ActionPlan;
  simulator_state?: SimulatorState;
  events: TaskEvent[];
  failure_reason?: string;
  created_at: string;
  updated_at: string;
};

export type SimulatorState = {
  mode: string;
  x: number;
  y: number;
  yaw_deg: number;
  path: SimulatorPose[];
  obstacles: SimulatorObject[];
  targets: SimulatorObject[];
  updated_at: string;
};

export type SimulatorPose = {
  x: number;
  y: number;
  yaw_deg: number;
  time: string;
};

export type SimulatorObject = {
  id: string;
  label: string;
  x: number;
  y: number;
  radius: number;
};

export type PerceptionResult = {
  source: string;
  image_size: {
    width: number;
    height: number;
  };
  objects: DetectedObject[];
  image_base64?: string;
  summary: string;
};

export type PerceptionServiceStatus = {
  connected: boolean;
  service_url: string;
  error?: string;
  last_checked: string;
};

export type DetectedObject = {
  label: string;
  display_name: string;
  confidence: number;
  bbox: [number, number, number, number];
  position_hint: string;
};

export type DogzillaRuntimeStatus = {
  connected: boolean;
  runtime_url: string;
  state?: DogzillaState;
  safety: DogzillaSafety;
  error?: string;
  last_checked: string;
};

export type DogzillaState = {
  mode: string;
  battery: number;
  roll: number;
  pitch: number;
  yaw: number;
  servo_angles: number[];
  updated_at: string;
};

export type DogzillaSafety = {
  emergency_stop_available: boolean;
  max_linear_x: number;
  max_linear_y: number;
  max_yaw_deg: number;
  max_duration_ms: number;
};

export type LessonWithStatus = {
  id: string;
  order: number;
  title: string;
  description: string;
  goal: string;
  steps: string[];
  completed: boolean;
  completed_at?: string;
};

export type EvaluationResult = {
  id: string;
  suite: string;
  summary: EvaluationSummary;
  cases: EvaluationCase[];
  created_at: string;
};

export type EvaluationSummary = {
  total_cases: number;
  passed_cases: number;
  success_rate: number;
  average_confidence: number;
  average_final_x: number;
};

export type EvaluationCase = {
  id: string;
  instruction: string;
  expected_object: string;
  detected_object_labels: string[];
  plan_goal: string;
  final_x: number;
  passed: boolean;
  failure_reason?: string;
};
