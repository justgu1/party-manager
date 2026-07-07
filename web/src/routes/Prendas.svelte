<script lang="ts">
  import { onMount } from "svelte";
  import { get, post, put, del } from "../lib/api";
  import { user } from "../lib/auth";

  interface Prenda {
    id: string;
    title: string;
    description: string;
    created_at: string;
  }

  let prendas = $state<Prenda[]>([]);
  let title = $state("");
  let description = $state("");
  let editingId = $state<string | null>(null);
  let error = $state("");
  let loading = $state(false);
  let saving = $state(false);

  const isAdmin = $derived($user?.is_admin ?? false);

  async function load() {
    loading = true;
    error = "";
    try {
      prendas = await get<Prenda[]>("/prendas");
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  function reset() {
    title = "";
    description = "";
    editingId = null;
  }

  async function submit(e: Event) {
    e.preventDefault();
    error = "";
    if (!title.trim()) {
      error = "Informe um título para a prenda.";
      return;
    }
    saving = true;
    try {
      const payload = { title: title.trim(), description: description.trim() };
      if (editingId) {
        await put(`/prendas/${editingId}`, payload);
      } else {
        await post("/prendas", payload);
      }
      reset();
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      saving = false;
    }
  }

  function edit(p: Prenda) {
    editingId = p.id;
    title = p.title;
    description = p.description;
    error = "";
  }

  async function remove(p: Prenda) {
    if (!confirm(`Remover a prenda "${p.title}"?`)) return;
    error = "";
    try {
      await del(`/prendas/${p.id}`);
      if (editingId === p.id) reset();
      await load();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  onMount(load);
</script>

<h2>🎭 Prendas</h2>
<p class="muted">Catálogo de prendas. Cada música só toca quando a prenda escolhida for cumprida.</p>

{#if !isAdmin}
  <div class="empty">Só administradores gerenciam as prendas.</div>
{:else}
  <form onsubmit={submit} class="card stack" style="margin-bottom:1.25rem;">
    <h3 style="margin:0;">{editingId ? "Editar prenda" : "Nova prenda"}</h3>
    <div class="field">
      <span class="label">Título</span>
      <input bind:value={title} placeholder="Ex: pagar uma rodada" required />
    </div>
    <div class="field">
      <span class="label">Descrição (opcional)</span>
      <input bind:value={description} placeholder="Detalhes da prenda" />
    </div>

    {#if error}<p class="error">{error}</p>{/if}

    <div class="row">
      <button class="btn" type="submit" disabled={saving}>
        {saving ? "..." : editingId ? "Salvar" : "Adicionar prenda"}
      </button>
      {#if editingId}
        <button type="button" class="btn btn-ghost" onclick={reset} disabled={saving}>
          Cancelar
        </button>
      {/if}
    </div>
  </form>

  {#if loading}
    <p class="muted">Carregando...</p>
  {:else if prendas.length === 0}
    <div class="empty">Nenhuma prenda cadastrada ainda. Crie a primeira acima. 🎉</div>
  {:else}
    <div class="grid">
      {#each prendas as p (p.id)}
        <div class="card card-hover row">
          <div class="spacer">
            <div class="strong">{p.title}</div>
            {#if p.description}<div class="muted small">{p.description}</div>{/if}
          </div>
          <button class="btn btn-ghost btn-sm" onclick={() => edit(p)}>Editar</button>
          <button class="btn btn-danger btn-sm" onclick={() => remove(p)}>Excluir</button>
        </div>
      {/each}
    </div>
  {/if}
{/if}
