<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { get, post, put, del } from "../lib/api";
  import { user } from "../lib/auth";

  interface Rental {
    id: string;
    url: string;
    source: string;
    title: string;
    price: string;
    rating: string;
    reviews_count: number;
    image_url: string;
    status: "pending" | "scraped" | "failed";
    error: string;
    notes: string;
    score: number;
    upvotes: number;
    downvotes: number;
    my_vote: -1 | 0 | 1;
    rank: number;
    created_at: string;
  }

  // Bracket / tournament shapes
  interface Side {
    id: string;
    title: string;
    image_url: string;
    score: number;
  }
  interface Match {
    id: string;
    position: number;
    a: Side | null;
    b: Side | null;
    winner: string | null;
  }
  interface RoundData {
    round: number;
    label: string;
    matches: Match[];
  }
  interface Tournament {
    active: boolean;
    rounds: number;
    places: number;
    champion: Side | null;
    rounds_data: RoundData[];
  }

  let rentals = $state<Rental[]>([]);
  let tournament = $state<Tournament | null>(null);
  // id -> current user's vote, kept in sync with /rentals; mutated optimistically.
  let voteMap = $state<Record<string, -1 | 0 | 1>>({});

  let url = $state("");
  let adding = $state(false);
  let error = $state("");
  let open = $state(false); // right-hand management/vote drawer (local to this page)
  let notesDraft = $state<Record<string, string>>({});
  let savingNotes = $state<Record<string, boolean>>({});
  let poll: ReturnType<typeof setInterval>;

  const isAdmin = $derived(!!$user?.is_admin);

  // ------------------------------------------------------------------ data
  async function load() {
    try {
      const [list, t] = await Promise.all([
        get<Rental[]>("/rentals"),
        get<Tournament>("/rentals/tournament"),
      ]);
      rentals = list;
      tournament = t;
      const vm: Record<string, -1 | 0 | 1> = {};
      for (const r of list) {
        vm[r.id] = r.my_vote;
        if (notesDraft[r.id] === undefined) notesDraft[r.id] = r.notes ?? "";
      }
      voteMap = vm;
      error = "";
    } catch (e) {
      error = (e as Error).message;
    }
  }

  async function add(e: Event) {
    e.preventDefault();
    if (!url.trim()) return;
    adding = true;
    error = "";
    try {
      await post("/rentals", { url: url.trim() });
      url = "";
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      adding = false;
    }
  }

  async function del_(r: Rental) {
    if (!confirm(`Remover "${r.title || r.url}"?`)) return;
    try {
      await del(`/rentals/${r.id}`);
      await load();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  // Toggle vote on a place; optimistic then reload (the bracket re-seeds live).
  async function vote(r: Rental, value: 1 | -1) {
    const prev = voteMap[r.id] ?? 0;
    const undo = prev === value;
    const next: -1 | 0 | 1 = undo ? 0 : value;
    voteMap = { ...voteMap, [r.id]: next };
    try {
      if (undo) await del(`/rentals/${r.id}/vote`);
      else await post(`/rentals/${r.id}/vote`, { value });
    } catch (e) {
      error = (e as Error).message;
    }
    await load();
  }

  async function saveNotes(r: Rental) {
    savingNotes[r.id] = true;
    try {
      await put(`/rentals/${r.id}`, { notes: notesDraft[r.id] ?? "" });
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      savingNotes[r.id] = false;
    }
  }

  // ================================================================ canvas
  // Canvas can't read CSS custom properties, so mirror the palette here.
  const BG = "#0a0b12";
  const SURF = "#161927";
  const SURF2 = "#1e2233";
  const BORDER = "#2a2f47";
  const TEXT = "#eef1ff";
  const MUTED = "#7a82a8";
  const OK = "#35d39a";
  const DANGER = "#ff5470";
  const GOLD = "#ffcc47";
  const FONT = '"Inter", system-ui, -apple-system, "Segoe UI", Roboto, sans-serif';

  let canvasEl = $state<HTMLCanvasElement>();
  let wrapEl = $state<HTMLDivElement>();
  let rafId = 0;

  // Thumbnail cache: draw a placeholder circle until the image loads, then
  // schedule a single redraw so the photo pops in.
  const imgCache = new Map<string, HTMLImageElement>();
  function getImg(src: string): HTMLImageElement | null {
    if (!src) return null;
    const cached = imgCache.get(src);
    if (cached) return cached.complete && cached.naturalWidth > 0 ? cached : null;
    const img = new Image();
    img.onload = () => scheduleDraw();
    img.onerror = () => {};
    img.src = src;
    imgCache.set(src, img);
    return null;
  }

  function scheduleDraw() {
    if (rafId) return;
    rafId = requestAnimationFrame(() => {
      rafId = 0;
      draw();
    });
  }

  function draw() {
    const cv = canvasEl;
    const wrap = wrapEl;
    const t = tournament;
    if (!cv || !wrap || !t || !t.active || !t.rounds_data?.length) return;

    // --- structure: split each non-final round into a left/right half that
    // converge on the central final. ---
    const rd = [...t.rounds_data].sort((a, b) => a.round - b.round);
    const R = rd.length;
    const finalRD = rd[R - 1];

    type Col = { colIndex: number; label: string; matches: Match[] };
    const leftCols: Col[] = [];
    const rightCols: Col[] = [];
    for (let r = 0; r < R - 1; r++) {
      const ms = [...rd[r].matches].sort((a, b) => a.position - b.position);
      const half = Math.ceil(ms.length / 2);
      leftCols.push({ colIndex: r, label: rd[r].label, matches: ms.slice(0, half) });
      rightCols.push({ colIndex: 2 * R - 2 - r, label: rd[r].label, matches: ms.slice(half) });
    }
    const finalCol: Col = {
      colIndex: R - 1,
      label: finalRD.label,
      matches: [...finalRD.matches].sort((a, b) => a.position - b.position),
    };

    const champion = t.champion;
    const numCols = 2 * R - 1;
    const perSide0 = Math.max(1, Math.ceil(rd[0].matches.length / 2));

    const topPad = champion ? 116 : 30;
    const botPad = 24;
    const Hb = perSide0 * 130; // ~130px per first-round match (2 stacked teams + air)
    const cssH = topPad + Hb + botPad;
    const minW = numCols * 160;
    const cssW = Math.max(wrap.clientWidth || minW, minW);
    const colW = cssW / numCols;
    const cardW = Math.min(colW - 18, 216);
    const cardH = 64;
    const teamH = cardH / 2;

    // --- HiDPI backing store ---
    const dpr = window.devicePixelRatio || 1;
    cv.width = Math.round(cssW * dpr);
    cv.height = Math.round(cssH * dpr);
    cv.style.width = cssW + "px";
    cv.style.height = cssH + "px";
    const ctx = cv.getContext("2d");
    if (!ctx) return;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, cssW, cssH);
    ctx.fillStyle = BG;
    ctx.fillRect(0, 0, cssW, cssH);

    const colXof = (i: number) => (i + 0.5) * colW;
    // The doubling of matches per round means match j of the next round sits at
    // the vertical midpoint of the pair (2j, 2j+1) that feeds it — the maths closes.
    const cyOf = (k: number, m: number) => topPad + ((k + 0.5) * Hb) / m;

    const seg = (x1: number, y1: number, x2: number, y2: number) => {
      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.lineTo(x2, y2);
      ctx.stroke();
    };
    const roundRect = (x: number, y: number, w: number, h: number, r: number) => {
      const rr = Math.min(r, w / 2, h / 2);
      ctx.beginPath();
      ctx.moveTo(x + rr, y);
      ctx.arcTo(x + w, y, x + w, y + h, rr);
      ctx.arcTo(x + w, y + h, x, y + h, rr);
      ctx.arcTo(x, y + h, x, y, rr);
      ctx.arcTo(x, y, x + w, y, rr);
      ctx.closePath();
    };
    const trunc = (text: string, maxW: number) => {
      if (maxW <= 0) return "";
      if (ctx.measureText(text).width <= maxW) return text;
      let s = text;
      while (s.length && ctx.measureText(s + "…").width > maxW) s = s.slice(0, -1);
      return s + "…";
    };

    // ------------------------------------------------ connectors (behind cards)
    const connect = (from: Col, to: Col, side: "left" | "right") => {
      const fx = colXof(from.colIndex);
      const tx = colXof(to.colIndex);
      const fm = from.matches.length;
      const tn = to.matches.length;
      for (let k = 0; k < fm; k++) {
        const m = from.matches[k];
        const y = cyOf(k, fm);
        const j = Math.floor((k * tn) / fm);
        const yN = cyOf(j, tn);
        let xStart: number, xEnd: number;
        if (side === "left") {
          xStart = fx + cardW / 2;
          xEnd = tx - cardW / 2;
        } else {
          xStart = fx - cardW / 2;
          xEnd = tx + cardW / 2;
        }
        const midX = (xStart + xEnd) / 2;
        ctx.lineWidth = 2;
        // outgoing trace of a decided winner glows green
        ctx.strokeStyle = m.winner ? OK : BORDER;
        seg(xStart, y, midX, y);
        ctx.strokeStyle = BORDER;
        seg(midX, y, midX, yN);
        seg(midX, yN, xEnd, yN);
      }
    };
    for (let r = 0; r < R - 1; r++) {
      connect(leftCols[r], r < R - 2 ? leftCols[r + 1] : finalCol, "left");
      connect(rightCols[r], r < R - 2 ? rightCols[r + 1] : finalCol, "right");
    }

    // ------------------------------------------------ column labels
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.font = `700 10px ${FONT}`;
    ctx.fillStyle = MUTED;
    for (const col of [...leftCols, ...rightCols]) {
      ctx.fillText(col.label.toUpperCase(), colXof(col.colIndex), 15);
    }
    if (!champion) ctx.fillText(finalCol.label.toUpperCase(), colXof(finalCol.colIndex), 15);

    // ------------------------------------------------ match cards
    const drawTeam = (m: Match, s: Side | null, x: number, y: number, w: number, h: number) => {
      const rowCy = y + h / 2;
      const padX = 9;
      if (!s) {
        ctx.globalAlpha = 1;
        ctx.fillStyle = MUTED;
        ctx.font = `italic 12px ${FONT}`;
        ctx.textAlign = "left";
        ctx.textBaseline = "middle";
        ctx.fillText("vaga livre", x + padX, rowCy);
        return;
      }
      const win = m.winner === s.id;
      const lose = !!m.winner && m.winner !== s.id;
      if (win) {
        roundRect(x + 2, y + 2, w - 4, h - 4, 7);
        ctx.fillStyle = "rgba(53,211,154,0.14)";
        ctx.fill();
        ctx.fillStyle = OK;
        ctx.fillRect(x + 2, y + 5, 3, h - 10);
      }
      ctx.globalAlpha = lose ? 0.45 : 1;

      // thumb (photo if loaded, otherwise a small dot)
      const r = 9;
      const tcx = x + padX + r;
      const img = getImg(s.image_url);
      if (img) {
        ctx.save();
        ctx.beginPath();
        ctx.arc(tcx, rowCy, r, 0, Math.PI * 2);
        ctx.clip();
        ctx.drawImage(img, tcx - r, rowCy - r, 2 * r, 2 * r);
        ctx.restore();
      } else {
        ctx.beginPath();
        ctx.arc(tcx, rowCy, r, 0, Math.PI * 2);
        ctx.fillStyle = win ? OK : SURF;
        ctx.fill();
      }
      ctx.lineWidth = 1;
      ctx.strokeStyle = BORDER;
      ctx.beginPath();
      ctx.arc(tcx, rowCy, r, 0, Math.PI * 2);
      ctx.stroke();

      const tx = tcx + r + 8;
      const rightX = x + w - padX;
      const scoreStr = String(s.score);
      ctx.font = `800 13px ${FONT}`;
      const scoreW = ctx.measureText(scoreStr).width;
      const markW = win ? 20 : 0;

      // title (elided by measured width)
      ctx.font = `600 13px ${FONT}`;
      ctx.textAlign = "left";
      ctx.textBaseline = "middle";
      ctx.fillStyle = lose ? MUTED : TEXT;
      ctx.fillText(trunc(s.title || "—", rightX - scoreW - markW - 6 - tx), tx, rowCy);

      // score
      ctx.textAlign = "right";
      ctx.font = `800 13px ${FONT}`;
      ctx.fillStyle = s.score > 0 ? OK : s.score < 0 ? DANGER : MUTED;
      ctx.fillText(scoreStr, rightX, rowCy);
      if (win) {
        ctx.font = `12px ${FONT}`;
        ctx.fillText("🏆", rightX - scoreW - 5, rowCy);
      }
      ctx.globalAlpha = 1;
    };

    const drawMatch = (m: Match, cx: number, cy: number) => {
      const w = cardW;
      const h = cardH;
      const x = cx - w / 2;
      const y = cy - h / 2;
      roundRect(x, y, w, h, 10);
      ctx.fillStyle = SURF2;
      ctx.fill();
      ctx.lineWidth = 1;
      ctx.strokeStyle = BORDER;
      ctx.stroke();
      ctx.strokeStyle = BORDER;
      seg(x + 8, y + h / 2, x + w - 8, y + h / 2);
      drawTeam(m, m.a, x, y, w, teamH);
      drawTeam(m, m.b, x, y + teamH, w, teamH);
    };

    const drawColumn = (col: Col) => {
      const x = colXof(col.colIndex);
      const m = col.matches.length;
      for (let k = 0; k < m; k++) drawMatch(col.matches[k], x, cyOf(k, m));
    };
    leftCols.forEach(drawColumn);
    rightCols.forEach(drawColumn);
    drawColumn(finalCol);

    // ------------------------------------------------ champion (above the final)
    {
      const cx = colXof(finalCol.colIndex);
      const boxW = Math.min(colW - 12, 236);
      const boxH = 84;
      const x = cx - boxW / 2;
      const y = 14;

      ctx.save();
      ctx.shadowColor = "rgba(255,204,71,0.5)";
      ctx.shadowBlur = 26;
      roundRect(x, y, boxW, boxH, 12);
      ctx.fillStyle = SURF;
      ctx.fill();
      ctx.restore();
      roundRect(x, y, boxW, boxH, 12);
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = GOLD;
      ctx.stroke();

      // golden line dropping toward the final
      ctx.strokeStyle = GOLD;
      ctx.lineWidth = 2;
      seg(cx, y + boxH, cx, cyOf(0, 1) - cardH / 2);

      ctx.textAlign = "left";
      ctx.textBaseline = "middle";
      ctx.font = `26px ${FONT}`;
      ctx.fillText("🏆", x + 14, y + boxH / 2);
      const tx = x + 14 + 36;
      ctx.font = `700 10px ${FONT}`;
      ctx.fillStyle = GOLD;
      ctx.fillText("CAMPEÃO", tx, y + 22);
      if (champion) {
        ctx.font = `800 15px ${FONT}`;
        ctx.fillStyle = TEXT;
        ctx.fillText(trunc(champion.title || "—", x + boxW - 14 - tx), tx, y + 45);
        ctx.font = `700 12px ${FONT}`;
        ctx.fillStyle = OK;
        ctx.fillText("saldo " + champion.score, tx, y + 65);
      } else {
        ctx.font = `800 18px ${FONT}`;
        ctx.fillStyle = MUTED;
        ctx.fillText("—", tx, y + 50);
      }
    }
  }

  // Redraw on data change …
  $effect(() => {
    // touch reactive deps so the effect re-runs when they change
    tournament;
    if (canvasEl && wrapEl) scheduleDraw();
  });
  // … and on wrapper resize.
  $effect(() => {
    if (!wrapEl) return;
    const ro = new ResizeObserver(() => scheduleDraw());
    ro.observe(wrapEl);
    return () => ro.disconnect();
  });

  onMount(() => {
    load();
    poll = setInterval(load, 4000);
  });
  onDestroy(() => {
    clearInterval(poll);
    if (rafId) cancelAnimationFrame(rafId);
  });
</script>

<!-- ====================== PAGE ====================== -->
<div class="head row">
  <div class="col" style="gap:0.2rem;min-width:0;">
    <h2 style="margin:0;">🏆 Chaveamento de Lugares</h2>
    <p class="muted small" style="margin:0;">
      Mata-mata ao vivo: o líder de cada confronto avança rumo ao centro, até sobrar o campeão.
    </p>
  </div>
  <span class="spacer"></span>
  <button class="btn" onclick={() => (open = true)}>🏆 Lugares &amp; votos</button>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if tournament?.active}
  <div class="canvas-wrap" bind:this={wrapEl}>
    <canvas bind:this={canvasEl}></canvas>
  </div>
  <p class="tiny muted center" style="margin-top:0.6rem;">
    Deslize na horizontal para ver toda a chave · abra “Lugares &amp; votos” para votar.
  </p>
{:else}
  <div class="empty">
    <div style="font-size:2rem;margin-bottom:0.5rem;">🏟️</div>
    {#if isAdmin}
      <p class="strong dim" style="margin:0;">Adicione ao menos 2 lugares para formar a chave.</p>
      <p class="small" style="margin:0.4rem 0 0;">
        Abra “Lugares &amp; votos” e cole os links dos anúncios.
      </p>
    {:else}
      <p class="strong dim" style="margin:0;">Aguardando lugares…</p>
      <p class="small" style="margin:0.4rem 0 0;">
        A chave aparece assim que houver ao menos 2 lugares.
      </p>
    {/if}
  </div>
{/if}

<!-- ====================== DRAWER (local to this page) ====================== -->
{#if open}
  <div
    class="rd-backdrop"
    role="button"
    tabindex="-1"
    aria-label="Fechar"
    onclick={() => (open = false)}
    onkeydown={(e) => e.key === "Escape" && (open = false)}
  ></div>
{/if}

<aside class="rd-drawer" class:open>
  <header class="rd-head">
    <strong>🏆 Lugares &amp; votos</strong>
    <span class="badge">{rentals.length}</span>
    <span class="spacer"></span>
    <button class="btn btn-ghost btn-sm" onclick={() => (open = false)}>✕</button>
  </header>

  <div class="rd-body">
    {#if isAdmin}
      <form onsubmit={add} class="stack" style="margin-bottom:0.9rem;">
        <div class="row" style="gap:0.4rem;">
          <span class="badge badge-warn">ADMIN</span>
          <span class="small dim">Cole o link — o lugar entra na chave automaticamente.</span>
        </div>
        <div class="row">
          <input bind:value={url} placeholder="Link do anúncio (Airbnb, Booking, OLX…)" />
          <button class="btn" type="submit" disabled={adding}>{adding ? "…" : "Add"}</button>
        </div>
      </form>
      <div class="divider"></div>
    {/if}

    {#if rentals.length === 0}
      <p class="empty" style="padding:1.2rem;">Nenhum lugar ainda.</p>
    {:else}
      <div class="stack-sm">
        {#each rentals as r (r.id)}
          <div class="rd-item">
            <div class="row" style="gap:0.55rem;align-items:flex-start;">
              {#if r.image_url}
                <img src={r.image_url} alt="" class="thumb rd-th" />
              {:else}
                <div class="thumb rd-th center muted">🏠</div>
              {/if}
              <div class="col" style="min-width:0;flex:1;gap:0.25rem;">
                <a
                  href={r.url}
                  target="_blank"
                  rel="noopener"
                  class="strong truncate"
                  style="color:var(--text);text-decoration:none;"
                  title={r.title || r.url}>{r.title || r.url}</a
                >
                <div class="row" style="gap:0.35rem;flex-wrap:wrap;">
                  {#if r.status === "pending"}
                    <span class="badge badge-warn">⏳ Raspando…</span>
                  {:else if r.status === "failed"}
                    <span class="badge badge-danger" title={r.error}>✕ Falhou</span>
                  {:else}
                    <span class="badge badge-ok">✓ OK</span>
                  {/if}
                  {#if r.price}<span class="badge">{r.price}</span>{/if}
                </div>
              </div>
              <div class="rd-vote">
                <button
                  class="vbtn up"
                  class:on={voteMap[r.id] === 1}
                  title="Votar a favor"
                  aria-label="Votar a favor de {r.title}"
                  onclick={() => vote(r, 1)}>▲</button
                >
                <span class="vbal" class:pos={r.score > 0} class:neg={r.score < 0}>{r.score}</span>
                <button
                  class="vbtn down"
                  class:on={voteMap[r.id] === -1}
                  title="Votar contra"
                  aria-label="Votar contra {r.title}"
                  onclick={() => vote(r, -1)}>▼</button
                >
              </div>
            </div>

            {#if isAdmin}
              <textarea
                rows="2"
                bind:value={notesDraft[r.id]}
                placeholder="Anotações sobre este lugar…"
              ></textarea>
              <div class="row" style="gap:0.4rem;">
                <button class="btn btn-ghost btn-sm" disabled={savingNotes[r.id]} onclick={() => saveNotes(r)}>
                  {savingNotes[r.id] ? "Salvando…" : "Salvar notas"}
                </button>
                <span class="spacer"></span>
                <button class="btn btn-danger btn-sm" onclick={() => del_(r)}>🗑 Excluir</button>
              </div>
            {:else if r.notes}
              <p class="small muted" style="margin:0;">{r.notes}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</aside>

<style>
  .head {
    margin-bottom: 1rem;
    align-items: flex-start;
  }

  /* Canvas bracket */
  .canvas-wrap {
    overflow-x: auto;
    overflow-y: hidden;
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    background: var(--bg);
    padding: 0;
  }
  .canvas-wrap canvas {
    display: block;
  }

  /* Drawer (fixed, slides in from the right, below the navbar z-index) */
  .rd-backdrop {
    position: fixed;
    inset: 0;
    z-index: 70;
    background: rgba(0, 0, 0, 0.45);
  }
  .rd-drawer {
    position: fixed;
    top: 0;
    right: 0;
    height: 100vh;
    width: 420px;
    max-width: 92vw;
    z-index: 80;
    background: var(--bg-elev);
    border-left: 1px solid var(--border);
    transform: translateX(105%);
    transition: transform 0.25s ease;
    display: flex;
    flex-direction: column;
  }
  .rd-drawer.open {
    transform: translateX(0);
  }
  .rd-head {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.7rem 0.9rem;
    border-bottom: 1px solid var(--border);
    min-height: 58px;
  }
  .rd-body {
    flex: 1;
    overflow-y: auto;
    padding: 0.9rem;
  }
  .stack-sm > * + * {
    margin-top: 0.5rem;
  }
  .rd-item {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0.6rem;
    border: 1px solid var(--border);
    border-radius: var(--r);
    background: var(--surface);
  }
  .rd-th {
    width: 52px;
    height: 52px;
    font-size: 1.1rem;
  }

  /* Vote control */
  .rd-vote {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.15rem;
  }
  .vbal {
    font-size: 0.85rem;
    font-weight: 800;
    color: var(--text-dim);
    min-width: 1.4ch;
    text-align: center;
  }
  .vbal.pos {
    color: var(--ok);
  }
  .vbal.neg {
    color: var(--danger);
  }
  .vbtn {
    font: inherit;
    font-size: 0.85rem;
    line-height: 1;
    cursor: pointer;
    width: 30px;
    height: 22px;
    border-radius: var(--r-sm);
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--muted);
    transition: all 0.12s ease;
  }
  .vbtn:hover {
    color: var(--text);
    border-color: var(--border-strong);
  }
  .vbtn.up.on {
    background: linear-gradient(180deg, var(--ok), #24b483);
    color: #04150f;
    border-color: transparent;
  }
  .vbtn.down.on {
    background: linear-gradient(180deg, var(--danger), #d63a58);
    color: #fff;
    border-color: transparent;
  }
</style>
