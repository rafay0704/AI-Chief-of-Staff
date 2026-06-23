import { z } from "zod";
import {
  authResponseSchema,
  breakdownSchema,
  habitSchema,
  habitsResponseSchema,
  planJobSchema,
  planSchema,
  priorityResultSchema,
  statsSchema,
  taskSchema,
  tasksResponseSchema,
  weeklyReportSchema,
  type PlanMode,
  type Priority,
  type TaskStatus,
} from "./schemas";

// ApiError carries the backend's error envelope { error: { code, message } }.
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

const errorEnvelope = z.object({
  error: z.object({ code: z.string(), message: z.string() }),
});

interface RequestOptions<T> {
  method?: string;
  body?: unknown;
  token?: string | null;
  schema?: z.ZodType<T>;
}

async function request<T>(path: string, opts: RequestOptions<T> = {}): Promise<T> {
  const { method = "GET", body, token, schema } = opts;

  const res = await fetch(`/api${path}`, {
    method,
    headers: {
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (!res.ok) {
    let code = "error";
    let message = `Request failed (${res.status})`;
    try {
      const parsed = errorEnvelope.safeParse(await res.json());
      if (parsed.success) {
        code = parsed.data.error.code;
        message = parsed.data.error.message;
      }
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(res.status, code, message);
  }

  if (res.status === 204) return undefined as T;

  const json = await res.json();
  return schema ? schema.parse(json) : (json as T);
}

// ── Auth ──────────────────────────────────────────────────────────────────────

export const api = {
  register: (input: { name: string; email: string; password: string }) =>
    request("/auth/register", { method: "POST", body: input, schema: authResponseSchema }),

  login: (input: { email: string; password: string }) =>
    request("/auth/login", { method: "POST", body: input, schema: authResponseSchema }),

  // ── Tasks ────────────────────────────────────────────────────────────────────

  listTasks: (token: string) =>
    request("/tasks", { token, schema: tasksResponseSchema }).then((r) => r.tasks),

  createTask: (
    token: string,
    input: { title: string; description?: string; priority?: Priority; duration_minutes?: number },
  ) => request("/tasks", { method: "POST", body: input, token, schema: taskSchema }),

  updateTask: (
    token: string,
    id: string,
    input: Partial<{
      title: string;
      description: string;
      priority: Priority;
      duration_minutes: number;
      status: TaskStatus;
    }>,
  ) => request(`/tasks/${id}`, { method: "PATCH", body: input, token, schema: taskSchema }),

  deleteTask: (token: string, id: string) =>
    request<void>(`/tasks/${id}`, { method: "DELETE", token }),

  // ── Plans ────────────────────────────────────────────────────────────────────

  generatePlan: (
    token: string,
    input: { date: string; available_minutes?: number; goals?: string[]; mode?: PlanMode },
  ) => request("/plans/generate", { method: "POST", body: input, token, schema: planJobSchema }),

  getPlanJob: (token: string, jobId: string) =>
    request(`/plans/jobs/${jobId}`, { token, schema: planJobSchema }),

  getPlanByDate: (token: string, date: string) =>
    request(`/plans?date=${encodeURIComponent(date)}`, { token, schema: planSchema }),

  // ── Interactive AI ───────────────────────────────────────────────────────────

  prioritize: (token: string) =>
    request("/ai/prioritize", { method: "POST", token, schema: priorityResultSchema }),

  breakdown: (token: string, taskId: string) =>
    request(`/ai/breakdown/${taskId}`, { method: "POST", token, schema: breakdownSchema }),

  weeklyReport: (token: string) =>
    request("/ai/weekly-report", { method: "POST", token, schema: weeklyReportSchema }),

  // ── Analytics ────────────────────────────────────────────────────────────────

  stats: (token: string) => request("/stats", { token, schema: statsSchema }),

  // ── Habits ───────────────────────────────────────────────────────────────────

  listHabits: (token: string) =>
    request("/habits", { token, schema: habitsResponseSchema }).then((r) => r.habits),

  createHabit: (token: string, name: string) =>
    request("/habits", { method: "POST", body: { name }, token, schema: habitSchema }),

  deleteHabit: (token: string, id: string) =>
    request<void>(`/habits/${id}`, { method: "DELETE", token }),

  checkHabit: (token: string, id: string, date: string) =>
    request<void>(`/habits/${id}/checkin`, { method: "POST", body: { date }, token }),

  uncheckHabit: (token: string, id: string, date: string) =>
    request<void>(`/habits/${id}/checkin?date=${encodeURIComponent(date)}`, {
      method: "DELETE",
      token,
    }),
};
