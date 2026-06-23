"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { Wordmark } from "@/components/brand";
import { HabitsPanel } from "@/components/habits-panel";
import { InsightsPanel } from "@/components/insights-panel";
import { PlanPanel } from "@/components/plan-panel";
import { TaskPanel } from "@/components/task-panel";
import { Button, Spinner } from "@/components/ui";
import { useAuth } from "@/lib/auth";

export default function DashboardPage() {
  const { ready, token, user, signOut } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (ready && !token) router.replace("/login");
  }, [ready, token, router]);

  if (!ready || !token) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted">
        <Spinner className="size-6" />
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b border-border bg-ink/80 backdrop-blur-md">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-5 py-3.5">
          <Wordmark />
          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-muted sm:inline">
              {user?.name ?? user?.email}
            </span>
            <Button variant="ghost" onClick={signOut} className="px-3 py-1.5 text-xs">
              Sign out
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-5 py-8">
        <div className="mb-6">
          <h1 className="text-xl font-semibold tracking-tight text-fg">
            Good {greeting()}, {firstName(user?.name) ?? "there"}.
          </h1>
          <p className="mt-1 text-sm text-muted">
            Capture what matters, then let your chief of staff shape the day.
          </p>
        </div>

        <div className="space-y-5">
          <InsightsPanel token={token} />
          <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
            <TaskPanel token={token} />
            <PlanPanel token={token} />
          </div>
          <HabitsPanel token={token} />
        </div>
      </main>
    </div>
  );
}

function greeting(): string {
  const h = new Date().getHours();
  if (h < 12) return "morning";
  if (h < 18) return "afternoon";
  return "evening";
}

function firstName(name?: string): string | undefined {
  return name?.trim().split(/\s+/)[0];
}
