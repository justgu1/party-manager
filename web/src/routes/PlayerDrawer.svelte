<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { location } from "svelte-spa-router";
  import { ApiError, get } from "../lib/api";
  import { playerOpen, jukeboxSlot } from "../lib/player";
  import {
    prendas,
    playable,
    history,
    refresh,
    loadPrendas,
    addSong,
    requeue,
    markPlayed,
    removeSong,
    type Song,
  } from "../lib/jukebox";

  // The one YouTube player. It's never destroyed/re-parented, so audio keeps
  // playing across navigation; we only move an overlay box that contains it
  // over whichever "slot" is active (the Jukebox page, or the drawer).
  let coreEl: HTMLDivElement;
  let drawerSlot = $state<HTMLDivElement | null>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let player: any = null;
  let ready = $state(false);
  let current = $state<Song | null>(null);
  let poll: ReturnType<typeof setInterval>;
  let raf = 0;

  let vw = $state(typeof window !== "undefined" ? window.innerWidth : 1024);
  const drawerW = $derived(Math.min(400, vw * 0.92));
  const playerH = $derived(Math.round(drawerW * (9 / 16)));

  const onJukebox = $derived($location === "/jukebox");
  const list = $derived($playable);

  // ---------- add-song form ----------
  interface SearchResult {
    video_id: string;
    title: string;
    channel: string;
    thumbnail_url: string;
  }
  let query = $state("");
  let results = $state<SearchResult[]>([]);
  let searching = $state(false);
  let searchError = $state("");
  let selected = $state<SearchResult | null>(null);
  let pastedUrl = $state("");
  let prendaId = $state("");
  let adding = $state(false);
  let addError = $state("");
  let debounce: ReturnType<typeof setTimeout>;

  const chosenUrl = $derived(
    selected ? `https://www.youtube.com/watch?v=${selected.video_id}` : pastedUrl.trim(),
  );
  const canAdd = $derived(!!chosenUrl && !!prendaId && !adding);

  async function runSearch(q: string) {
    if (!q.trim()) {
      results = [];
      searchError = "";
      searching = false;
      return;
    }
    searching = true;
    searchError = "";
    try {
      results = await get<SearchResult[]>(`/songs/search?q=${encodeURIComponent(q)}`);
    } catch (e) {
      results = [];
      searchError =
        e instanceof ApiError && e.status === 502
          ? "Busca do YouTube indisponível (sem chave). Cole o link do vídeo."
          : (e as Error).message;
    } finally {
      searching = false;
    }
  }
  function onQueryInput() {
    clearTimeout(debounce);
    debounce = setTimeout(() => runSearch(query), 400);
  }
  function pick(r: SearchResult) {
    selected = r;
    pastedUrl = "";
  }
  function clearSelection() {
    selected = null;
    query = "";
    results = [];
    pastedUrl = "";
    searchError = "";
  }
  async function submitAdd(e: Event) {
    e.preventDefault();
    if (!canAdd) return;
    adding = true;
    addError = "";
    try {
      await addSong(chosenUrl, prendaId);
      clearSelection();
      prendaId = "";
      await refresh();
    } catch (err) {
      addError = (err as Error).message;
    } finally {
      adding = false;
    }
  }

  // ---------- playback ----------
  function playNext() {
    const next = list.find((s) => s.id !== current?.id) ?? list[0];
    if (!next) {
      current = null;
      return;
    }
    current = next;
    player?.loadVideoById(next.youtube_id);
  }
  async function onEnded() {
    const done = current;
    current = null;
    if (done) await markPlayed(done.id).catch(() => {});
    await refresh();
    if (list.length) playNext();
  }
  async function skip() {
    await onEnded();
  }

  function initPlayer() {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const YT = (window as any).YT;
    player = new YT.Player("yt-player-core", {
      width: "100%",
      height: "100%",
      playerVars: { autoplay: 1, playsinline: 1 },
      events: {
        onReady: () => {
          ready = true;
          if (!current && list.length) playNext();
        },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onStateChange: (e: any) => {
          // 0 === ENDED
          if (e.data === 0) onEnded();
        },
      },
    });
  }

  // ---------- geometry: overlay the core over the active slot ----------
  function activeSlot(): HTMLElement | null {
    if (onJukebox) return $jukeboxSlot;
    if ($playerOpen) return drawerSlot;
    return null;
  }
  function applyGeo() {
    if (!coreEl) return;
    const s = coreEl.style;
    const slot = activeSlot();
    if (slot) {
      const r = slot.getBoundingClientRect();
      s.top = `${r.top}px`;
      s.left = `${r.left}px`;
      s.width = `${r.width}px`;
      s.height = `${r.height}px`;
      s.opacity = "1";
      s.pointerEvents = "auto";
      // Below the navbar when docked in the page; above the drawer when inside it.
      s.zIndex = onJukebox ? "15" : "90";
    } else {
      // Keep playing, tucked off-screen.
      s.top = "0px";
      s.left = `${vw + 60}px`;
      s.width = "320px";
      s.height = "180px";
      s.opacity = "0";
      s.pointerEvents = "none";
    }
  }

  onMount(() => {
    loadPrendas();
    refresh();
    poll = setInterval(async () => {
      await refresh();
      if (ready && !current && list.length) playNext();
      else if (current && list.length === 0) {
        // Queue emptied: stop so nothing keeps playing.
        current = null;
        player?.stopVideo?.();
      }
    }, 4000);

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const w = window as any;
    if (w.YT && w.YT.Player) initPlayer();
    else {
      const tag = document.createElement("script");
      tag.src = "https://www.youtube.com/iframe_api";
      document.head.appendChild(tag);
      w.onYouTubeIframeAPIReady = initPlayer;
    }

    const onResize = () => (vw = window.innerWidth);
    window.addEventListener("resize", onResize);

    const loop = () => {
      applyGeo();
      raf = requestAnimationFrame(loop);
    };
    raf = requestAnimationFrame(loop);

    return () => window.removeEventListener("resize", onResize);
  });
  onDestroy(() => {
    clearInterval(poll);
    clearTimeout(debounce);
    if (raf) cancelAnimationFrame(raf);
    player?.destroy?.();
  });

  function close() {
    $playerOpen = false;
  }
