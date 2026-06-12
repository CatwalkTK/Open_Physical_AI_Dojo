import React from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  BarChart3,
  Bot,
  CheckCircle2,
  Circle,
  Clock,
  Eye,
  Map,
  Play,
  RefreshCw,
  ShieldAlert,
  Square,
  Waypoints,
} from "lucide-react";
import {
  createTask,
  emergencyStopDogzilla,
  executeTask,
  getDogzillaStatus,
  getPerceptionStatus,
  listEpisodes,
  listEvaluations,
  runEvaluation,
  runPerception,
  stopTask,
  subscribeTask,
} from "./lib/api/client";
import type {
  DetectedObject,
  DogzillaRuntimeStatus,
  Environment,
  EvaluationResult,
  PerceptionResult,
  PerceptionServiceStatus,
  SimulatorState,
  Task,
} from "./lib/api/types";
import { Simulator3D } from "./components/Simulator3D";
import "./styles.css";

function App() {
  const [instruction, setInstruction] = React.useState("赤いブロックの近くまで移動して止まって");
  const [environment, setEnvironment] = React.useState<Environment>("simulator");
  const [task, setTask] = React.useState<Task | null>(null);
  const [perception, setPerception] = React.useState<PerceptionResult | null>(null);
  const [perceptionStatus, setPerceptionStatus] = React.useState<PerceptionServiceStatus | null>(null);
  const [dogzillaStatus, setDogzillaStatus] = React.useState<DogzillaRuntimeStatus | null>(null);
  const [evaluation, setEvaluation] = React.useState<EvaluationResult | null>(null);
  const [episodeHistory, setEpisodeHistory] = React.useState<Task[]>([]);
  const [evaluationHistory, setEvaluationHistory] = React.useState<EvaluationResult[]>([]);
  const [error, setError] = React.useState("");
  const [uploadedImage, setUploadedImage] = React.useState<{ name: string; base64: string } | null>(null);
  const [isBusy, setIsBusy] = React.useState(false);
  const [isVisionBusy, setIsVisionBusy] = React.useState(false);
  const [isPerceptionStatusBusy, setIsPerceptionStatusBusy] = React.useState(false);
  const [isRobotBusy, setIsRobotBusy] = React.useState(false);
  const [isEvaluationBusy, setIsEvaluationBusy] = React.useState(false);

  React.useEffect(() => {
    if (!task?.id) return;
    const close = subscribeTask(task.id, (nextTask) => {
      setTask(nextTask);
    });
    return close;
  }, [task?.id]);

  React.useEffect(() => {
    void handleDogzillaStatus();
    void handlePerceptionStatus();
    void handleRefreshHistory();
  }, []);

  async function handleCreate() {
    setError("");
    setIsBusy(true);
    try {
      const nextTask = await createTask({ instruction, environment });
      setTask(nextTask);
    } catch (err) {
      setError(err instanceof Error ? err.message : "タスク作成に失敗しました");
    } finally {
      setIsBusy(false);
    }
  }

  async function handlePerception() {
    setError("");
    setIsVisionBusy(true);
    try {
      const result = await runPerception({
        source: uploadedImage ? `upload:${uploadedImage.name}` : "sample_workbench",
        instruction,
        image_base64: uploadedImage?.base64,
      });
      setPerception(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "画像認識に失敗しました");
    } finally {
      setIsVisionBusy(false);
    }
  }

  async function handleDogzillaCameraPerception() {
    setError("");
    setIsVisionBusy(true);
    try {
      const result = await runPerception({ source: "dogzilla_camera", instruction });
      setPerception(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Dogzillaカメラ認識に失敗しました");
    } finally {
      setIsVisionBusy(false);
    }
  }

  function handleImageSelected(file: File | null) {
    if (!file) {
      setUploadedImage(null);
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const dataUrl = String(reader.result ?? "");
      const base64 = dataUrl.includes(",") ? dataUrl.slice(dataUrl.indexOf(",") + 1) : dataUrl;
      setUploadedImage({ name: file.name, base64 });
    };
    reader.readAsDataURL(file);
  }

  async function handlePerceptionStatus() {
    setIsPerceptionStatusBusy(true);
    try {
      const status = await getPerceptionStatus();
      setPerceptionStatus(status);
    } catch (err) {
      setPerceptionStatus({
        connected: false,
        service_url: "http://localhost:8070",
        error: err instanceof Error ? err.message : "Perception Serviceに接続できません",
        last_checked: new Date().toISOString(),
      });
    } finally {
      setIsPerceptionStatusBusy(false);
    }
  }

  async function handleExecute() {
    if (!task) return;
    setError("");
    setIsBusy(true);
    try {
      const nextTask = await executeTask(task.id);
      setTask(nextTask);
      window.setTimeout(() => void handleRefreshHistory(), 2200);
    } catch (err) {
      setError(err instanceof Error ? err.message : "実行に失敗しました");
    } finally {
      setIsBusy(false);
    }
  }

  async function handleDogzillaStatus() {
    setIsRobotBusy(true);
    try {
      const status = await getDogzillaStatus();
      setDogzillaStatus(status);
    } catch (err) {
      setDogzillaStatus({
        connected: false,
        runtime_url: "http://localhost:8090",
        error: err instanceof Error ? err.message : "Dogzilla Runtimeに接続できません",
        last_checked: new Date().toISOString(),
        safety: {
          emergency_stop_available: true,
          max_linear_x: 0.12,
          max_linear_y: 0.08,
          max_yaw_deg: 30,
          max_duration_ms: 1800,
        },
      });
    } finally {
      setIsRobotBusy(false);
    }
  }

  async function handleDogzillaEmergencyStop() {
    setIsRobotBusy(true);
    setError("");
    try {
      const status = await emergencyStopDogzilla();
      setDogzillaStatus(status);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Dogzilla停止に失敗しました");
      await handleDogzillaStatus();
    } finally {
      setIsRobotBusy(false);
    }
  }

  async function handleRunEvaluation() {
    setError("");
    setIsEvaluationBusy(true);
    try {
      const result = await runEvaluation({ suite: "simulator_basic" });
      setEvaluation(result);
      await handleRefreshHistory();
    } catch (err) {
      setError(err instanceof Error ? err.message : "評価ベンチマークに失敗しました");
    } finally {
      setIsEvaluationBusy(false);
    }
  }

  async function handleStop() {
    if (!task) return;
    setError("");
    try {
      const nextTask = await stopTask(task.id);
      setTask(nextTask);
    } catch (err) {
      setError(err instanceof Error ? err.message : "停止に失敗しました");
    }
  }

  async function handleRefreshHistory() {
    const [episodes, evaluations] = await Promise.all([listEpisodes(), listEvaluations()]);
    setEpisodeHistory(episodes);
    setEvaluationHistory(evaluations);
  }

  const trimmedInstruction = instruction.trim();
  const isPlanStale =
    !task || task.instruction.trim() !== trimmedInstruction || task.environment !== environment;
  const canRun = !!task && task.status !== "running" && !isPlanStale && !isBusy;

  return (
    <main className="shell">
      <section className="topbar">
        <div>
          <h1>Open Physical AI Dojo</h1>
          <p>Dogzilla VLA Task Runner</p>
        </div>
        <StatusBadge status={task?.status ?? "idle"} />
      </section>

      <section className="phase-panel" aria-label="development phase">
        <div className="phase-summary">
          <span className="phase-kicker">Current Development Stage</span>
          <h2>Phase 7: Perception Service Integration</h2>
          <p>画像認識をGin内のmockからPython Perception Serviceへ分離し、モデル差し替えの境界を作る段階です。</p>
        </div>
        <div className="phase-steps">
          <PhaseStep status="done" label="Phase 0" description="Gin / React / Dogzilla mock基盤" />
          <PhaseStep status="done" label="Phase 1" description="Task Runnerと実行ログ" />
          <PhaseStep status="done" label="Phase 2" description="画像認識とVision Viewer" />
          <PhaseStep status="done" label="Phase 3" description="Simulator強化と軌跡表示" />
          <PhaseStep status="done" label="Phase 4" description="Dogzilla状態確認と停止系統" />
          <PhaseStep status="done" label="Phase 5" description="評価ベンチマーク" />
          <PhaseStep status="done" label="Phase 6" description="履歴保存と再表示" />
          <PhaseStep status="active" label="Phase 7" description="Perception Service連携" />
        </div>
        <div className="capability-grid">
          <Capability title="今できること" items={["Perception Service接続確認", "外部サービス経由でVision実行", "モデル差し替え境界を維持"]} />
          <Capability title="まだ未実装" items={["本物のYOLO/SAM接続", "実機ROS2ノード接続", "SQLite/Postgres移行"]} />
        </div>
      </section>

      <section className="workspace">
        <div className="panel task-panel">
          <div className="panel-title">
            <Bot size={20} />
            <h2>Task Runner</h2>
          </div>

          <label className="field">
            <span>日本語指示</span>
            <textarea value={instruction} onChange={(event) => setInstruction(event.target.value)} rows={5} />
          </label>

          <div className="field">
            <span>実行環境</span>
            <div className="segment">
              <button className={environment === "simulator" ? "active" : ""} onClick={() => setEnvironment("simulator")}>
                Simulator
              </button>
              <button className={environment === "dogzilla" ? "active" : ""} onClick={() => setEnvironment("dogzilla")}>
                Dogzilla
              </button>
            </div>
          </div>

          <div className="actions">
            <button onClick={handlePerception} disabled={isVisionBusy}>
              <Eye size={18} />
              Vision
            </button>
            <button className="primary" onClick={handleCreate} disabled={isBusy || !instruction.trim()}>
              <Waypoints size={18} />
              Plan
            </button>
            <button onClick={handleExecute} disabled={!canRun}>
              <Play size={18} />
              Run
            </button>
            <button className="danger" onClick={handleStop} disabled={!task}>
              <Square size={18} />
              Stop
            </button>
          </div>

          {!task && trimmedInstruction && (
            <p className="plan-hint">Planを押すと、この指示から行動計画を作成します。</p>
          )}
          {isPlanStale && task && (
            <p className="plan-stale-hint">
              指示または実行環境が変わっています。Runの前に必ずPlanを押してください。
            </p>
          )}
          {task && !isPlanStale && task.status === "succeeded" && (
            <p className="plan-hint">計画は最新です。Runで原点から再実行できます。</p>
          )}

          {error && <p className="error">{error}</p>}
        </div>

        <div className="panel simulator-panel">
          <div className="panel-title">
            <Map size={20} />
            <h2>Simulator Viewer</h2>
          </div>
          <SimulatorViewer state={task?.simulator_state} environment={task?.environment ?? environment} />
        </div>

        <div className="panel vision-panel">
          <div className="panel-title">
            <Eye size={20} />
            <h2>Vision Viewer</h2>
          </div>
          <VisionViewer
            result={perception}
            status={perceptionStatus}
            isStatusBusy={isPerceptionStatusBusy}
            isVisionBusy={isVisionBusy}
            uploadedImageName={uploadedImage?.name ?? null}
            onRefreshStatus={handlePerceptionStatus}
            onImageSelected={handleImageSelected}
            onRunDogzillaCamera={handleDogzillaCameraPerception}
          />
        </div>

        <div className="panel">
          <div className="panel-title">
            <Waypoints size={20} />
            <h2>Plan</h2>
          </div>
          {task?.plan ? (
            <div className="plan">
              <div className="meta-grid">
                <span>Instruction</span>
                <strong>{task.instruction}</strong>
                <span>Task</span>
                <strong>{task.id}</strong>
                <span>Goal</span>
                <strong>{task.plan.goal}</strong>
                <span>Risk</span>
                <strong>{task.plan.risk_level}</strong>
                <span>Environment</span>
                <strong>{task.environment}</strong>
              </div>
              <ol>
                {task.plan.steps.map((step, index) => (
                  <li key={`${step.type}-${index}`}>
                    <strong>{step.type}</strong>
                    <span>{formatStep(step)}</span>
                  </li>
                ))}
              </ol>
            </div>
          ) : (
            <EmptyState text="計画はまだありません" />
          )}
        </div>

        <div className="panel dogzilla-panel">
          <div className="panel-title split-title">
            <div>
              <ShieldAlert size={20} />
              <h2>Dogzilla Status</h2>
            </div>
            <button className="icon-button" onClick={handleDogzillaStatus} disabled={isRobotBusy} title="Refresh Dogzilla status">
              <RefreshCw size={18} />
            </button>
          </div>
          <DogzillaStatusViewer
            status={dogzillaStatus}
            isBusy={isRobotBusy}
            onRefresh={handleDogzillaStatus}
            onEmergencyStop={handleDogzillaEmergencyStop}
          />
        </div>

        <div className="panel benchmark-panel">
          <div className="panel-title split-title">
            <div>
              <BarChart3 size={20} />
              <h2>Benchmark</h2>
            </div>
            <button onClick={handleRunEvaluation} disabled={isEvaluationBusy}>
              <BarChart3 size={18} />
              Run
            </button>
          </div>
          <BenchmarkViewer result={evaluation} isBusy={isEvaluationBusy} />
        </div>

        <div className="panel history-panel">
          <div className="panel-title split-title">
            <div>
              <Clock size={20} />
              <h2>History</h2>
            </div>
            <button onClick={handleRefreshHistory}>
              <RefreshCw size={18} />
              Refresh
            </button>
          </div>
          <HistoryViewer episodes={episodeHistory} evaluations={evaluationHistory} />
        </div>

        <div className="panel monitor">
          <div className="panel-title">
            <Activity size={20} />
            <h2>Execution Monitor</h2>
          </div>
          {task ? (
            <>
              <div className="meta-grid">
                <span>Task</span>
                <strong>{task.id}</strong>
                <span>Status</span>
                <strong>{task.status}</strong>
                <span>Updated</span>
                <strong>{new Date(task.updated_at).toLocaleTimeString()}</strong>
              </div>
              <div className="events">
                {task.events.slice().reverse().map((event, index) => (
                  <div className="event" key={`${event.time}-${index}`}>
                    <time>{new Date(event.time).toLocaleTimeString()}</time>
                    <span>{event.type}</span>
                    <p>{event.message}</p>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <EmptyState text="タスクを作成するとログが表示されます" />
          )}
        </div>
      </section>
    </main>
  );
}

function HistoryViewer({ episodes, evaluations }: { episodes: Task[]; evaluations: EvaluationResult[] }) {
  return (
    <div className="history-grid">
      <div>
        <h3>Episodes</h3>
        {episodes.length > 0 ? (
          <div className="history-list">
            {episodes.slice(0, 5).map((episode) => (
              <div className="history-item" key={`${episode.id}-${episode.updated_at}`}>
                <strong>{episode.id}</strong>
                <span>{episode.status} / {episode.environment}</span>
                <p>{episode.instruction}</p>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState text="完了したEpisodeはまだありません" />
        )}
      </div>
      <div>
        <h3>Evaluations</h3>
        {evaluations.length > 0 ? (
          <div className="history-list">
            {evaluations.slice(0, 5).map((result) => (
              <div className="history-item" key={result.id}>
                <strong>{result.id}</strong>
                <span>{Math.round(result.summary.success_rate * 100)}% / {result.summary.passed_cases}/{result.summary.total_cases}</span>
                <p>{result.suite}</p>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState text="評価履歴はまだありません" />
        )}
      </div>
    </div>
  );
}

function BenchmarkViewer({ result, isBusy }: { result: EvaluationResult | null; isBusy: boolean }) {
  if (!result) {
    return <EmptyState text={isBusy ? "評価を実行しています" : "Runを押すと評価結果が表示されます"} />;
  }

  return (
    <div className="benchmark">
      <div className="score-row">
        <ScoreCard label="Success" value={`${Math.round(result.summary.success_rate * 100)}%`} />
        <ScoreCard label="Cases" value={`${result.summary.passed_cases}/${result.summary.total_cases}`} />
        <ScoreCard label="Confidence" value={`${Math.round(result.summary.average_confidence * 100)}%`} />
        <ScoreCard label="Final X" value={result.summary.average_final_x.toFixed(2)} />
      </div>
      <div className="benchmark-cases">
        {result.cases.map((testCase) => (
          <div className={`benchmark-case ${testCase.passed ? "passed" : "failed"}`} key={testCase.id}>
            <div>
              <strong>{testCase.id}</strong>
              <p>{testCase.instruction}</p>
            </div>
            <span>{testCase.passed ? "PASS" : "FAIL"}</span>
            {!testCase.passed && <p className="error">{testCase.failure_reason}</p>}
          </div>
        ))}
      </div>
    </div>
  );
}

function ScoreCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="score-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function DogzillaStatusViewer({
  status,
  isBusy,
  onRefresh,
  onEmergencyStop,
}: {
  status: DogzillaRuntimeStatus | null;
  isBusy: boolean;
  onRefresh: () => void;
  onEmergencyStop: () => void;
}) {
  if (!status) {
    return (
      <div className="dogzilla-empty">
        <EmptyState text="Dogzilla Runtime状態を取得しています" />
        <button onClick={onRefresh} disabled={isBusy}>
          <RefreshCw size={18} />
          Refresh
        </button>
      </div>
    );
  }

  return (
    <div className="dogzilla-status">
      <div className={`connection-card ${status.connected ? "connected" : "disconnected"}`}>
        <strong>{status.connected ? "Connected" : "Disconnected"}</strong>
        <span>{status.runtime_url}</span>
      </div>
      <div className="meta-grid compact">
        <span>Mode</span>
        <strong>{status.state?.mode ?? "-"}</strong>
        <span>Battery</span>
        <strong>{status.state ? `${status.state.battery.toFixed(0)}%` : "-"}</strong>
        <span>IMU</span>
        <strong>
          r {(status.state?.roll ?? 0).toFixed(1)} / p {(status.state?.pitch ?? 0).toFixed(1)} / y {(status.state?.yaw ?? 0).toFixed(1)}
        </strong>
        <span>Checked</span>
        <strong>{new Date(status.last_checked).toLocaleTimeString()}</strong>
      </div>
      {status.error && <p className="error">{status.error}</p>}
      <div className="safety-limits">
        <strong>Safety Limits</strong>
        <span>x {status.safety.max_linear_x} / y {status.safety.max_linear_y} / yaw {status.safety.max_yaw_deg} / {status.safety.max_duration_ms}ms</span>
      </div>
      <button className="danger full-width" onClick={onEmergencyStop} disabled={isBusy || !status.safety.emergency_stop_available}>
        <Square size={18} />
        Emergency Stop
      </button>
    </div>
  );
}

function SimulatorViewer({ state, environment }: { state?: SimulatorState; environment: Environment }) {
  if (environment !== "simulator") {
    return <EmptyState text="Simulatorを選ぶと軌跡が表示されます" />;
  }

  return (
    <div>
      <Simulator3D state={state} />
      <p className="sim3d-hint">ドラッグで視点回転 / ホイールでズーム</p>
      <div className="meta-grid compact simulator-meta">
        <span>Mode</span>
        <strong>{state?.mode ?? "idle"}</strong>
        <span>Pose</span>
        <strong>
          x {(state?.x ?? 0).toFixed(2)} / y {(state?.y ?? 0).toFixed(2)} / yaw {(state?.yaw_deg ?? 0).toFixed(0)}
        </strong>
      </div>
    </div>
  );
}

function VisionViewer({
  result,
  status,
  isStatusBusy,
  isVisionBusy,
  uploadedImageName,
  onRefreshStatus,
  onImageSelected,
  onRunDogzillaCamera,
}: {
  result: PerceptionResult | null;
  status: PerceptionServiceStatus | null;
  isStatusBusy: boolean;
  isVisionBusy: boolean;
  uploadedImageName: string | null;
  onRefreshStatus: () => void;
  onImageSelected: (file: File | null) => void;
  onRunDogzillaCamera: () => void;
}) {
  const objects = result?.objects ?? [];
  const imageSize = result?.image_size ?? { width: 480, height: 320 };
  return (
    <div>
      <div className="service-status-row">
        <div className={`connection-card compact-card ${status?.connected ? "connected" : "disconnected"}`}>
          <strong>{status?.connected ? "Service Connected" : "Service Unknown"}</strong>
          <span>{status?.service_url ?? "http://localhost:8070"}</span>
        </div>
        <button className="icon-button" onClick={onRefreshStatus} disabled={isStatusBusy} title="Refresh Perception Service status">
          <RefreshCw size={18} />
        </button>
      </div>
      {status?.error && <p className="error">{status.error}</p>}
      <div className="vision-controls">
        <label className="upload-field">
          <input
            type="file"
            accept="image/*"
            onChange={(event) => onImageSelected(event.target.files?.[0] ?? null)}
          />
          <span>{uploadedImageName ?? "画像を選択(未選択ならサンプル)"}</span>
        </label>
        <button onClick={onRunDogzillaCamera} disabled={isVisionBusy} title="Dogzillaのカメラフレームで認識">
          <Bot size={16} />
          Dogzillaカメラ
        </button>
      </div>
      {result?.image_base64 ? (
        <div
          className="vision-stage vision-image-stage"
          aria-label="analyzed image"
          style={{ aspectRatio: `${imageSize.width} / ${imageSize.height}` }}
        >
          <img src={`data:image/jpeg;base64,${result.image_base64}`} alt={result.source} />
          {objects.map((object, index) => (
            <DetectionBox key={`${object.label}-${index}`} object={object} imageSize={imageSize} />
          ))}
        </div>
      ) : (
        <div className="vision-stage" aria-label="sample workbench">
          <div className="bench-surface">
            <div className="sample-object red-block" />
            <div className="sample-object blue-marker" />
            <div className="table-edge" />
          </div>
          {objects.map((object, index) => (
            <DetectionBox key={`${object.label}-${index}`} object={object} imageSize={imageSize} />
          ))}
        </div>
      )}
      {result ? (
        <div className="detections">
          <div className="meta-grid compact">
            <span>Source</span>
            <strong>{result.source}</strong>
            <span>Objects</span>
            <strong>{result.objects.length}</strong>
          </div>
          <ul>
            {result.objects.map((object, index) => (
              <li key={`${object.label}-${index}`}>
                <strong>{object.display_name}</strong>
                <span>{Math.round(object.confidence * 100)}% / {object.position_hint}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <EmptyState text="Visionを押すと認識結果が表示されます" />
      )}
    </div>
  );
}

function DetectionBox({
  object,
  imageSize,
}: {
  object: DetectedObject;
  imageSize: { width: number; height: number };
}) {
  const [x1, y1, x2, y2] = object.bbox;
  const width = imageSize.width || 480;
  const height = imageSize.height || 320;
  return (
    <div
      className="detection-box"
      style={{
        left: `${(x1 / width) * 100}%`,
        top: `${(y1 / height) * 100}%`,
        width: `${((x2 - x1) / width) * 100}%`,
        height: `${((y2 - y1) / height) * 100}%`,
      }}
    >
      <span>{object.display_name}</span>
    </div>
  );
}

function PhaseStep({ status, label, description }: { status: "done" | "active" | "next"; label: string; description: string }) {
  const Icon = status === "done" ? CheckCircle2 : Circle;
  return (
    <div className={`phase-step phase-step-${status}`}>
      <Icon size={18} />
      <div>
        <strong>{label}</strong>
        <span>{description}</span>
      </div>
    </div>
  );
}

function Capability({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="capability">
      <strong>{title}</strong>
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  return <div className={`status status-${status}`}>{status}</div>;
}

function EmptyState({ text }: { text: string }) {
  return <div className="empty">{text}</div>;
}

function formatStep(step: Task["plan"]["steps"][number]) {
  const parts = [];
  if (step.linear_x) parts.push(`x ${step.linear_x}`);
  if (step.linear_y) parts.push(`y ${step.linear_y}`);
  if (step.yaw_deg) parts.push(`yaw ${step.yaw_deg}deg`);
  if (step.duration_ms) parts.push(`${step.duration_ms}ms`);
  return parts.join(" / ") || "command";
}

createRoot(document.getElementById("root")!).render(<App />);
