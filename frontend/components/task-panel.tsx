"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { Breakdown, Priority, PriorityResult, Task } from "@/lib/schemas";
import { Button, Card, Input, Label, Select, Spinner, cn } from "./ui";

const priorityStyles: Record<Priority, string> = {
  high: "border-high/30 bg-high/10 text-high",
  medium: "border-medium/30 bg-medium/10 text-medium",
  low: "border-low/30 bg-low/10 text-low",
};

type RankInfo = { rank: number; urgent: boolean; reason: string };

export function TaskPanel({ token }: { token: string }) {
  const qc = useQueryClient();
  const tasksQuery = useQuery({ queryKey: ["tasks"], queryFn: () => api.listTasks(token) });

  const [priority, setPriority_] = useState<PriorityResult | null>(null);
  // Ranking is a snapshot — drop it whenever the task set changes.
  const refresh = () => {
    setPriority_(null);
    return qc.invalidateQueries({ queryKey: ["tasks"] });
  };

  const create = useMutation({
    mutationFn: (input: { title: string; priority: Priority; duration_minutes: number }) =>
      api.createTask(token, input),
    onSuccess: refresh,
  });
  const toggle = useMutation({
    mutationFn: (t: Task) =>
      api.updateTask(token, t.id, { status: t.status === "completed" ? "pending" : "completed" }),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteTask(token, id),
    onSuccess: refresh,
  });
  const prioritize = useMutation({
    mutationFn: () => api.prioritize(token),
    onSuccess: setPriority_,
  });

  const [title, setTitle] = useState("");
  const [prio, setPrio] = useState<Priority>("medium");
  const [duration, setDuration] = useState(60);

  function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    create.mutate(
      { title: title.trim(), priority: prio, duration_minutes: duration },
      { onSuccess: () => setTitle("") },
    );
  }

  const tasks = tasksQuery.data ?? [];
  const pending = tasks.filter((t) => t.status === "pending");
  const done = tasks.filter((t) => t.status === "completed");

  const rankMap = new Map<string, RankInfo>();
  priority?.ranked.forEach((r) => rankMap.set(r.task_id, r));

  // When a ranking exists, order pending tasks by rank.
  const orderedPending = [...pending].sort(
    (a, b) => (rankMap.get(a.id)?.rank ?? Infinity) - (rankMap.get(b.id)?.rank ?? Infinity),
  );

  return (
    <Card className="flex flex-col p-6">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-fg">Tasks</h2>
        <div className="flex items-center gap-3">
          <span className="text-xs text-faint">
            {pending.length} open · {done.length} done
          </span>
          <Button
            variant="ghost"
            className="px-2.5 py-1.5 text-xs"
            disabled={pending.length === 0}
            loading={prioritize.isPending}
            onClick={() => prioritize.mutate()}
            title="Rank your pending tasks with AI"
          >
            <SparkIcon className="size-3.5" />
            Prioritize
          </Button>
        </div>
      </div>
      {prioritize.error instanceof ApiError && (
        <p className="mt-2 text-xs text-high">{prioritize.error.message}</p>
      )}

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
            <Select id="priority" value={prio} onChange={(e) => setPrio(e.target.value as Priority)}>
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
        {[...orderedPending, ...done].map((t) => (
          <TaskRow
            key={t.id}
            task={t}
            token={token}
            rank={rankMap.get(t.id)}
            onToggle={() => toggle.mutate(t)}
            onDelete={() => remove.mutate(t.id)}
            busy={toggle.isPending || remove.isPending}
          />
        ))}
      </div>

      {/* Drop suggestions */}
      {priority && priority.drop_suggestions.length > 0 && (
        <div className="mt-4 rounded-lg border border-border bg-surface-2/40 p-3">
          <p className="mb-1.5 text-xs font-medium text-muted">Suggested to drop</p>
          <ul className="space-y-1">
            {priority.drop_suggestions.map((d) => {
              const task = tasks.find((t) => t.id === d.task_id);
              return (
                <li key={d.task_id} className="text-xs text-faint">
                  <span className="text-fg">{task?.title ?? "Task"}</span> — {d.reason}
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </Card>
  );
}

function TaskRow({
  task,
  token,
  rank,
  onToggle,
  onDelete,
  busy,
}: {
  task: Task;
  token: string;
  rank?: RankInfo;
  onToggle: () => void;
  onDelete: () => void;
  busy: boolean;
}) {
  const completed = task.status === "completed";
  const [open, setOpen] = useState(false);
  const [breakdown, setBreakdown] = useState<Breakdown | null>(null);

  const breakdownMut = useMutation({
    mutationFn: () => api.breakdown(token, task.id),
    onSuccess: setBreakdown,
  });

  function onBreakdown() {
    const next = !open;
    setOpen(next);
    if (next && !breakdown && !breakdownMut.isPending) breakdownMut.mutate();
  }

  return (
    <div className="rounded-lg hover:bg-surface-2">
      <div className="group flex items-center gap-3 px-2 py-2">
        {rank && !completed && (
          <span
            className={cn(
              "flex size-5 shrink-0 items-center justify-center rounded-md text-[11px] font-semibold",
              rank.urgent ? "bg-high/15 text-high" : "bg-surface-2 text-muted",
            )}
            title={rank.reason}
          >
            {rank.rank}
          </span>
        )}

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

        {rank?.urgent && !completed && (
          <span className="shrink-0 rounded-md border border-high/30 bg-high/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-high">
            urgent
          </span>
        )}
        <span
          className={cn(
            "shrink-0 rounded-md border px-1.5 py-0.5 text-[11px] font-medium capitalize",
            priorityStyles[task.priority],
          )}
        >
          {task.priority}
        </span>
        <span className="shrink-0 font-mono text-[11px] text-faint">{task.duration_minutes}m</span>

        {!completed && (
          <button
            onClick={onBreakdown}
            aria-label="Break down task"
            title="Break this task into steps"
            className={cn(
              "shrink-0 rounded-md p-1 transition-colors",
              open ? "text-accent" : "text-faint hover:text-accent",
            )}
          >
            <BranchIcon className="size-4" />
          </button>
        )}
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

      {/* Breakdown steps */}
      {open && (
        <div className="ml-9 mb-2 mr-2 rounded-lg border border-border bg-surface-2/40 px-3 py-2.5">
          {breakdownMut.isPending && (
            <div className="flex items-center gap-2 text-xs text-muted">
              <Spinner className="size-3.5" /> Breaking it down…
            </div>
          )}
          {breakdownMut.error instanceof ApiError && (
            <p className="text-xs text-high">{breakdownMut.error.message}</p>
          )}
          {breakdown && (
            <ol className="space-y-1.5">
              {breakdown.steps.map((s) => (
                <li key={s.order} className="flex items-baseline gap-2 text-xs">
                  <span className="font-mono text-faint">{s.order}.</span>
                  <span className="flex-1 text-fg">{s.title}</span>
                  <span className="font-mono text-faint">{s.duration_minutes}m</span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}
    </div>
  );
}

function SparkIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={className} fill="currentColor" aria-hidden="true">
      <path d="M8 1l1.4 3.9L13 6.3 9.4 7.7 8 11.6 6.6 7.7 3 6.3l3.6-1.4L8 1z" />
      <path d="M13 10l.6 1.6L15 12l-1.4.4L13 14l-.6-1.6L11 12l1.4-.4L13 10z" opacity="0.6" />
    </svg>
  );
}

function BranchIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={className} fill="none" aria-hidden="true">
      <path
        d="M4 2v6a2 2 0 0 0 2 2h6M4 8a2 2 0 1 0 0-4 2 2 0 0 0 0 4zm8 4a2 2 0 1 0 0-4 2 2 0 0 0 0 4z"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
