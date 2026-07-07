<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { get, post } from "../lib/api";

  interface Song {
    id: string;
    youtube_id: string;
    title: string;
    thumbnail_url: string;
    author: string;
    prenda_id: string | null;
    prenda_done: boolean;
    status: string;
    requested_by: string;
  }

  let queue = $state<Song[]>([]);
  let current = $state<Song | null>(null);
  let ready = $state(false);
  let poll: ReturnType<typeof setInterval>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let player: any = null;

  // Only songs still queued with the prenda fulfilled are playable.
  const playable = (s: Song) => s.status === "queued" && s.prenda_done;

  // The list of upcoming songs excluding whatever is playing right now.
  const upcoming = $derived(queue.filter((s) => s.id !== current?.id));

  async function refresh() {
    const all = await get<Song[]>("/songs");
    queue = all.filter(playable);
    // If nothing is playing and we have songs, start.
    if (!current && ready && queue.length) playNext();
  }

  function playNext() {
    const next = queue.find((s) => s.id !== current?.id) ?? queue[0];
    if (!next) {
      current = null;
      return;
    }
    current = next;
    player?.loadVideoById(next.youtube_id);
  }

  async function onEnded() {
    if (current) {
      await post(`/songs/${current.id}/played`).catch(() => {});
    }
    const done = current;
    current = null;
    await refresh();
    // Avoid immediately replaying the just-finished song.
    queue = queue.filter((s) => s.id !== done?.id);
    if (queue.length) playNext();
  }

  async function skip() {
    await onEnded();
  }

  function initPlayer() {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const YT = (window as any).YT;
    player = new YT.Player("yt-player", {
      height: "100%",
      width: "100%",
      playerVars: { autoplay: 1, playsinline: 1 },
      events: {
        onReady: () => {
          ready = true;
          refresh();
        },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onStateChange: (e: any) => {
          if (e.data === YT.PlayerState.ENDED) onEnded();
        },
      },
    });
  }

  onMount(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const w = window as any;
    if (w.YT && w.YT.Player) {
      initPlayer();
    } else {
      const tag = document.createElement("script");
      tag.src = "https://www.youtube.com/iframe_api";
      document.head.appendChild(tag);
      w.onYouTubeIframeAPIReady = initPlayer;
    }
    poll = setInterval(refresh, 5000);
  });

  onDestroy(() => {
    clearInterval(poll);
    player?.destroy?.();
  });
</script>

<div class="row" style="margin-bottom:0.25rem;">
  <h2 style="margin:0;">📺 Player</h2>
  <span class="badge badge-cyan">TV</span>
</div>
<p class="muted">Toque esta tela na TV. Só toca músicas com a prenda cumprida.</p>

<div class="card stage">
  <div class="video">
    <div id="yt-player"></div>
    {#if !current}
      <div class="overlay">
        <span class="ovico">{ready ? "🎧" : "⏳"}</span>
        <span class="dim">
          {ready
            ? "Nada tocando — aguardando músicas destravadas."
            : "Carregando player…"}
        </span>
      </div>
    {/if}
  </div>

  <div class="nowbar">
    {#if current}
      <span class="badge badge-accent">▶ tocando agora</span>
      <div class="col" style="flex:1; min-width:0;">
        <span class="strong truncate">{current.title}</span>
        {#if current.author || current.requested_by}
          <span class="muted tiny truncate">
            {current.author || "—"} · pedida por {current.requested_by}
          </span>
        {/if}
      </div>
    {:else}
      <span class="muted" style="flex:1;">Sem música tocando</span>
    {/if}
    <button class="btn btn-ghost" onclick={skip} disabled={!current}>Pular ⏭</button>
  </div>
</div>

<div class="row" style="margin:1.75rem 0 0.75rem;">
  <h3 style="margin:0;">Próximas</h3>
  <span class="badge">{upcoming.length}</span>
</div>

{#if upcoming.length === 0}
  <div class="empty">Fila vazia. Destrave músicas na aba Jukebox. 🔓</div>
{:else}
  <div class="grid">
    {#each upcoming as s, i (s.id)}
      <div class="card card-hover next">
        <span class="idx">{i + 1}</span>
        {#if s.thumbnail_url}
          <img src={s.thumbnail_url} alt="" class="thumb nthumb" />
        {:else}
          <div class="thumb nthumb"></div>
        {/if}
        <div class="col" style="flex:1; min-width:0;">
          <span class="strong truncate">{s.title}</span>
          <span class="muted tiny truncate">{s.author || "—"} · {s.requested_by}</span>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .stage {
    padding: 0.75rem;
  }
  .video {
    position: relative;
    width: 100%;
    aspect-ratio: 16 / 9;
    background: #000;
    border-radius: var(--r-sm);
    overflow: hidden;
  }
  .video :global(#yt-player),
  .video :global(iframe) {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    border: 0;
  }
  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    text-align: center;
    padding: 1rem;
    background: radial-gradient(circle at 50% 40%, var(--surface), var(--bg));
  }
  .ovico {
    font-size: 2.5rem;
  }
  .nowbar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.75rem;
    padding: 0.15rem 0.25rem;
  }

  .next {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 0.75rem;
  }
  .idx {
    font-weight: 800;
    color: var(--muted);
    width: 1.4rem;
    text-align: center;
    flex-shrink: 0;
  }
  .nthumb {
    width: 96px;
    height: 54px;
  }
  @media (max-width: 520px) {
    .nthumb {
      width: 72px;
      height: 40px;
    }
  }
</style>
