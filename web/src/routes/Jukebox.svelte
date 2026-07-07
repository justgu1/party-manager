<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { playerOpen, jukeboxSlot } from "../lib/player";
  import {
    playable,
    upcomingLocked,
    refresh,
    markDone,
    removeSong,
  } from "../lib/jukebox";

  // The persistent player docks over this element while the Jukebox is open.
  let slot = $state<HTMLDivElement | null>(null);

  onMount(() => {
    refresh();
    jukeboxSlot.set(slot);
  });
  onDestroy(() => jukeboxSlot.set(null));

  // Keep the store in sync once the element is bound.
  $effect(() => {
    jukeboxSlot.set(slot);
  });
</script>

<div class="row" style="margin-bottom:0.75rem;">
  <h2 style="margin:0;">🎵 Jukebox</h2>
  <span class="spacer"></span>
  <button class="btn btn-sm" onclick={() => ($playerOpen = true)}>+ Adicionar música</button>
</div>
<p class="muted">O player toca aqui e continua tocando mesmo se você navegar. Adicionar músicas e histórico ficam no painel lateral →</p>

<!-- Player docks over this slot (16:9). -->
<div bind:this={slot} class="slot"></div>

<h3 style="margin-top:1.5rem;">Próximas ({$playable.length})</h3>
{#if $playable.length === 0}
  <div class="empty">Nenhuma música destravada. Adicione uma e cumpra a prenda. 🎶</div>
{:else}
  <div class="grid">
    {#each $playable as s (s.id)}
      <div class="card card-hover song">
        {#if s.thumbnail_url}<img src={s.thumbnail_url} alt="" class="thumb sthumb" />{/if}
        <div class="col spacer" style="min-width:0;">
          <span class="strong truncate">{s.title}</span>
          <span class="muted small truncate">{s.author || "—"} · {s.requested_by}</span>
        </div>
        <span class="badge badge-ok">🔓</span>
      </div>
    {/each}
  </div>
{/if}

{#if $upcomingLocked.length}
  <h3 style="margin-top:1.5rem;">Aguardando prenda ({$upcomingLocked.length})</h3>
  <div class="grid">
    {#each $upcomingLocked as s (s.id)}
      <div class="card song locked">
        {#if s.thumbnail_url}<img src={s.thumbnail_url} alt="" class="thumb sthumb" />{/if}
        <div class="col spacer" style="min-width:0;">
          <span class="strong truncate">{s.title}</span>
          <span class="muted small truncate">🔒 {s.prenda_title} · {s.requested_by}</span>
        </div>
        <button class="btn btn-sm" onclick={() => markDone(s.id).then(refresh)}>Prenda cumprida</button>
        <button class="btn btn-ghost btn-sm" onclick={() => removeSong(s.id).then(refresh)}>🗑</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .slot {
    width: 100%;
    max-width: 720px;
    aspect-ratio: 16 / 9;
    margin: 0 auto;
    border-radius: 14px;
    background: var(--surface);
    border: 1px solid var(--border);
  }
  .grid {
    display: grid;
    gap: 0.5rem;
  }
  .song {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .song.locked {
    opacity: 0.85;
  }
  .sthumb {
    width: 90px;
    height: 51px;
  }
</style>
