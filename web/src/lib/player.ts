import { writable } from "svelte/store";

// Whether the side drawer (add-song form + history) is open.
export const playerOpen = writable(false);

// The Jukebox page registers its player slot here so the persistent player can
// dock over it while that route is active.
export const jukeboxSlot = writable<HTMLElement | null>(null);
