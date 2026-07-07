// Shared jukebox state so the persistent player (App-level) and the Jukebox
// page render from one source of truth.
import { writable, derived } from "svelte/store";
import { get as apiGet, post, del } from "./api";

export interface Prenda {
  id: string;
  title: string;
}
export interface Song {
  id: string;
  youtube_id: string;
  url: string;
  title: string;
  thumbnail_url: string;
  author: string;
  prenda_id: string | null;
  prenda_title: string;
  prenda_done: boolean;
  status: "queued" | "playing" | "played" | string;
  requested_by: string;
}

export const songs = writable<Song[]>([]);
export const prendas = writable<Prenda[]>([]);

export const playable = derived(songs, ($s) =>
  $s.filter((x) => x.status === "queued" && x.prenda_done),
);
export const upcomingLocked = derived(songs, ($s) =>
  $s.filter((x) => x.status === "queued" && !x.prenda_done),
);
export const history = derived(songs, ($s) => $s.filter((x) => x.status === "played"));

export async function refresh() {
  try {
    songs.set(await apiGet<Song[]>("/songs"));
  } catch {
    /* keep last known queue on transient errors */
  }
}
export async function loadPrendas() {
  try {
    prendas.set(await apiGet<Prenda[]>("/prendas"));
  } catch {
    prendas.set([]);
  }
}
export const addSong = (url: string, prendaId: string) =>
  post("/songs", { url, prenda_id: prendaId });
export const markDone = (id: string) => post(`/songs/${id}/prenda-done`);
export const requeue = (id: string) => post(`/songs/${id}/requeue`);
export const markPlayed = (id: string) => post(`/songs/${id}/played`);
export const removeSong = (id: string) => del(`/songs/${id}`);