</script>

<!-- The single, persistent player. Fixed; positioned over the active slot. -->
<div bind:this={coreEl} class="pd-core">
  <div id="yt-player-core"></div>
  {#if !current}
    <div class="pd-idle">{ready ? "🎧 fila vazia" : "⏳"}</div>
  {/if}
</div>

{#if current && !onJukebox && !$playerOpen}
  <div class="pd-now">
    <span class="badge badge-accent">▶</span>
    <span class="strong truncate">{current.title}</span>
    <button class="btn btn-ghost btn-sm" onclick={skip}>Pular ⏭</button>
  </div>
{/if}

<!-- Backdrop: click outside closes the drawer. -->
{#if $playerOpen}
  <div
    class="pd-backdrop"
    role="button"
    tabindex="-1"
    aria-label="Fechar"
    onclick={close}
    onkeydown={(e) => e.key === "Escape" && close()}
  ></div>
{/if}

<!-- Drawer: (off-jukebox) player + upcoming, plus add-song form + history. -->
<aside class="pd-drawer" class:open={$playerOpen} style="width:{drawerW}px;">
  <header class="pd-head">
    <strong>🎵 Player</strong>
    <span class="spacer"></span>
    <button class="btn btn-ghost btn-sm" onclick={close}>✕</button>
  </header>

  <div class="pd-body">
    {#if !onJukebox}
      <!-- Off the Jukebox page the player docks at the drawer top. -->
      <div bind:this={drawerSlot} class="pd-drawerslot" style="height:{playerH}px;"></div>
      {#if current}
        <div class="row" style="margin-bottom:0.5rem;">
          <span class="truncate strong small spacer">▶ {current.title}</span>
          <button class="btn btn-ghost btn-sm" onclick={skip}>Pular ⏭</button>
        </div>
      {/if}
      <h4 class="pd-h">Próximas ({list.length})</h4>
      {#if list.length === 0}
        <p class="empty" style="padding:1rem;">Nada destravado. Libere músicas abaixo.</p>
      {:else}
        <div class="stack-sm">
          {#each list as s (s.id)}
            <div class="pd-song" class:now={s.id === current?.id}>
              {#if s.thumbnail_url}<img src={s.thumbnail_url} alt="" class="thumb pd-th" />{/if}
              <span class="truncate small">{s.title}</span>
            </div>
          {/each}
        </div>
      {/if}
      <div class="divider"></div>
    {/if}

    <h4 class="pd-h">Adicionar música</h4>
    <form onsubmit={submitAdd} class="stack">
      <div class="field">
        <span class="label">Buscar no YouTube</span>
        <input bind:value={query} oninput={onQueryInput} placeholder="Nome da música ou artista…" />
      </div>
      {#if searching}<p class="dim small">Buscando…</p>{/if}
      {#if searchError}<p class="error small">{searchError}</p>{/if}
      {#if results.length}
        <div class="pd-results">
          {#each results as r (r.video_id)}
            <button
              type="button"
              class="pd-result"
              class:sel={selected?.video_id === r.video_id}
              onclick={() => pick(r)}
            >
              <img src={r.thumbnail_url} alt="" class="thumb pd-th" />
              <span class="col" style="min-width:0;">
                <span class="truncate small strong">{r.title}</span>
                <span class="muted tiny truncate">{r.channel}</span>
              </span>
            </button>
          {/each}
        </div>
      {/if}
      <div class="field">
        <span class="label">…ou cole um link</span>
        <input
          bind:value={pastedUrl}
          oninput={() => (selected = null)}
          placeholder="https://youtube.com/watch?v=…"
        />
      </div>
      <div class="field">
        <span class="label">Prenda (obrigatória)</span>
        {#if $prendas.length === 0}
          <p class="empty" style="padding:0.8rem;">Peça a um admin para criar prendas.</p>
        {:else}
          <select bind:value={prendaId}>
            <option value="" disabled selected>Escolha uma prenda…</option>
            {#each $prendas as p (p.id)}
              <option value={p.id}>{p.title}</option>
            {/each}
          </select>
        {/if}
      </div>
      {#if addError}<p class="error small">{addError}</p>{/if}
      <button class="btn btn-block" type="submit" disabled={!canAdd}>
        {adding ? "Adicionando…" : "Adicionar à fila"}
      </button>
    </form>

    <div class="divider"></div>
    <h4 class="pd-h">Histórico ({$history.length})</h4>
    {#if $history.length === 0}
      <p class="muted small">Nada tocado ainda.</p>
    {:else}
      <div class="stack-sm">
        {#each $history as s (s.id)}
          <div class="pd-song played">
            {#if s.thumbnail_url}<img src={s.thumbnail_url} alt="" class="thumb pd-th" />{/if}
            <span class="truncate small spacer">{s.title}</span>
            <button class="btn btn-ghost btn-sm" onclick={() => requeue(s.id).then(refresh)}>↺</button>
            <button class="btn btn-ghost btn-sm" onclick={() => removeSong(s.id).then(refresh)}>🗑</button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</aside>

<style>
  .pd-core {
    position: fixed;
    z-index: 15;
    background: #000;
    border-radius: 12px;
    overflow: hidden;
    box-shadow: var(--shadow);
    transition: opacity 0.2s ease;
  }
  .pd-core :global(iframe),
  .pd-core :global(#yt-player-core) {
    width: 100%;
    height: 100%;
    border: 0;
    display: block;
  }
  .pd-idle {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--muted);
    background: radial-gradient(circle at 50% 40%, var(--surface), var(--bg));
    pointer-events: none;
  }
  .pd-now {
    position: fixed;
    z-index: 65;
    bottom: 12px;
    left: 12px;
    right: 12px;
    max-width: 520px;
    margin: 0 auto;
    display: flex;
    gap: 0.6rem;
    align-items: center;
    padding: 0.5rem 0.75rem;
    background: rgba(16, 18, 29, 0.92);
    backdrop-filter: blur(10px);
    border: 1px solid var(--border);
    border-radius: var(--r-pill);
  }
  .pd-backdrop {
    position: fixed;
    inset: 0;
    z-index: 70;
    background: rgba(0, 0, 0, 0.45);
  }
  .pd-drawer {
    position: fixed;
    top: 0;
    right: 0;
    height: 100vh;
    max-width: 92vw;
    z-index: 80;
    background: var(--bg-elev);
    border-left: 1px solid var(--border);
    transform: translateX(105%);
    transition: transform 0.25s ease;
    display: flex;
    flex-direction: column;
  }
  .pd-drawer.open {
    transform: translateX(0);
  }
  .pd-head {
    display: flex;
    align-items: center;
    padding: 0.7rem 0.9rem;
    border-bottom: 1px solid var(--border);
    min-height: 58px;
  }
  .pd-body {
    flex: 1;
    overflow-y: auto;
    padding: 0.9rem;
  }
  .pd-drawerslot {
    width: 100%;
    border-radius: 12px;
    background: var(--surface);
    margin-bottom: 0.6rem;
  }
  .pd-h {
    margin: 0.2rem 0 0.5rem;
    font-size: 0.9rem;
  }
  .stack-sm > * + * {
    margin-top: 0.35rem;
  }
  .pd-song {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem;
    border-radius: var(--r-sm);
    background: var(--surface);
  }
  .pd-song.now {
    border: 1px solid var(--accent);
  }
  .pd-song.played {
    opacity: 0.6;
  }
  .pd-th {
    width: 52px;
    height: 30px;
  }
  .pd-results {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    max-height: 260px;
    overflow-y: auto;
  }
  .pd-result {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    text-align: left;
    padding: 0.35rem;
    border-radius: var(--r-sm);
    background: var(--surface);
    border: 1px solid transparent;
    cursor: pointer;
    color: var(--text);
  }
  .pd-result.sel {
    border-color: var(--accent);
  }
</style>
