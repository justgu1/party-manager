<script lang="ts">
  import { onMount, onDestroy, untrack } from "svelte";
  import { get, post } from "../lib/api";
  import { user } from "../lib/auth";

  interface SpinResp {
    options: string[];
    mode: string;
    winner_index: number;
    winner: string;
    seed: number;
  }

  interface HistoryItem {
    id: string;
    mode: "roulette" | "race";
    options: string[];
    winner: string;
    created_by: string;
    created_at: string;
  }

  let history = $state<HistoryItem[]>([]);

  async function loadHistory() {
    try {
      history = await get<HistoryItem[]>("/game/history");
    } catch {
      // History is non-critical; keep the current list on failure.
    }
  }

  // Logical canvas size — the backing store is scaled by DPR in onMount and
  // the element is stretched responsively via CSS (max-width:100%).
  const W = 640;
  const H = 420;

  // Palette derived from the design tokens (--accent, --accent-2, --cyan,
  // --gold, --ok, --danger …) plus a few sibling hues for enough variety.
  const PALETTE = [
    "#8b5cff", "#ff4d8d", "#35e0d0", "#ffcc47", "#35d39a", "#ff5470",
    "#7be04f", "#5b8cff", "#c77dff", "#ff8a5c", "#4dd0e1", "#f472b6",
  ];

  let raw = $state("Casa da praia\nChácara\nApê na cidade\nSítio na serra");
  let mode = $state<"roulette" | "race">("roulette");
  let spinning = $state(false);
  let winner = $state("");
  let error = $state("");
  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D;
  let rafId = 0;

  const isAdmin = $derived(!!$user?.is_admin);
  const opts = $derived(
    raw.split("\n").map((s) => s.trim()).filter(Boolean),
  );

  // Deterministic PRNG so the animation's "randomness" is reproducible from
  // the server seed (identical every replay, but unpredictable to the user).
  function lcg(seed: number) {
    let s = seed % 2147483647;
    if (s <= 0) s += 2147483646;
    return () => (s = (s * 16807) % 2147483647) / 2147483647;
  }

  const easeOutQuint = (t: number) => 1 - Math.pow(1 - t, 5);
  const easeInOut = (t: number) =>
    t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
  const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v));
  const trunc = (s: string, n: number) =>
    s.length > n ? s.slice(0, n - 1) + "…" : s;

  function stop() {
    if (rafId) cancelAnimationFrame(rafId);
    rafId = 0;
  }

  async function spin() {
    if (!isAdmin || spinning) return;
    error = "";
    if (opts.length < 2) {
      error = "Informe ao menos 2 opções.";
      return;
    }
    stop();
    spinning = true;
    winner = "";
    try {
      const res = await post<SpinResp>("/game/spin", {
        options: opts,
        mode,
      });
      if (res.mode === "roulette") await animateWheel(res);
      else await animateRace(res);
      winner = res.winner;
      launchConfetti(() => {
        if (res.mode === "roulette")
          drawWheel(res.options, wheelFinalRot(res), res.winner_index, 0);
        else drawRaceFrame(res, raceState!, true);
      });
      loadHistory();
    } catch (e) {
      error = (e as Error).message;
      startIdle();
    } finally {
      spinning = false;
    }
  }

  /* ============================ ROULETTE ============================ */

  function wheelFinalRot(res: SpinResp): number {
    const N = res.options.length;
    const seg = (2 * Math.PI) / N;
    const rng = lcg(res.seed + 7);
    const pointer = -Math.PI / 2; // pointer sits at the top
    const spins = 6 + Math.floor(rng() * 6); // 6–11 full turns
    // Land the winner's segment centre exactly under the pointer.
    return 2 * Math.PI * spins + (pointer - (res.winner_index + 0.5) * seg);
  }

  function drawWheel(
    options: string[],
    rot: number,
    highlight: number,
    tilt: number,
  ) {
    const N = options.length;
    const cx = W / 2;
    const cy = H / 2;
    const R = Math.min(W, H) / 2 - 30;
    const seg = (2 * Math.PI) / N;

    ctx.clearRect(0, 0, W, H);

    // Ambient glow behind the wheel.
    const glow = ctx.createRadialGradient(cx, cy, R * 0.2, cx, cy, R + 40);
    glow.addColorStop(0, "rgba(139,92,255,0.20)");
    glow.addColorStop(1, "rgba(139,92,255,0)");
    ctx.fillStyle = glow;
    ctx.fillRect(0, 0, W, H);

    // Sectors.
    for (let i = 0; i < N; i++) {
      const a0 = rot + i * seg;
      const a1 = a0 + seg;
      const base = PALETTE[i % PALETTE.length];
      const win = i === highlight;

      const g = ctx.createRadialGradient(cx, cy, R * 0.15, cx, cy, R);
      g.addColorStop(0, shade(base, win ? 0.35 : 0.12));
      g.addColorStop(1, shade(base, win ? -0.05 : -0.28));

      ctx.beginPath();
      ctx.moveTo(cx, cy);
      ctx.arc(cx, cy, R, a0, a1);
      ctx.closePath();
      ctx.fillStyle = g;
      ctx.fill();
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = "rgba(10,11,18,0.55)";
      ctx.stroke();

      if (win) {
        ctx.save();
        ctx.shadowColor = base;
        ctx.shadowBlur = 26;
        ctx.lineWidth = 3;
        ctx.strokeStyle = "#fff";
        ctx.stroke();
        ctx.restore();
      }

      // Radial label.
      const mid = a0 + seg / 2;
      ctx.save();
      ctx.translate(cx, cy);
      ctx.rotate(mid);
      ctx.fillStyle = "rgba(10,11,18,0.92)";
      ctx.font = `700 ${clamp(seg * R * 0.42, 10, 16)}px Inter, system-ui`;
      const maxChars = Math.max(6, Math.floor(N > 8 ? 10 : 16));
      const label = trunc(options[i], maxChars);
      if (Math.cos(mid) < 0) {
        ctx.rotate(Math.PI);
        ctx.textAlign = "left";
        ctx.fillText(label, -(R - 16), 4);
      } else {
        ctx.textAlign = "right";
        ctx.fillText(label, R - 16, 4);
      }
      ctx.restore();
    }

    // Rim + pegs.
    ctx.beginPath();
    ctx.arc(cx, cy, R + 6, 0, 2 * Math.PI);
    ctx.lineWidth = 6;
    ctx.strokeStyle = "#1e2233";
    ctx.stroke();
    for (let i = 0; i < N; i++) {
      const a = rot + i * seg;
      const px = cx + Math.cos(a) * (R + 6);
      const py = cy + Math.sin(a) * (R + 6);
      ctx.beginPath();
      ctx.arc(px, py, 3, 0, 2 * Math.PI);
      ctx.fillStyle = "#ffcc47";
      ctx.fill();
    }

    // Hub.
    const hub = ctx.createRadialGradient(cx - 6, cy - 6, 2, cx, cy, 26);
    hub.addColorStop(0, "#2a2f47");
    hub.addColorStop(1, "#10121d");
    ctx.beginPath();
    ctx.arc(cx, cy, 24, 0, 2 * Math.PI);
    ctx.fillStyle = hub;
    ctx.fill();
    ctx.lineWidth = 2;
    ctx.strokeStyle = "#3a4166";
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(cx, cy, 7, 0, 2 * Math.PI);
    ctx.fillStyle = "#8b5cff";
    ctx.fill();

    // Pointer (top), tilts with the ticking of the pegs.
    ctx.save();
    ctx.translate(cx, cy - R - 4);
    ctx.rotate(tilt);
    ctx.beginPath();
    ctx.moveTo(-15, -18);
    ctx.lineTo(15, -18);
    ctx.lineTo(0, 14);
    ctx.closePath();
    const pg = ctx.createLinearGradient(0, -18, 0, 14);
    pg.addColorStop(0, "#ffe08a");
    pg.addColorStop(1, "#ffcc47");
    ctx.fillStyle = pg;
    ctx.fill();
    ctx.lineWidth = 2;
    ctx.strokeStyle = "#0a0b12";
    ctx.stroke();
    ctx.restore();
  }

  function animateWheel(res: SpinResp): Promise<void> {
    const target = wheelFinalRot(res);
    const N = res.options.length;
    const seg = (2 * Math.PI) / N;
    const duration = 4800;
    let start = 0;
    let prevRot = 0;

    return new Promise((resolve) => {
      function frame(ts: number) {
        if (!start) start = ts;
        const t = Math.min(1, (ts - start) / duration);
        const rot = easeOutQuint(t) * target;

        // Peg-tick wobble on the pointer, scaled by angular velocity.
        const vel = Math.abs(rot - prevRot);
        prevRot = rot;
        const tilt =
          clamp(vel * 12, 0, 1) * 0.28 * Math.sin((rot / seg) * Math.PI * 2);

        drawWheel(res.options, rot, t >= 1 ? res.winner_index : -1, tilt);

        if (t < 1) rafId = requestAnimationFrame(frame);
        else resolve();
      }
      rafId = requestAnimationFrame(frame);
    });
  }

  /* ============================== RACE ============================== */

  interface RaceState {
    startX: number;
    finishX: number;
    laneH: number;
    finishTime: number[]; // ms per horse; winner is smallest
    amp: number[];
    freq: number[];
    phase: number[];
    bobPhase: number[];
    elapsed: number;
  }
  let raceState: RaceState | null = null;

  function buildRaceState(res: SpinResp): RaceState {
    const N = res.options.length;
    const rng = lcg(res.seed + 3);
    const laneH = H / N;
    const winT = 3800;

    // Winner finishes first; give the runner-up a near photo-finish gap and
    // spread the rest out for a believable field.
    const finishTime: number[] = [];
    let secondAssigned = false;
    for (let i = 0; i < N; i++) {
      if (i === res.winner_index) finishTime[i] = winT;
      else if (!secondAssigned) {
        finishTime[i] = winT + 120 + rng() * 260; // photo-finish rival
        secondAssigned = true;
      } else finishTime[i] = winT + 500 + rng() * 2600;
    }

    return {
      startX: 96,
      finishX: W - 46,
      laneH,
      finishTime,
      amp: res.options.map(() => 0.05 + rng() * 0.09),
      freq: res.options.map(() => 3 + rng() * 5),
      phase: res.options.map(() => rng() * Math.PI * 2),
      bobPhase: res.options.map(() => rng() * Math.PI * 2),
      elapsed: 0,
    };
  }

  // Progress of a horse in [0,1]. Perturbation uses a sin-envelope that is zero
  // at start and finish, so surges/overtakes never change the exact arrival
  // time — the server winner always crosses first.
  function raceProgress(s: RaceState, i: number): number {
    const t = clamp(s.elapsed / s.finishTime[i], 0, 1);
    const eased = easeInOut(t);
    const env = Math.sin(Math.PI * t); // 0 at ends, 1 in the middle
    const surge = s.amp[i] * env * Math.sin(s.freq[i] * t * Math.PI * 2 + s.phase[i]);
    return t >= 1 ? 1 : clamp(eased + surge, 0, 0.999);
  }

  function drawRaceFrame(res: SpinResp, s: RaceState, finished: boolean) {
    const N = res.options.length;
    ctx.clearRect(0, 0, W, H);

    // Track lanes.
    for (let i = 0; i < N; i++) {
      const y = i * s.laneH;
      ctx.fillStyle = i % 2 ? "rgba(139,92,255,0.05)" : "rgba(53,224,208,0.04)";
      ctx.fillRect(0, y, W, s.laneH);
      ctx.strokeStyle = "rgba(58,65,102,0.5)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(W, y);
      ctx.stroke();
    }

    // Start line.
    ctx.strokeStyle = "rgba(122,130,168,0.55)";
    ctx.setLineDash([4, 4]);
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(s.startX, 0);
    ctx.lineTo(s.startX, H);
    ctx.stroke();
    ctx.setLineDash([]);

    // Finish line — checkered + dashed accent.
    const sq = 8;
    for (let y = 0; y < H; y += sq) {
      const row = Math.floor(y / sq);
      ctx.fillStyle = row % 2 ? "#0a0b12" : "#eef1ff";
      ctx.fillRect(s.finishX, y, 6, sq);
    }
    ctx.strokeStyle = "#ffcc47";
    ctx.setLineDash([7, 7]);
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(s.finishX + 9, 0);
    ctx.lineTo(s.finishX + 9, H);
    ctx.stroke();
    ctx.setLineDash([]);

    // Compute ranking for medal chips.
    const rank = res.options
      .map((_, i) => ({ i, p: raceProgress(s, i) }))
      .sort((a, b) => b.p - a.p);
    const place: number[] = [];
    rank.forEach((r, k) => (place[r.i] = k));

    for (let i = 0; i < N; i++) {
      const color = PALETTE[i % PALETTE.length];
      const p = raceProgress(s, i);
      const x = s.startX + p * (s.finishX - s.startX);
      const yBase = i * s.laneH + s.laneH / 2;
      const moving = p < 1;
      const bob = moving
        ? Math.sin(s.elapsed / 90 + s.bobPhase[i]) * Math.min(4, s.laneH * 0.12)
        : 0;
      const y = yBase + bob;

      // Name label + colour chip.
      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.arc(12, yBase, 5, 0, 2 * Math.PI);
      ctx.fill();
      ctx.fillStyle = "#eef1ff";
      ctx.font = `600 ${clamp(s.laneH * 0.34, 10, 14)}px Inter, system-ui`;
      ctx.textAlign = "left";
      ctx.fillText(trunc(res.options[i], N > 6 ? 8 : 12), 24, yBase + 4);

      // Dust trail behind a moving horse.
      if (moving && x > s.startX + 6) {
        ctx.fillStyle = "rgba(179,185,214,0.28)";
        for (let d = 1; d <= 3; d++) {
          const dr = 2.5 - d * 0.4;
          ctx.beginPath();
          ctx.arc(x - 16 - d * 7, y + 8, Math.max(0.5, dr), 0, 2 * Math.PI);
          ctx.fill();
        }
      }

      // Horse.
      ctx.font = `${clamp(s.laneH * 0.7, 18, 30)}px system-ui`;
      ctx.textAlign = "center";
      ctx.fillText("🐎", x, y + 9);

      // Leader crown once someone is clearly ahead.
      if (place[i] === 0 && p > 0.05 && !finished) {
        ctx.font = `${clamp(s.laneH * 0.32, 10, 14)}px system-ui`;
        ctx.fillText("👑", x, y - s.laneH * 0.34);
      }
    }

    if (finished) {
      // Flash the winning lane.
      const y = res.winner_index * s.laneH;
      ctx.fillStyle = "rgba(255,204,71,0.14)";
      ctx.fillRect(0, y, W, s.laneH);
      ctx.strokeStyle = "#ffcc47";
      ctx.lineWidth = 2;
      ctx.strokeRect(1, y + 1, W - 2, s.laneH - 2);
    }
  }

  function animateRace(res: SpinResp): Promise<void> {
    const s = buildRaceState(res);
    raceState = s;
    let start = 0;

    return new Promise((resolve) => {
      function frame(ts: number) {
        if (!start) start = ts;
        s.elapsed = ts - start;
        const winnerDone = s.elapsed >= s.finishTime[res.winner_index];
        drawRaceFrame(res, s, winnerDone);
        if (!winnerDone) rafId = requestAnimationFrame(frame);
        else resolve();
      }
      rafId = requestAnimationFrame(frame);
    });
  }

  /* ============================ CONFETTI ============================ */

  function launchConfetti(redraw: () => void) {
    const rng = lcg(Date.now() % 2147483647);
    const parts = Array.from({ length: 130 }, () => ({
      x: W * (0.2 + rng() * 0.6),
      y: -20 - rng() * H * 0.4,
      vx: (rng() - 0.5) * 4,
      vy: 2 + rng() * 4,
      size: 4 + rng() * 6,
      rot: rng() * Math.PI,
      vr: (rng() - 0.5) * 0.4,
      color: PALETTE[Math.floor(rng() * PALETTE.length)],
    }));
    const duration = 2600;
    let start = 0;

    function frame(ts: number) {
      if (!start) start = ts;
      const t = (ts - start) / duration;
      redraw();
      for (const p of parts) {
        p.x += p.vx;
        p.y += p.vy;
        p.vy += 0.12;
        p.vx *= 0.99;
        p.rot += p.vr;
        ctx.save();
        ctx.globalAlpha = clamp(1 - t, 0, 1);
        ctx.translate(p.x, p.y);
        ctx.rotate(p.rot);
        ctx.fillStyle = p.color;
        ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size * 0.6);
        ctx.restore();
      }
      if (t < 1) rafId = requestAnimationFrame(frame);
      else redraw();
    }
    rafId = requestAnimationFrame(frame);
  }

  /* ============================== IDLE ============================== */

  function startIdle() {
    stop();
    if (!ctx) return;
    if (mode === "roulette") {
      let rot = 0;
      const loop = () => {
        rot += 0.004;
        drawWheel(opts.length ? opts : ["—", "—"], rot, -1, 0);
        rafId = requestAnimationFrame(loop);
      };
      rafId = requestAnimationFrame(loop);
    } else {
      const preview = {
        options: opts.length ? opts : ["—", "—"],
        mode: "race",
        winner_index: -1,
        winner: "",
        seed: 1,
      } as SpinResp;
      const s = buildRaceState(preview);
      // Infinite finish times keep progress at 0 (horses stay at the gate)
      // while elapsed still advances so the gallop bob animates.
      s.finishTime = s.finishTime.map(() => Infinity);
      let t0 = 0;
      const loop = (ts: number) => {
        if (!t0) t0 = ts;
        s.elapsed = ts - t0;
        drawRaceFrame(preview, s, false);
        rafId = requestAnimationFrame(loop);
      };
      rafId = requestAnimationFrame(loop);
    }
  }

  /* ---- helpers ---- */

  // Lighten (amt>0) or darken (amt<0) a hex colour.
  function shade(hex: string, amt: number): string {
    const n = parseInt(hex.slice(1), 16);
    let r = (n >> 16) & 255,
      g = (n >> 8) & 255,
      b = n & 255;
    if (amt >= 0) {
      r += (255 - r) * amt;
      g += (255 - g) * amt;
      b += (255 - b) * amt;
    } else {
      r *= 1 + amt;
      g *= 1 + amt;
      b *= 1 + amt;
    }
    return `rgb(${r | 0},${g | 0},${b | 0})`;
  }

  onMount(() => {
    ctx = canvas.getContext("2d")!;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    canvas.width = W * dpr;
    canvas.height = H * dpr;
    ctx.scale(dpr, dpr);
    startIdle();
    loadHistory();
  });

  onDestroy(stop);

  // Restart the idle preview whenever the mode or options change (but never
  // interrupt an in-flight spin).
  $effect(() => {
    void mode;
    void opts;
    // Read spinning without tracking it, so finishing a spin (spinning→false)
    // does not re-run this effect and wipe the revealed winner / final frame.
    untrack(() => {
      if (!spinning) {
        winner = "";
        error = "";
        startIdle();
      }
    });
  });
