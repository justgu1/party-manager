<script lang="ts">
  import { post } from "../lib/api";

  // Token comes from the email link: #/reset?token=UUID
  function tokenFromHash(): string {
    const q = window.location.hash.split("?")[1] ?? "";
    return new URLSearchParams(q).get("token") ?? "";
  }

  let token = $state(tokenFromHash());
  let password = $state("");
  let confirm = $state("");
  let done = $state(false);
  let error = $state("");
  let loading = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    error = "";
    if (password.length < 6) {
      error = "A senha deve ter ao menos 6 caracteres.";
      return;
    }
    if (password !== confirm) {
      error = "As senhas não conferem.";
      return;
    }
    loading = true;
    try {
      await post("/auth/reset", { token, password });
      done = true;
    } catch (err) {
      error = (err as Error).message;
    } finally {
      loading = false;
    }
  }
</script>

<div style="max-width: 420px; margin: 8vh auto 0;">
  <div class="card">
    <h1 style="margin-top:0;">Definir senha</h1>
    {#if done}
      <p class="dim">Senha definida com sucesso! 🎉</p>
      <a class="btn btn-block" href="#/" onclick={() => location.reload()}>Ir para o login</a>
    {:else}
      <p class="muted small">Escolha sua senha para acessar o help-party.</p>
      <form onsubmit={submit} class="stack">
        {#if !tokenFromHash()}
          <div class="field">
            <span class="label">Token</span>
            <input bind:value={token} placeholder="cole o token do email" />
          </div>
        {/if}
        <div class="field">
          <span class="label">Nova senha</span>
          <input type="password" bind:value={password} placeholder="••••••••" />
        </div>
        <div class="field">
          <span class="label">Confirmar senha</span>
          <input type="password" bind:value={confirm} placeholder="••••••••" />
        </div>
        {#if error}<p class="error">{error}</p>{/if}
        <button class="btn btn-block" type="submit" disabled={loading}>
          {loading ? "..." : "Salvar senha"}
        </button>
      </form>
    {/if}
  </div>
</div>
