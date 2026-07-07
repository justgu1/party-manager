import { writable } from "svelte/store";

export interface User {
  id: string;
  email: string;
  name: string;
  is_admin: boolean;
}

const TOKEN_KEY = "hp_token";
const USER_KEY = "hp_user";

function loadUser(): User | null {
  const raw = localStorage.getItem(USER_KEY);
  return raw ? (JSON.parse(raw) as User) : null;
}

export const token = writable<string | null>(localStorage.getItem(TOKEN_KEY));
export const user = writable<User | null>(loadUser());

export function setSession(newToken: string, newUser: User) {
  localStorage.setItem(TOKEN_KEY, newToken);
  localStorage.setItem(USER_KEY, JSON.stringify(newUser));
  token.set(newToken);
  user.set(newUser);
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
  token.set(null);
  user.set(null);
}

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}
