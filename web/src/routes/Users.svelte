<script lang="ts">
  import { onMount } from "svelte";
  import { get, post, put, del } from "../lib/api";
  import { user } from "../lib/auth";

  interface AppUser {
    id: string;
    email: string;
    name: string;
    is_admin: boolean;
  }

  let users = $state<AppUser[]>([]);
  let email = $state("");
  let name = $state("");
  let isAdminFlag = $state(false);
  let editingId = $state<string | null>(null);
  let error = $state("");
  let notice = $state("");
  let loading = $state(false);
  let saving = $state(false);
  let busyId = $state<string | null>(null);

  const isAdmin = $derived($user?.is_admin ?? false);

  async function load() {
    loading = true;
    error = "";
    try {
      users = await get<AppUser[]>("/users");
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  function reset() {
    email = "";
    name = "";
    isAdminFlag = false;
    editingId = null;
  }

  async function submit(e: Event) {
    e.preventDefault();
    error = "";
    notice = "";
    if (!editingId && !email.trim()) {
      error = "Informe um email.";
      return;
    }
    saving = true;
    try {
      if (editingId) {
        await put(`/users/${editingId}`, {
          name: name.trim(),
          is_admin: isAdminFlag,
        });
      } else {
        await post("/users", {
          email: email.trim(),
          name: name.trim(),
          is_admin: isAdminFlag,
        });
        notice = "Usuário criado — enviamos um link para definir a senha.";
      }
      reset();
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      saving = false;
    }
  }

  function edit(u: AppUser) {
    editingId = u.id;
    email = u.email;
    name = u.name;
    isAdminFlag = u.is_admin;
    error = "";
    notice = "";
  }

  async function resetPassword(u: AppUser) {
    error = "";
    notice = "";
    busyId = u.id;
    try {
      await post(`/users/${u.id}/reset`);
      notice = `Link de definição de senha reenviado para ${u.email}.`;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busyId = null;
    }
  }

  async function remove(u: AppUser) {
    if (!confirm(`Excluir o usuário ${u.name || u.email}?`)) return;
    error = "";
    notice = "";
    busyId = u.id;
    try {
      await del(`/users/${u.id}`);
      if (editingId === u.id) reset();
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busyId = null;
    }
  }

  onMount(() => {
    if (isAdmin) load();
  });
</script>

<h2>👥 Usuários</h2>
<p class="muted">Gerencie quem tem acesso ao help-party e quem é administrador.</p>

{#if !isAdmin}
  <div class="empty">Apenas administradores.</div>
{:else}
  <form onsubmit={submit} class="card stack" style="margin-bottom:1.25rem;">
    <h3 style="margin:0;">{editingId ? "Editar usuário" : "Novo usuário"}</h3>

    <div class="field">
      <span class="label">Email</span>
      <input
        type="email"
        bind:value={email}
        placeholder="pessoa@exemplo.com"
        disabled={!!editingId}
        required={!editingId}
      />
      {#if editingId}<span class="muted tiny">O email não pode ser alterado.</span>{/if}
    </div>

    <div class="field">
      <span class="label">Nome (opcional)</span>
      <input bind:value={name} placeholder="Nome da pessoa" />
    </div>

    <label class="row" style="cursor:pointer;">
      <input
        type="checkbox"
        bind:checked={isAdminFlag}
        style="width:auto;flex-shrink:0;"
      />
      <span class="dim small">Administrador</span>
    </label>

    {#if error}<p class="error">{error}</p>{/if}

    <div class="row">
      <button class="btn" type="submit" disabled={saving || (!editingId && !email.trim())}>
        {saving ? "..." : editingId ? "Salvar" : "Criar usuário"}
      </button>
      {#if editingId}
        <button type="button" class="btn btn-ghost" onclick={reset} disabled={saving}>
          Cancelar
        </button>
      {/if}
    </div>

    {#if notice}<p class="dim small">{notice}</p>{/if}
  </form>

  {#if loading}
    <p class="muted">Carregando...</p>
  {:else if users.length === 0}
    <div class="empty">Nenhum usuário cadastrado ainda.</div>
  {:else}
    <div class="grid">
      {#each users as u (u.id)}
        <div class="card card-hover row row-wrap">
          <div class="spacer col" style="min-width:0;">
            <div class="row" style="gap:0.4rem;">
              <span class="strong truncate">{u.name || "(sem nome)"}</span>
              {#if u.is_admin}<span class="badge badge-warn">ADMIN</span>{/if}
            </div>
            <div class="muted small truncate">{u.email}</div>
          </div>
          <button class="btn btn-ghost btn-sm" onclick={() => edit(u)}>Editar</button>
          <button
            class="btn btn-outline btn-sm"
            onclick={() => resetPassword(u)}
            disabled={busyId === u.id}
          >
            Reenviar senha
          </button>
          <button
            class="btn btn-danger btn-sm"
            onclick={() => remove(u)}
            disabled={busyId === u.id || $user?.id === u.id}
            title={$user?.id === u.id ? "Você não pode excluir a si mesmo" : ""}
          >
            Excluir
          </button>
        </div>
      {/each}
    </div>
  {/if}
{/if}
