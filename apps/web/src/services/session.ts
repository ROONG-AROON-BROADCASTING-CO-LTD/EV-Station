import type { AuthSession } from "../types/domain";

export const SESSION_STORAGE_KEY = "rbc-session";
export const SESSION_TOKEN_STORAGE_KEY = "rbc-session-token";

export function readSession(): AuthSession | null {
  try {
    const saved = localStorage.getItem(SESSION_STORAGE_KEY);
    return saved ? (JSON.parse(saved) as AuthSession) : null;
  } catch {
    return null;
  }
}

export function saveSession(session: AuthSession) {
  localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
  localStorage.setItem(SESSION_TOKEN_STORAGE_KEY, session.token);
}

export function clearSession() {
  localStorage.removeItem(SESSION_STORAGE_KEY);
  localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
}

export function getSessionToken() {
  return localStorage.getItem(SESSION_TOKEN_STORAGE_KEY);
}
