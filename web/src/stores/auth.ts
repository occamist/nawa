import { RouteV1 } from "@libs/route";
import { persistentAtom } from "@nanostores/persistent";

export const isAuthenticated = persistentAtom<boolean>("nawa_auth", false, {
  encode: JSON.stringify,
  decode: JSON.parse,
});

export async function login(username: string, password: string): Promise<void> {
  const res = await fetch(`${RouteV1.Auth}/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.trim() || "Login failed");
  }
  isAuthenticated.set(true);
}

/** Call on any 401 response or explicit logout — clears local state. */
export function clearAuth(): void {
  isAuthenticated.set(false);
}

export async function logout(): Promise<void> {
  await fetch(`${RouteV1.Auth}/logout`, {
    method: "POST",
    credentials: "include",
  });
  clearAuth();
}
