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

export async function logout(): Promise<void> {
  try {
    await fetch(`${RouteV1.Auth}/logout`, {
      method: "POST",
      credentials: "include",
    });
  } finally {
    isAuthenticated.set(false);
  }
}

// fetch wrapper for authenticated pages, sends the payload with the credentials if response is 401, it redirects to login.
export async function sessionFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response | undefined> {
  const res = await fetch(input, { ...init, credentials: "include" });
  if (res.status === 401) {
    isAuthenticated.set(false);
    window.location.href = "/";
    return;
  }
  return res;
}
