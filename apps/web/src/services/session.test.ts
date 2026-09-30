import { beforeEach, describe, expect, it } from "vitest";
import {
  clearSession,
  getSessionToken,
  readSession,
  saveSession,
} from "./session";

describe("session storage", () => {
  beforeEach(() => localStorage.clear());

  it("persists and restores the authenticated user and token together", () => {
    const session = {
      user: {
        id: "user-1",
        email: "sales@example.com",
        displayName: "Sales",
        role: "sales" as const,
        isActive: true,
        createdAt: "2026-01-01T00:00:00Z",
      },
      token: "token-1",
    };

    saveSession(session);

    expect(readSession()).toEqual(session);
    expect(getSessionToken()).toBe(session.token);
  });

  it("fails closed for malformed stored session data and clears both keys", () => {
    localStorage.setItem("rbc-session", "{malformed");

    expect(readSession()).toBeNull();

    clearSession();
    expect(readSession()).toBeNull();
    expect(getSessionToken()).toBeNull();
  });
});
