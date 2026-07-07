<script lang="ts">
  import { post } from "../lib/api";
  import { setSession, type User } from "../lib/auth";

  let mode = $state<"login" | "register" | "forgot">("login");
  let email = $state("");
  let password = $state("");
  let name = $state("");
  let error = $state("");
  let notice = $state("");
  let loading = $state(false);

  interface AuthResponse {
    token: string;
    user: User;
  }

  async function submit(e: Event) {
    e.preventDefault();
    error = "";
    notice = "";
    loading = true;
    try {
      if (mode === "forgot") {
        await post("/auth/reset-request", { email });
        notice = "Se o email existir, enviamos um link para definir a senha.";
      } else {
        const path = mode === "login" ? "/auth/login" : "/auth/register";
        const res = await post<AuthResponse>(path, { email, password, name });
        setSession(res.token, res.user);
      }
    } catch (err) {
      error = (err as Error).message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="wrap">
  <div class="card pop">
    <h1 style="margin-top:0;">🎉 help-party</h1>
    <p class="muted">Organize a festa de fim de ano com a galera.</p>

    <form onsubmit={submit} class="stack">
      {#if mode === "register"}
        <div class="field">
          <span class="label">Nome</span>
          <input bind:value={name} placeholder="Seu nome" />
        </div>
      {/if}
      <div class="field">
        <span class="label">Email</span>
        <input type="email" bind:value={email} placeholder="voce@email.com" required />
      </div>
      {#if mode !== "forgot"}
        <div class="field">
          <span class="label">Senha</span>
          <input type="password" bind:value={password} placeholder="••••••••" required />
        </div>
      {/if}

      {#if error}<p class="error">{error}</p>{/if}
      {#if notice}<p class="dim small">{notice}</p>{/if}

      <button class="btn btn-block" type="submit" disabled={loading}>
        {loading
          ? "..."
          : mode === "login"
            ? "Entrar"
            : mode === "register"
              ? "Criar conta"
              : "Enviar link"}
      </button>
    </form>

    <div class="foot muted small">
      {#if mode === "login"}
        <button class="link-btn" onclick={() => (mode = "register")}>Criar conta</button>
        <span>·</span>
        <button class="link-btn" onclick={() => (mode = "forgot")}>Esqueci a senha</button>
      {:else}
        <button class="link-btn" onclick={() => (mode = "login")}>Voltar ao login</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .wrap {
    max-width: 420px;
    margin: 0 auto;
    padding: 12vh 1.25rem 2rem;
  }
  .foot {
    display: flex;
    gap: 0.5rem;
    justify-content: center;
    margin-top: 1rem;
  }
</style>
