import { z } from "zod";

// Mirrors the Go API contracts (see backend/internal/domain and docs/API.md).

export const userSchema = z.object({
  id: z.string(),
  name: z.string(),
  email: z.string(),
  created_at: z.string(),
});
export type User = z.infer<typeof userSchema>;

export const authResponseSchema = z.object({
  user: userSchema,
  token: z.string(),
});

export const prioritySchema = z.enum(["low", "medium", "high"]);
export type Priority = z.infer<typeof prioritySchema>;

export const taskStatusSchema = z.enum(["pending", "completed"]);
export type TaskStatus = z.infer<typeof taskStatusSchema>;

export const taskSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  priority: prioritySchema,
  duration_minutes: z.number(),
  status: taskStatusSchema,
  created_at: z.string(),
  updated_at: z.string(),
});
export type Task = z.infer<typeof taskSchema>;

export const tasksResponseSchema = z.object({ tasks: z.array(taskSchema) });

export const scheduleItemSchema = z.object({
  time: z.string(),
  task: z.string(),
  type: z.enum(["focus", "admin", "meeting", "rest", "buffer"]).catch("focus"),
});
export type ScheduleItem = z.infer<typeof scheduleItemSchema>;

export const scheduleSchema = z.object({
  date: z.string(),
  summary: z.string(),
  schedule: z.array(scheduleItemSchema),
});
export type Schedule = z.infer<typeof scheduleSchema>;

export const planStatusSchema = z.enum(["queued", "running", "done", "failed"]);
export type PlanStatus = z.infer<typeof planStatusSchema>;

// POST /plans/generate response.
export const planJobSchema = z.object({
  job_id: z.string(),
  status: planStatusSchema,
  date: z.string(),
  schedule: scheduleSchema.optional(),
  error: z.string().optional(),
});
export type PlanJob = z.infer<typeof planJobSchema>;

// GET /plans?date= response.
export const planSchema = z.object({
  id: z.string(),
  date: z.string(),
  status: planStatusSchema,
  schedule: scheduleSchema.optional(),
  error: z.string().optional(),
  created_at: z.string(),
  updated_at: z.string(),
});
export type Plan = z.infer<typeof planSchema>;

// POST /ai/prioritize response.
export const priorityResultSchema = z.object({
  ranked: z.array(
    z.object({
      task_id: z.string(),
      rank: z.number(),
      reason: z.string(),
      urgent: z.boolean(),
    }),
  ),
  drop_suggestions: z
    .array(z.object({ task_id: z.string(), reason: z.string() }))
    .nullish()
    .transform((v) => v ?? []),
});
export type PriorityResult = z.infer<typeof priorityResultSchema>;

// POST /ai/breakdown/:id response.
export const breakdownSchema = z.object({
  task_id: z.string(),
  steps: z.array(
    z.object({
      order: z.number(),
      title: z.string(),
      duration_minutes: z.number(),
    }),
  ),
});
export type Breakdown = z.infer<typeof breakdownSchema>;

// GET /stats response.
export const statsSchema = z.object({
  total_tasks: z.number(),
  completed: z.number(),
  pending: z.number(),
  completion_rate: z.number(),
  pending_minutes: z.number(),
  completed_minutes: z.number(),
  by_priority: z.object({ high: z.number(), medium: z.number(), low: z.number() }),
  plans_generated: z.number(),
  trend: z.array(z.object({ date: z.string(), completed: z.number() })),
});
export type Stats = z.infer<typeof statsSchema>;

// POST /ai/weekly-report response.
const strList = z
  .array(z.string())
  .nullish()
  .transform((v) => v ?? []);
export const weeklyReportSchema = z.object({
  headline: z.string(),
  summary: z.string(),
  wins: strList,
  watch_outs: strList,
  suggestions: strList,
});
export type WeeklyReport = z.infer<typeof weeklyReportSchema>;

// Planner focus modes.
export const planModeSchema = z.enum(["balanced", "deep_focus", "stress_relief", "light"]);
export type PlanMode = z.infer<typeof planModeSchema>;

// Habit (GET /habits).
export const habitSchema = z.object({
  id: z.string(),
  name: z.string(),
  created_at: z.string(),
  streak: z.number(),
  checkins: z.array(z.string()),
});
export type Habit = z.infer<typeof habitSchema>;
export const habitsResponseSchema = z.object({ habits: z.array(habitSchema) });
