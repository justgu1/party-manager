import { getToken, clearSession } from "./auth";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

// api performs a JSON request against the Go backend, injecting the JWT and
// unwrapping errors into ApiError.
export async function api<T = unknown>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  const tok = getToken();
  if (tok) headers.set("Authorization", `Bearer ${tok}`);

  const res = await fetch(`/api${path}`, { ...options, headers });

  if (res.status === 401) {
    clearSession();
  }

  const text = await res.text();
  const body = text ? JSON.parse(text) : null;

  if (!res.ok) {
    const msg = body?.error ?? `request failed (${res.status})`;
    throw new ApiError(res.status, msg);
  }
  return body as T;
}

// upload posts multipart/form-data (e.g. a shopping receipt). The browser sets
// the Content-Type boundary, so we must not set it ourselves.
export async function upload<T = unknown>(path: string, form: FormData): Promise<T> {
  const headers = new Headers();
  const tok = getToken();
  if (tok) headers.set("Authorization", `Bearer ${tok}`);

  const res = await fetch(`/api${path}`, { method: "POST", body: form, headers });
  if (res.status === 401) clearSession();

  const text = await res.text();
  const body = text ? JSON.parse(text) : null;
  if (!res.ok) throw new ApiError(res.status, body?.error ?? `request failed (${res.status})`);
  return body as T;
}

export const get = <T>(p: string) => api<T>(p);
export const post = <T>(p: string, data?: unknown) =>
  api<T>(p, { method: "POST", body: data ? JSON.stringify(data) : undefined });
export const put = <T>(p: string, data?: unknown) =>
  api<T>(p, { method: "PUT", body: data ? JSON.stringify(data) : undefined });
export const del = <T>(p: string) => api<T>(p, { method: "DELETE" });
