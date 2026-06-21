"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { Schedule, ScheduleItem } from "@/lib/schemas";
import { Button, Card, Input, Label, Spinner, cn } from "./ui";

function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate(),
  ).padStart(2, "0")}`;
}

const blockStyles: Record<ScheduleItem["type"], { dot: string; label: string }> = {
  focus: { dot: "bg-block-focus", label: "text-block-focus" },
  rest: { dot: "bg-block-rest", label: "text-block-rest" },
  admin: { dot: "bg-block-admin", label: "text-block-admin" },
  meeting: { dot: "bg-block-meeting", label: "text-block-meeting" },
  buffer: { dot: "bg-block-buffer", label: "text-block-buffer" },
};

export function PlanPanel({ token }: { token: string }) {
  const qc = useQueryClient();
  const [date, setDate] = useState(todayISO());
  const [minutes, setMinutes] = useState(480);
  const [goals, setGoals] = useState("");
  const [activeJob, setActiveJob] = useState<string | null>(null);

  // Existing plan for the selected date (404 → none).
  const planQuery = useQuery({
    queryKey: ["plan", date],
    queryFn: async () => {
      try {
        return await api.getPlanByDate(token, date);
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
      }
    },
  });

  // Live polling of an in-flight generation job.
  const jobQuery = useQuery({
    queryKey: ["plan-job", activeJob],
    queryFn: () => api.getPlanJob(token, activeJob as string),
    enabled: !!activeJob,
    refetchInterval: (q) => {
      const s = q.state.data?.status;
      return s === "queued" || s === "running" ? 1500 : false;
    },
  });

  // Resume polling if the stored plan is still in progress (e.g. after reload).
  useEffect(() => {
    const s = planQuery.data?.status;
    if (!activeJob && planQuery.data && (s === "queued" || s === "running")) {
      setActiveJob(planQuery.data.id);
    }
  }, [planQuery.data, activeJob]);

  // When the job reaches a terminal state, refresh the stored plan and stop polling.
  useEffect(() => {
    const s = jobQuery.data?.status;
    if (activeJob && (s === "done" || s === "failed")) {
      qc.invalidateQueries({ queryKey: ["plan", date] });
      setActiveJob(null);
    }
  }, [jobQuery.data, activeJob, date, qc]);

  const generate = useMutation({
    mutationFn: () =>
      api.generatePlan(token, {
        date,
        available_minutes: minutes,
        goals: goals
          .split(",")
          .map((g) => g.trim())
          .filter(Boolean),
      }),
    onSuccess: (job) => setActiveJob(job.job_id),
  });

  // Derive the view from the live job (if active) else the stored plan.
  const live = activeJob ? jobQuery.data : undefined;
  const stored = planQuery.data ?? undefined;
  const status = live?.status ?? stored?.status ?? null;
  const schedule: Schedule | undefined = live?.schedule ?? stored?.schedule;
  const error = live?.error ?? stored?.error;
  const isWorking = status === "queued" || status === "running" || generate.isPending;

  return (
    <Card className="flex flex-col p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-fg">Daily plan</h2>
        {status && <StatusPill status={status} />}
      </div>

      {/* Controls */}
      <div className="mt-4 grid grid-cols-2 gap-2">
        <div>
          <Label htmlFor="date">Date</Label>
          <Input id="date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </div>
        <div>
          <Label htmlFor="minutes">Available minutes</Label>
          <Input
            id="minutes"
            type="number"
            min={30}
            step={30}
            value={minutes}
            onChange={(e) => setMinutes(Math.max(30, Number(e.target.value) || 30))}
          />
        </div>
      </div>
      <div className="mt-2">
        <Label htmlFor="goals">Goals (comma-separated)</Label>
        <Input
          id="goals"
          value={goals}
          onChange={(e) => setGoals(e.target.value)}
          placeholder="ship the release, deep work on auth"
        />
      </div>
      <Button
        className="mt-3"
        loading={generate.isPending || isWorking}
        onClick={() => generate.mutate()}
      >
        {schedule ? "Regenerate plan" : "Generate plan"}
      </Button>

      {generate.error instanceof ApiError && (
        <p className="mt-2 text-sm text-high">{generate.error.message}</p>
      )}

      {/* Result */}
      <div className="mt-5 flex-1">
        {isWorking && !schedule && (
          <div className="flex flex-col items-center gap-2 py-12 text-sm text-muted">
            <Spinner className="size-5" />
            <p>Your chief of staff is planning the day…</p>
          </div>
        )}

        {status === "failed" && (
          <div className="rounded-lg border border-high/30 bg-high/10 px-3 py-3 text-sm text-high">
            Planning failed: {error ?? "unknown error"}
          </div>
        )}

        {!isWorking && !schedule && status !== "failed" && (
          <p className="py-12 text-center text-sm text-faint">
            No plan for this date yet. Add tasks, then generate one.
          </p>
        )}

        {schedule && <ScheduleView schedule={schedule} dimmed={isWorking} />}
      </div>
    </Card>
  );
}

function StatusPill({ status }: { status: string }) {
  const map: Record<string, string> = {
    queued: "border-medium/30 bg-medium/10 text-medium",
    running: "border-accent/30 bg-accent/10 text-accent",
    done: "border-block-rest/30 bg-block-rest/10 text-block-rest",
    failed: "border-high/30 bg-high/10 text-high",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium capitalize",
        map[status] ?? "border-border text-muted",
      )}
    >
      {(status === "queued" || status === "running") && <Spinner className="size-3" />}
      {status}
    </span>
  );
}

function ScheduleView({ schedule, dimmed }: { schedule: Schedule; dimmed?: boolean }) {
  return (
    <div className={cn(dimmed && "opacity-50")}>
      {schedule.summary && (
        <p className="mb-4 rounded-lg border border-border bg-surface-2/60 px-3 py-2.5 text-sm leading-relaxed text-muted">
          {schedule.summary}
        </p>
      )}
      <ol className="space-y-1">
        {schedule.schedule.map((item, i) => {
          const s = blockStyles[item.type];
          return (
            <li key={i} className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-surface-2">
              <span className="w-28 shrink-0 font-mono text-xs text-faint">{item.time}</span>
              <span className={cn("size-2 shrink-0 rounded-full", s.dot)} />
              <span className="min-w-0 flex-1 truncate text-sm text-fg">{item.task}</span>
              <span className={cn("shrink-0 text-[11px] font-medium capitalize", s.label)}>
                {item.type}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
