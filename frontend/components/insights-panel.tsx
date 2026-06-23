"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { Stats, WeeklyReport } from "@/lib/schemas";
import { Button, Card, Spinner, cn } from "./ui";

export function InsightsPanel({ token }: { token: string }) {
  const statsQuery = useQuery({ queryKey: ["stats"], queryFn: () => api.stats(token) });
  const [report, setReport] = useState<WeeklyReport | null>(null);
  const reportMut = useMutation({
    mutationFn: () => api.weeklyReport(token),
    onSuccess: setReport,
  });

  const s = statsQuery.data;

  return (
    <Card className="p-6">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-fg">Insights</h2>
        <Button
          variant="ghost"
          className="px-2.5 py-1.5 text-xs"
          loading={reportMut.isPending}
          onClick={() => reportMut.mutate()}
          disabled={!s || s.total_tasks === 0}
        >
          <SparkIcon className="size-3.5" />
          Weekly review
        </Button>
      </div>
      {reportMut.error instanceof ApiError && (
        <p className="mt-2 text-xs text-high">{reportMut.error.message}</p>
      )}

      {statsQuery.isLoading && (
        <div className="flex items-center gap-2 py-8 text-sm text-muted">
          <Spinner className="size-4" /> Loading insights…
        </div>
      )}

      {s && (
        <div className="mt-5 grid grid-cols-1 gap-6 md:grid-cols-[auto_1fr_1fr]">
          {/* Completion ring */}
          <div className="flex items-center gap-4">
            <Ring value={s.completion_rate} />
            <div>
              <div className="text-2xl font-semibold tracking-tight text-fg">
                {s.completed}
                <span className="text-base text-faint">/{s.total_tasks}</span>
              </div>
              <p className="text-xs text-muted">tasks completed</p>
            </div>
          </div>

          {/* Stat tiles */}
          <div className="grid grid-cols-3 gap-3">
            <Stat label="Focus done" value={`${Math.round(s.completed_minutes / 60)}h`} sub={`${s.completed_minutes}m`} />
            <Stat label="Planned load" value={`${Math.round(s.pending_minutes / 60)}h`} sub={`${s.pending} open`} />
            <Stat label="Plans made" value={`${s.plans_generated}`} sub="AI plans" />
          </div>

          {/* Trend + priority mix */}
          <div className="space-y-4">
            <Trend data={s.trend} />
            <PriorityMix by={s.by_priority} />
          </div>
        </div>
      )}

      {/* Weekly review */}
      {report && (
        <div className="mt-6 rounded-xl border border-accent/20 bg-accent/5 p-4">
          <p className="text-sm font-semibold text-accent">{report.headline}</p>
          <p className="mt-1.5 text-sm leading-relaxed text-muted">{report.summary}</p>
          <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <ReportList title="Wins" items={report.wins} tone="text-block-rest" />
            <ReportList title="Watch-outs" items={report.watch_outs} tone="text-medium" />
            <ReportList title="Suggestions" items={report.suggestions} tone="text-accent" />
          </div>
        </div>
      )}
    </Card>
  );
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="rounded-lg border border-border bg-surface-2/40 px-3 py-2.5">
      <div className="text-lg font-semibold tracking-tight text-fg">{value}</div>
      <div className="text-[11px] text-faint">{sub}</div>
      <div className="mt-0.5 text-[11px] font-medium text-muted">{label}</div>
    </div>
  );
}

function Ring({ value }: { value: number }) {
  const r = 30;
  const c = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(1, value));
  return (
    <div className="relative size-[76px]">
      <svg viewBox="0 0 76 76" className="size-full -rotate-90">
        <circle cx="38" cy="38" r={r} fill="none" stroke="var(--color-border)" strokeWidth="7" />
        <circle
          cx="38"
          cy="38"
          r={r}
          fill="none"
          stroke="var(--color-accent)"
          strokeWidth="7"
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={c * (1 - pct)}
        />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center text-sm font-semibold text-fg">
        {Math.round(pct * 100)}%
      </div>
    </div>
  );
}

function Trend({ data }: { data: Stats["trend"] }) {
  const max = Math.max(1, ...data.map((d) => d.completed));
  return (
    <div>
      <p className="mb-1.5 text-[11px] font-medium text-muted">Completed · last 7 days</p>
      <div className="flex items-end gap-1.5">
        {data.map((d) => (
          <div key={d.date} className="flex flex-1 flex-col items-center gap-1" title={`${d.date}: ${d.completed}`}>
            <div className="flex h-12 w-full items-end">
              <div
                className={cn("w-full rounded-sm", d.completed > 0 ? "bg-accent" : "bg-border")}
                style={{ height: `${Math.max(8, (d.completed / max) * 100)}%` }}
              />
            </div>
            <span className="text-[10px] text-faint">{d.date.slice(8)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function PriorityMix({ by }: { by: Stats["by_priority"] }) {
  const total = by.high + by.medium + by.low;
  const seg = (n: number) => (total === 0 ? 0 : (n / total) * 100);
  return (
    <div>
      <p className="mb-1.5 text-[11px] font-medium text-muted">Task mix by priority</p>
      <div className="flex h-2.5 overflow-hidden rounded-full bg-border">
        <div className="bg-high" style={{ width: `${seg(by.high)}%` }} />
        <div className="bg-medium" style={{ width: `${seg(by.medium)}%` }} />
        <div className="bg-low" style={{ width: `${seg(by.low)}%` }} />
      </div>
      <div className="mt-1.5 flex gap-3 text-[11px] text-muted">
        <Legend color="bg-high" label={`High ${by.high}`} />
        <Legend color="bg-medium" label={`Med ${by.medium}`} />
        <Legend color="bg-low" label={`Low ${by.low}`} />
      </div>
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="inline-flex items-center gap-1">
      <span className={cn("size-2 rounded-full", color)} /> {label}
    </span>
  );
}

function ReportList({ title, items, tone }: { title: string; items: string[]; tone: string }) {
  return (
    <div>
      <p className={cn("mb-1 text-[11px] font-semibold uppercase tracking-wide", tone)}>{title}</p>
      {items.length === 0 ? (
        <p className="text-xs text-faint">—</p>
      ) : (
        <ul className="space-y-1">
          {items.map((it, i) => (
            <li key={i} className="text-xs leading-snug text-muted">
              • {it}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function SparkIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={className} fill="currentColor" aria-hidden="true">
      <path d="M8 1l1.4 3.9L13 6.3 9.4 7.7 8 11.6 6.6 7.7 3 6.3l3.6-1.4L8 1z" />
    </svg>
  );
}
