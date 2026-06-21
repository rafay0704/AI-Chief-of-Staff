"use client";

import { useSyncExternalStore } from "react";
import type { User } from "./schemas";

// Auth session is kept in localStorage and exposed via useSyncExternalStore —
// the SSR-safe, effect-free way to read external mutable state in React. No
// AuthProvider is needed; the store is module-level.

const STORAGE_KEY = "acos.auth";

export interface Session {
  token: string;
  user: User;
}

// getSnapshot must return a stable reference between renders, so we cache the
// parsed value keyed on the raw string.
let cachedRaw: string | null = null;
let cachedSession: Session | null = null;

function read(): Session | null {
  const raw = typeof window === "undefined" ? null : window.localStorage.getItem(STORAGE_KEY);
  if (raw !== cachedRaw) {
    cachedRaw = raw;
    try {
      cachedSession = raw ? (JSON.parse(raw) as Session) : null;
    } catch {
      cachedSession = null;
    }
  }
  return cachedSession;
}

const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  const onStorage = (e: StorageEvent) => {
    if (e.key === STORAGE_KEY) cb(); // sync across tabs
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(cb);
    window.removeEventListener("storage", onStorage);
  };
}

export function signIn(session: Session): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  emit();
}

export function signOut(): void {
  window.localStorage.removeItem(STORAGE_KEY);
  emit();
}

// useHydrated returns false during SSR and the hydration render, true after —
// without any effect — so callers can avoid redirecting before hydration.
const noopSubscribe = () => () => {};

export function useAuth() {
  const session = useSyncExternalStore(subscribe, read, () => null);
  const ready = useSyncExternalStore(
    noopSubscribe,
    () => true,
    () => false,
  );
  return {
    user: session?.user ?? null,
    token: session?.token ?? null,
    ready,
    signIn,
    signOut,
  };
}
