import type { DogzillaRuntimeStatus, Environment, EvaluationResult, LessonWithStatus, PerceptionResult, PerceptionServiceStatus, Task } from "./types";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";

export async function createTask(input: { instruction: string; environment: Environment }): Promise<Task> {
  return request("/api/tasks", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function executeTask(taskId: string): Promise<Task> {
  return request("/api/actions/execute", {
    method: "POST",
    body: JSON.stringify({ task_id: taskId }),
  });
}

export async function stopTask(taskId: string): Promise<Task> {
  return request("/api/actions/stop", {
    method: "POST",
    body: JSON.stringify({ task_id: taskId }),
  });
}

export async function runPerception(input: {
  source: string;
  instruction: string;
  image_base64?: string;
}): Promise<PerceptionResult> {
  return request("/api/perception", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getPerceptionStatus(): Promise<PerceptionServiceStatus> {
  return request("/api/perception/status");
}

export async function getDogzillaStatus(): Promise<DogzillaRuntimeStatus> {
  return request("/api/robot/dogzilla");
}

export async function emergencyStopDogzilla(): Promise<DogzillaRuntimeStatus> {
  return request("/api/robot/dogzilla/stop", {
    method: "POST",
    body: JSON.stringify({}),
  });
}

export async function runEvaluation(input: { suite: string }): Promise<EvaluationResult> {
  return request("/api/evaluations/run", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function listEpisodes(): Promise<Task[]> {
  return request("/api/episodes");
}

export async function listEvaluations(): Promise<EvaluationResult[]> {
  return request("/api/evaluations");
}

export async function listLessons(): Promise<LessonWithStatus[]> {
  return request("/api/lessons");
}

export function subscribeTask(taskId: string, onTask: (task: Task) => void) {
  const source = new EventSource(`${API_BASE}/api/stream/tasks/${taskId}`);
  source.addEventListener("task", (event) => {
    onTask(JSON.parse(event.data));
  });
  source.onerror = () => {
    source.close();
  };
  return () => source.close();
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
    ...init,
  });
  const contentType = response.headers.get("content-type") ?? "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const message = typeof payload === "string" ? payload : payload.error;
    throw new Error(message || `Request failed: ${response.status}`);
  }
  return payload as T;
}
