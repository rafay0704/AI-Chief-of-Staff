"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Priority, Task } from "@/lib/schemas";
import { Button, Card, Input, Label, Select, Spinner, cn } from "./ui";

const priorityStyles: Record<Priority, string> = {
  high: "border-high/30 bg-high/10 text-high",
  medium: "border-medium/30 bg-medium/10 text-medium",
  low: "border-low/30 bg-low/10 text-low",
};

export function TaskPanel({ token }: { token: string }) {
  const qc = useQueryClient();
  const tasksQuery = useQuery({ queryKey: ["tasks"], queryFn: () => api.listTasks(token) });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["tasks"] });

  const create = useMutation({
    mutationFn: (input: { title: string; priority: Priority; duration_minutes: number }) =>
      api.createTask(token, input),
    onSuccess: invalidate,
  });
  const toggle = useMutation({
    mutationFn: (t: Task) =>
      api.updateTask(token, t.id, {
        status: t.status === "completed" ? "pending" : "completed",
      }),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteTask(token, id),
    onSuccess: invalidate,
  });

  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState<Priority>("medium");
  const [duration, setDuration] = useState(60);

  function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    create.mutate(
      { title: title.trim(), priority, duration_minutes: duration },
      { onSuccess: () => setTitle("") },
    );
  }

  const tasks = tasksQuery.data ?? [];
  const pending = tasks.filter((t) => t.status === "pending");
  const done = tasks.filter((t) => t.status === "completed");

  return (
    <Card className="flex flex-col p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-fg">Tasks</h2>
        <span className="text-xs text-faint">
          {pending.length} open · {done.length} done
        </span>
      </div>

      {/* Add task */}
      <form onSubmit={onAdd} className="mt-4 space-y-3">
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Add a task…"
          aria-label="Task title"
        />
        <div className="flex gap-2">
          <div className="w-32">
            <Label htmlFor="priority">Priority</Label>
            <Select
              id="priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as Priority)}
            >
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
            </Select>
          </div>
          <div className="w-28">
            <Label htmlFor="duration">Minutes</Label>
            <Input
              id="duration"
              type="number"
              min={5}
              step={5}
              value={duration}
              onChange={(e) => setDuration(Math.max(5, Number(e.target.value) || 5))}
            />
          </div>
          <div className="flex flex-1 items-end">
            <Button type="submit" loading={create.isPending} className="w-full">
              Add
            </Button>
          </div>
        </div>
      </form>

      {/* List */}
      <div className="mt-5 flex-1 space-y-1.5">
        {tasksQuery.isLoading && (
          <div className="flex items-center gap-2 py-8 text-sm text-muted">
            <Spinner className="size-4" /> Loading tasks…
          </div>
        )}
        {!tasksQuery.isLoading && tasks.length === 0 && (
          <p className="py-10 text-center text-sm text-faint">
            No tasks yet. Add a few, then generate a plan.
          </p>
        )}
        {[...pending, ...done].map((t) => (
          <TaskRow
            key={t.id}
            task={t}
            onToggle={() => toggle.mutate(t)}
            onDelete={() => remove.mutate(t.id)}
            busy={toggle.isPending || remove.isPending}
          />
        ))}
      </div>
    </Card>
  );
}

function TaskRow({
  task,
  onToggle,
  onDelete,
  busy,
}: {
  task: Task;
  onToggle: () => void;
  onDelete: () => void;
  busy: boolean;
}) {
  const completed = task.status === "completed";
  return (
    <div className="group flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-surface-2">
      <button
        onClick={onToggle}
        disabled={busy}
        aria-label={completed ? "Mark pending" : "Mark complete"}
        className={cn(
          "flex size-5 shrink-0 items-center justify-center rounded-md border transition-colors",
          completed
            ? "border-block-rest bg-block-rest/20 text-block-rest"
            : "border-border-strong text-transparent hover:border-accent",
        )}
      >
        <svg viewBox="0 0 16 16" className="size-3.5" fill="none">
          <path d="M3 8.5l3 3 7-7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
      </button>

      <div className="min-w-0 flex-1">
        <p className={cn("truncate text-sm", completed ? "text-faint line-through" : "text-fg")}>
          {task.title}
        </p>
      </div>

      <span
        className={cn(
          "shrink-0 rounded-md border px-1.5 py-0.5 text-[11px] font-medium capitalize",
          priorityStyles[task.priority],
        )}
      >
        {task.priority}
      </span>
      <span className="shrink-0 font-mono text-[11px] text-faint">{task.duration_minutes}m</span>

      <button
        onClick={onDelete}
        disabled={busy}
        aria-label="Delete task"
        className="shrink-0 rounded-md p-1 text-faint opacity-0 transition-opacity hover:text-high group-hover:opacity-100"
      >
        <svg viewBox="0 0 16 16" className="size-4" fill="none">
          <path
            d="M3 4h10M6.5 4V3h3v1M5 4l.5 9h5L11 4"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
    </div>
  );
}
