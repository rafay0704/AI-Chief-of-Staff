"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Habit } from "@/lib/schemas";
import { Button, Card, Input, Spinner, cn } from "./ui";

const WINDOW = 28;

// Last 28 UTC days, oldest → newest (today is last). Matches the backend window.
function windowDays(): string[] {
  return Array.from({ length: WINDOW }, (_, i) => {
    const d = new Date();
    d.setUTCDate(d.getUTCDate() - (WINDOW - 1 - i));
    return d.toISOString().slice(0, 10);
  });
}

export function HabitsPanel({ token }: { token: string }) {
  const qc = useQueryClient();
  const habitsQuery = useQuery({ queryKey: ["habits"], queryFn: () => api.listHabits(token) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["habits"] });

  const create = useMutation({
    mutationFn: (name: string) => api.createHabit(token, name),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteHabit(token, id),
    onSuccess: invalidate,
  });
  const check = useMutation({
    mutationFn: (v: { id: string; date: string }) => api.checkHabit(token, v.id, v.date),
    onSuccess: invalidate,
  });
  const uncheck = useMutation({
    mutationFn: (v: { id: string; date: string }) => api.uncheckHabit(token, v.id, v.date),
    onSuccess: invalidate,
  });

  const [name, setName] = useState("");
  const days = windowDays();
  const today = days[days.length - 1];

  function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    create.mutate(name.trim(), { onSuccess: () => setName("") });
  }

  const habits = habitsQuery.data ?? [];

  return (
    <Card className="p-6">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-fg">Habits</h2>
        <span className="text-xs text-faint">last 4 weeks · today {today.slice(5)}</span>
      </div>

      <form onSubmit={onAdd} className="mt-4 flex gap-2">
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="New habit (e.g. Read 20 min)…"
          aria-label="Habit name"
        />
        <Button type="submit" loading={create.isPending}>
          Add
        </Button>
      </form>

      <div className="mt-5 space-y-4">
        {habitsQuery.isLoading && (
          <div className="flex items-center gap-2 py-6 text-sm text-muted">
            <Spinner className="size-4" /> Loading habits…
          </div>
        )}
        {!habitsQuery.isLoading && habits.length === 0 && (
          <p className="py-8 text-center text-sm text-faint">
            No habits yet. Add one and start a streak.
          </p>
        )}
        {habits.map((h) => (
          <HabitRow
            key={h.id}
            habit={h}
            days={days}
            today={today}
            onToggle={(date, checked) =>
              checked ? uncheck.mutate({ id: h.id, date }) : check.mutate({ id: h.id, date })
            }
            onDelete={() => remove.mutate(h.id)}
          />
        ))}
      </div>
    </Card>
  );
}

function HabitRow({
  habit,
  days,
  today,
  onToggle,
  onDelete,
}: {
  habit: Habit;
  days: string[];
  today: string;
  onToggle: (date: string, checked: boolean) => void;
  onDelete: () => void;
}) {
  const set = new Set(habit.checkins);
  return (
    <div className="group rounded-lg px-1 py-1">
      <div className="mb-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-fg">{habit.name}</span>
          <span
            className={cn(
              "inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] font-semibold",
              habit.streak > 0 ? "bg-accent/15 text-accent" : "bg-surface-2 text-faint",
            )}
            title="Current streak"
          >
            <FlameIcon className="size-3" />
            {habit.streak}
          </span>
        </div>
        <button
          onClick={onDelete}
          aria-label="Delete habit"
          className="rounded-md p-1 text-faint opacity-0 transition-opacity hover:text-high group-hover:opacity-100"
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
      <div className="flex flex-wrap gap-1">
        {days.map((d) => {
          const checked = set.has(d);
          const isToday = d === today;
          return (
            <button
              key={d}
              onClick={() => onToggle(d, checked)}
              title={d}
              aria-label={`${habit.name} ${d} ${checked ? "done" : "not done"}`}
              className={cn(
                "size-4 rounded-[4px] transition-colors",
                checked ? "bg-accent hover:bg-accent-strong" : "bg-surface-2 hover:bg-border",
                isToday && "ring-1 ring-accent/70",
              )}
            />
          );
        })}
      </div>
    </div>
  );
}

function FlameIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={className} fill="currentColor" aria-hidden="true">
      <path d="M8 1s4 3 4 7a4 4 0 1 1-8 0c0-1.3.6-2.3 1.2-3 .1 1 .8 1.6 1.3 1.6.8 0 .9-1 .6-2.1C6.7 3.4 8 1 8 1z" />
    </svg>
  );
}
