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