</script>

<h2>🎲 Sorteio</h2>
<p class="muted">
  Roleta ou corrida de cavalos para decidir qualquer coisa. O resultado é
  sorteado no servidor de forma justa — a animação só mostra o desfecho.
</p>

<div class="layout">
  <div class="card stack controls">
    <div class="field">
      <span class="label">Opções (uma por linha)</span>
      <textarea bind:value={raw} rows="7" disabled={spinning}></textarea>
    </div>

    <div class="row">
      <button
        class="btn btn-block {mode === 'roulette' ? '' : 'btn-ghost'}"
        onclick={() => (mode = "roulette")}
        disabled={spinning}
      >
        🎡 Roleta
      </button>
      <button
        class="btn btn-block {mode === 'race' ? '' : 'btn-ghost'}"
        onclick={() => (mode = "race")}
        disabled={spinning}
      >
        🐎 Corrida
      </button>
    </div>

    {#if isAdmin}
      <button
        class="btn btn-block"
        onclick={spin}
        disabled={spinning || opts.length < 2}
      >
        {spinning ? "Sorteando…" : "Sortear!"}
      </button>
    {:else}
      <div class="empty small">
        🔒 Apenas admins podem rodar o sorteio.
      </div>
    {/if}

    {#if error}<p class="error">{error}</p>{/if}

    {#if winner}
      <div class="reveal pop">
        <div class="dim small">🏆 Vencedor</div>
        <div class="badge badge-accent winner-badge">{winner}</div>
      </div>
    {/if}

    <div class="dim tiny">{opts.length} opções</div>
  </div>

  <div class="card stage center">
    <canvas bind:this={canvas}></canvas>
  </div>
</div>

<section class="history stack">
  <h3>🗂️ Histórico de sorteios</h3>
  {#if history.length === 0}
    <div class="empty">Nenhum sorteio ainda</div>
  {:else}
    <div class="history-list stack">
      {#each history as h (h.id)}
        <div class="card hist-card stack">
          <div class="row hist-head">
            <span class="hist-mode">{h.mode === "roulette" ? "🎡" : "🐎"}</span>
            <span class="badge badge-accent hist-winner">{h.winner}</span>
          </div>
          <div class="muted small hist-opts">{h.options.join(", ")}</div>
          <div class="row hist-meta small muted">
            <span>👤 {h.created_by}</span>
            <span>🕒 {new Date(h.created_at).toLocaleString("pt-BR")}</span>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 1rem;
    align-items: start;
  }
  @media (max-width: 720px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
  .stage {
    padding: 0.6rem;
    overflow: hidden;
  }
  canvas {
    display: block;
    width: 100%;
    height: auto;
    max-width: 640px;
    border-radius: var(--r);
  }
  .reveal {
    text-align: center;
    margin-top: 0.25rem;
  }
  .winner-badge {
    display: inline-block;
    margin-top: 0.35rem;
    font-size: 1.15rem;
    font-weight: 800;
    padding: 0.45rem 1rem;
    border-radius: var(--r-pill);
    box-shadow: var(--accent-glow);
    max-width: 100%;
  }
  .history {
    margin-top: 1.25rem;
  }
  .history-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 0.75rem;
  }
  .hist-card {
    padding: 0.75rem;
    gap: 0.4rem;
  }
  .hist-head {
    align-items: center;
    gap: 0.5rem;
  }
  .hist-mode {
    font-size: 1.25rem;
    line-height: 1;
  }
  .hist-winner {
    font-weight: 700;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hist-opts {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hist-meta {
    flex-wrap: wrap;
    gap: 0.25rem 0.75rem;
    justify-content: space-between;
  }
</style>
