<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { get, del, upload } from "../lib/api";
  import { user } from "../lib/auth";

  interface Item {
    id: string;
    name: string;
    unit: string;
    unit_cents: number;
    quantity: number;
    total_cents: number;
    item_url: string;
    receipt_url: string;
    paid_by_id: string;
    paid_by_name: string;
    created_at: string;
  }
  interface Balance {
    user_id: string;
    name: string;
    paid_cents: number;
    balance_cents: number;
  }
  interface ShoppingData {
    items: Item[];
    total_cents: number;
    user_count: number;
    share_cents: number;
    balances: Balance[];
  }

  let data = $state<ShoppingData | null>(null);
  let error = $state("");

  // form state
  let name = $state("");
  let value = $state("");
  let quantity = $state(1);
  let unit = $state("un");
  let itemFile = $state<File | null>(null);
  let itemFileName = $state("");
  let itemFileInput: HTMLInputElement;
  let receiptFile = $state<File | null>(null);
  let receiptFileName = $state("");
  let receiptFileInput: HTMLInputElement;
  let saving = $state(false);
  let formError = $state("");

  // drawer
  let open = $state(false);

  // lightbox
  let lightbox = $state<string | null>(null);

  let poll: ReturnType<typeof setInterval>;

  const canSubmit = $derived(
    name.trim() !== "" &&
      value.trim() !== "" &&
      !!itemFile &&
      !!receiptFile &&
      !saving,
  );

  function brl(cents: number): string {
    return (cents / 100).toLocaleString("pt-BR", {
      style: "currency",
      currency: "BRL",
    });
  }

  async function load() {
    try {
      data = await get<ShoppingData>("/shopping");
      error = "";
    } catch (e) {
      error = (e as Error).message;
    }
  }

  function onItemFile(e: Event) {
    const input = e.target as HTMLInputElement;
    const f = input.files?.[0] ?? null;
    itemFile = f;
    itemFileName = f ? f.name : "";
  }

  function onReceiptFile(e: Event) {
    const input = e.target as HTMLInputElement;
    const f = input.files?.[0] ?? null;
    receiptFile = f;
    receiptFileName = f ? f.name : "";
  }

  function resetForm() {
    name = "";
    value = "";
    quantity = 1;
    unit = "un";
    itemFile = null;
    itemFileName = "";
    receiptFile = null;
    receiptFileName = "";
    if (itemFileInput) itemFileInput.value = "";
    if (receiptFileInput) receiptFileInput.value = "";
  }

  async function add(e: Event) {
    e.preventDefault();
    if (!canSubmit || !itemFile || !receiptFile) return;
    saving = true;
    formError = "";
    try {
      const form = new FormData();
      form.append("name", name.trim());
      form.append("value", value.trim());
      form.append("quantity", String(quantity || 1));
      form.append("unit", unit);
      form.append("item_image", itemFile);
      form.append("receipt", receiptFile);
      data = await upload<ShoppingData>("/shopping", form);
      resetForm();
      open = false;
    } catch (err) {
      formError = (err as Error).message;
    } finally {
      saving = false;
    }
  }

  async function remove(item: Item) {
    if (!confirm(`Remover "${item.name}"?`)) return;
    try {
      await del(`/shopping/${item.id}`);
      await load();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  function canDelete(item: Item): boolean {
    return !!$user && ($user.is_admin || item.paid_by_id === $user.id);
  }

  const isImage = (url: string) => !/\.pdf($|\?)/i.test(url);

  function openMedia(url: string) {
    if (!url) return;
    if (isImage(url)) lightbox = url;
    else window.open(url, "_blank");
  }

  onMount(() => {
    load();
    poll = setInterval(load, 5000);
  });
  onDestroy(() => clearInterval(poll));
</script>

<div class="head">
  <div class="row" style="gap:0.75rem;align-items:flex-start;">
    <div class="col" style="min-width:0;gap:0.2rem;">
      <h2 style="margin:0;">🛒 Lista de Compras</h2>
      <p class="muted" style="margin:0;">
        Todo mundo adiciona itens (com comprovante). O custo é dividido
        igualmente entre todos — veja quem recebe e quem deve.
      </p>
    </div>
    <div class="spacer"></div>
    <button class="btn add-btn" onclick={() => (open = true)}>
      + Adicionar compra
    </button>
  </div>
</div>

{#if error}<p class="error">{error}</p>{/if}

<!-- SUMMARY -->
{#if data}
  <div class="summary card pop">
    <div class="sum-cell">
      <span class="sum-label">Total gasto</span>
      <span class="sum-val total">{brl(data.total_cents)}</span>
    </div>
    <div class="sum-cell">
      <span class="sum-label">Cota por pessoa</span>
      <span class="sum-val share">{brl(data.share_cents)}</span>
    </div>
    <div class="sum-cell">
      <span class="sum-label">Pessoas</span>
      <span class="sum-val people">{data.user_count}</span>
    </div>
  </div>

  <!-- BALANCES -->
  <h3 class="section-title">Saldos</h3>
  {#if data.balances.length === 0}
    <div class="empty">Ninguém participando ainda.</div>
  {:else}
    <div class="card balances stack">
      {#each data.balances as b (b.user_id)}
        <div class="bal-row">
          <div class="col" style="gap:0.1rem;min-width:0;">
            <span class="strong truncate">
              {b.name}
              {#if $user && b.user_id === $user.id}
                <span class="badge badge-cyan" style="margin-left:0.3rem;">você</span>
              {/if}
            </span>
            <span class="tiny muted">pagou {brl(b.paid_cents)}</span>
          </div>
          <div class="spacer"></div>
          <div class="bal-amount">
            {#if b.balance_cents > 0}
              <span class="bal-val ok">+{brl(b.balance_cents)}</span>
              <span class="tiny" style="color:var(--ok);">recebe</span>
            {:else if b.balance_cents < 0}
              <span class="bal-val danger">−{brl(Math.abs(b.balance_cents))}</span>
              <span class="tiny" style="color:var(--danger);">deve</span>
            {:else}
              <span class="bal-val muted">{brl(0)}</span>
              <span class="tiny muted">quitado</span>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <!-- ITEMS -->
  <h3 class="section-title">Itens ({data.items.length})</h3>
  {#if data.items.length === 0}
    <div class="empty">
      <div style="font-size:2rem;margin-bottom:0.5rem;">🧾</div>
      <p class="strong" style="color:var(--text-dim);margin:0;">Nenhum item ainda</p>
      <p class="small" style="margin:0.4rem 0 0;">
        Toque em “+ Adicionar compra” para lançar o primeiro item com o comprovante.
      </p>
    </div>
  {:else}
    <div class="grid items">
      {#each data.items as item (item.id)}
        <div class="card card-hover item">
          <div class="thumbs">
            <div class="thumbcell">
              {#if item.item_url}
                <button
                  type="button"
                  class="thumbwrap"
                  title="Ver foto do item"
                  onclick={() => openMedia(item.item_url)}
                >
                  {#if isImage(item.item_url)}
                    <img class="thumb pic" src={item.item_url} alt="Foto de {item.name}" />
                  {:else}
                    <span class="thumb pic center pdf">📄 PDF</span>
                  {/if}
                </button>
              {:else}
                <span class="thumb pic center pdf">—</span>
              {/if}
              <span class="thumblabel">Item</span>
            </div>
            <div class="thumbcell">
              {#if item.receipt_url}
                <button
                  type="button"
                  class="thumbwrap"
                  title="Ver comprovante"
                  onclick={() => openMedia(item.receipt_url)}
                >
                  {#if isImage(item.receipt_url)}
                    <img class="thumb pic" src={item.receipt_url} alt="Comprovante de {item.name}" />
                  {:else}
                    <span class="thumb pic center pdf">📄 PDF</span>
                  {/if}
                </button>
              {:else}
                <span class="thumb pic center pdf">—</span>
              {/if}
              <span class="thumblabel">Comprovante</span>
            </div>
          </div>

          <div class="info">
            <span class="iname strong truncate">{item.name}</span>
            <span class="calc dim small">
              {item.quantity} {item.unit} × {brl(item.unit_cents)} =
              <span class="strong" style="color:var(--text);">{brl(item.total_cents)}</span>
            </span>
            <span class="tiny muted">pago por {item.paid_by_name}</span>
          </div>

          {#if canDelete(item)}
            <button class="btn btn-danger btn-sm rm" title="Remover" onclick={() => remove(item)}>🗑</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
{/if}

<!-- BACKDROP -->
{#if open}
  <div
    class="sh-backdrop"
    role="button"
    tabindex="-1"
    aria-label="Fechar"
    onclick={() => (open = false)}
    onkeydown={(e) => e.key === "Escape" && (open = false)}
  ></div>
{/if}

<!-- ADD DRAWER -->
<aside class="sh-drawer" class:open>
  <header class="sh-head">
    <strong>🛒 Adicionar compra</strong>
    <span class="spacer"></span>
    <button class="btn btn-ghost btn-sm" onclick={() => (open = false)}>✕</button>
  </header>

  <div class="sh-body">
    <div class="row" style="gap:0.4rem;margin-bottom:0.6rem;">
      <span class="badge badge-accent">Novo item</span>
      <span class="small dim">Foto do item e comprovante obrigatórios</span>
    </div>
    <form onsubmit={add} class="stack">
    <div class="field">
      <span class="label">Nome do item</span>
      <input bind:value={name} placeholder="Ex: Refrigerante, gelo, carvão…" />
    </div>
    <div class="row row-wrap grow-fields">
      <div class="field grow">
        <span class="label">Valor unitário (R$)</span>
        <input
          bind:value
          inputmode="decimal"
          placeholder="Ex: 150 ou 150,50"
        />
      </div>
      <div class="field qty">
        <span class="label">Quantidade</span>
        <div class="row qty-row">
          <input type="number" min="1" step="1" bind:value={quantity} />
          <select bind:value={unit} class="unit-select">
            <option value="un">un</option>
            <option value="kg">kg</option>
            <option value="g">g</option>
            <option value="L">L</option>
            <option value="ml">ml</option>
            <option value="cx">cx</option>
            <option value="pct">pct</option>
            <option value="dz">dz</option>
          </select>
        </div>
      </div>
    </div>

    <div class="field">
      <span class="label">Foto do item (imagem ou PDF)</span>
      <div class="filerow">
        <button
          type="button"
          class="btn btn-ghost btn-sm"
          onclick={() => itemFileInput?.click()}
        >
          📷 Escolher foto
        </button>
        <span class="filename truncate" class:has={!!itemFileName}>
          {itemFileName || "Nenhum arquivo selecionado"}
        </span>
        <input
          bind:this={itemFileInput}
          type="file"
          accept="image/*,.pdf"
          onchange={onItemFile}
          hidden
        />
      </div>
    </div>

    <div class="field">
      <span class="label">Comprovante (imagem ou PDF)</span>
      <div class="filerow">
        <button
          type="button"
          class="btn btn-ghost btn-sm"
          onclick={() => receiptFileInput?.click()}
        >
          📎 Escolher comprovante
        </button>
        <span class="filename truncate" class:has={!!receiptFileName}>
          {receiptFileName || "Nenhum arquivo selecionado"}
        </span>
        <input
          bind:this={receiptFileInput}
          type="file"
          accept="image/*,.pdf"
          onchange={onReceiptFile}
          hidden
        />
      </div>
    </div>

      {#if formError}<p class="error">{formError}</p>{/if}

      <button class="btn btn-block" type="submit" disabled={!canSubmit}>
        {saving ? "Enviando…" : "Adicionar"}
      </button>
    </form>
  </div>
</aside>

<!-- LIGHTBOX -->
{#if lightbox}
  <div
    class="lightbox"
    role="button"
    tabindex="0"
    onclick={() => (lightbox = null)}
    onkeydown={(e) => e.key === "Escape" && (lightbox = null)}
  >
    <img src={lightbox} alt="Comprovante" />
    <a class="open-tab" href={lightbox} target="_blank" rel="noopener" onclick={(e) => e.stopPropagation()}>
      Abrir em nova aba ↗
    </a>
  </div>
{/if}

<style>
  .head {
    margin-bottom: 1.1rem;
  }
  .head p {
    margin: 0.2rem 0 0;
  }

  .add-btn {
    flex-shrink: 0;
    white-space: nowrap;
  }

  /* Drawer */
  .sh-backdrop {
    position: fixed;
    inset: 0;
    z-index: 70;
    background: rgba(0, 0, 0, 0.45);
  }
  .sh-drawer {
    position: fixed;
    top: 0;
    right: 0;
    height: 100vh;
    width: 440px;
    max-width: 92vw;
    z-index: 80;
    background: var(--bg-elev);
    border-left: 1px solid var(--border);
    transform: translateX(105%);
    transition: transform 0.25s ease;
    display: flex;
    flex-direction: column;
  }
  .sh-drawer.open {
    transform: translateX(0);
  }
  .sh-head {
    display: flex;
    align-items: center;
    padding: 0.7rem 0.9rem;
    border-bottom: 1px solid var(--border);
    min-height: 58px;
  }
  .sh-body {
    flex: 1;
    overflow-y: auto;
    padding: 0.9rem;
  }

  .grow-fields {
    align-items: flex-end;
  }
  .grow {
    flex: 1;
    min-width: 160px;
  }
  .qty {
    width: 200px;
  }
  .qty-row {
    gap: 0.5rem;
  }
  .qty-row input[type="number"] {
    flex: 1;
    min-width: 0;
  }
  .unit-select {
    width: 90px;
    flex-shrink: 0;
  }
  .field + .field,
  .field {
    margin-top: 0;
  }

  .filerow {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .filename {
    color: var(--muted);
    font-size: 0.85rem;
    min-width: 0;
  }
  .filename.has {
    color: var(--cyan);
  }

  /* Summary */
  .summary {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.5rem;
    margin-bottom: 1.4rem;
    border-color: var(--border-strong);
    background:
      radial-gradient(600px 200px at 0% 0%, rgba(139, 92, 255, 0.18), transparent 70%),
      radial-gradient(500px 200px at 100% 100%, rgba(255, 77, 141, 0.12), transparent 70%),
      linear-gradient(180deg, var(--surface), var(--bg-elev));
    box-shadow: var(--accent-glow);
  }
  .sum-cell {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    padding: 0.6rem 0.4rem;
    text-align: center;
  }
  .sum-cell + .sum-cell {
    border-left: 1px solid var(--border);
  }
  .sum-label {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
    font-weight: 700;
  }
  .sum-val {
    font-size: 1.55rem;
    font-weight: 850;
    letter-spacing: -0.02em;
    line-height: 1.1;
  }
  .sum-val.total {
    background: linear-gradient(90deg, var(--accent-2), var(--accent));
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }
  .sum-val.share {
    color: var(--gold);
  }
  .sum-val.people {
    color: var(--cyan);
  }

  .section-title {
    margin: 0 0 0.6rem;
  }

  /* Balances */
  .balances {
    margin-bottom: 1.4rem;
  }
  .bal-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .bal-row + .bal-row {
    padding-top: 0.7rem;
    border-top: 1px solid var(--border);
    margin-top: 0.7rem;
  }
  .bal-amount {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 0.1rem;
  }
  .bal-val {
    font-size: 1.1rem;
    font-weight: 800;
    letter-spacing: -0.01em;
  }
  .bal-val.ok {
    color: var(--ok);
  }
  .bal-val.danger {
    color: var(--danger);
  }

  /* Items */
  .items {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
  .item {
    display: flex;
    align-items: center;
    gap: 0.85rem;
    padding: 0.8rem;
  }
  .thumbs {
    display: flex;
    gap: 0.5rem;
    flex-shrink: 0;
  }
  .thumbcell {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.2rem;
  }
  .thumblabel {
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    font-weight: 700;
  }
  .thumbwrap {
    padding: 0;
    border: none;
    background: none;
    cursor: pointer;
    flex-shrink: 0;
    line-height: 0;
  }
  .pic {
    width: 56px;
    height: 56px;
    transition: filter 0.15s ease;
  }
  .thumbwrap:hover .pic {
    filter: brightness(1.15);
  }
  .pdf {
    font-size: 0.75rem;
    color: var(--muted);
    font-weight: 700;
  }
  .info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
    flex: 1;
  }
  .iname {
    letter-spacing: -0.01em;
  }
  .rm {
    align-self: flex-start;
    flex-shrink: 0;
  }

  /* Lightbox */
  .lightbox {
    position: fixed;
    inset: 0;
    z-index: 50;
    background: rgba(4, 5, 10, 0.85);
    backdrop-filter: blur(6px);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    padding: 2rem;
    cursor: zoom-out;
  }
  .lightbox img {
    max-width: min(92vw, 900px);
    max-height: 80vh;
    object-fit: contain;
    border-radius: var(--r);
    box-shadow: var(--shadow);
  }
  .open-tab {
    color: var(--text);
    text-decoration: none;
    background: var(--surface-2);
    border: 1px solid var(--border);
    padding: 0.5rem 1rem;
    border-radius: var(--r-sm);
    font-weight: 650;
    font-size: 0.9rem;
  }
  .open-tab:hover {
    background: var(--surface-3);
  }

  @media (max-width: 640px) {
    .summary {
      grid-template-columns: 1fr;
    }
    .sum-cell + .sum-cell {
      border-left: none;
      border-top: 1px solid var(--border);
    }
    .qty {
      width: 100%;
    }
  }
</style>
