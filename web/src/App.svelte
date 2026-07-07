<script lang="ts">
  import Router, { link } from "svelte-spa-router";
  import active from "svelte-spa-router/active";
  import { user, clearSession } from "./lib/auth";
  import { playerOpen } from "./lib/player";
  import Login from "./routes/Login.svelte";
  import Reset from "./routes/Reset.svelte";
  import Rentals from "./routes/Rentals.svelte";
  import Jukebox from "./routes/Jukebox.svelte";
  import Prendas from "./routes/Prendas.svelte";
  import Shopping from "./routes/Shopping.svelte";
  import Game from "./routes/Game.svelte";
  import Users from "./routes/Users.svelte";
  import PlayerDrawer from "./routes/PlayerDrawer.svelte";

  const routes = {
    "/": Rentals,
    "/jukebox": Jukebox,
    "/prendas": Prendas,
    "/compras": Shopping,
    "/game": Game,
    "/usuarios": Users,
    "/reset": Reset,
  };

  // The password-reset link is opened while logged out, so honour it first.
  let hash = $state(window.location.hash);
  $effect(() => {
    const onHash = () => (hash = window.location.hash);
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  });
  const isReset = $derived(hash.startsWith("#/reset"));
</script>

{#if $user}
  <nav class="topbar">
    <span class="brand">🎉 help-party</span>
    <a href="/" use:link use:active={{ path: "/", className: "active" }}>Lugares</a>
    <a href="/jukebox" use:link use:active>Jukebox</a>
    <a href="/compras" use:link use:active>Compras</a>
    <a href="/game" use:link use:active>Sorteio</a>
    {#if $user.is_admin}
      <a href="/prendas" use:link use:active>Prendas</a>
      <a href="/usuarios" use:link use:active>Usuários</a>
    {/if}
    <span class="spacer"></span>
    <button class="btn btn-ghost btn-sm" onclick={() => ($playerOpen = !$playerOpen)} title="Player">
      🎵 Player
    </button>
    {#if $user.is_admin}<span class="admin-tag">ADMIN</span>{/if}
    <span class="muted small">{$user.name}</span>
    <button class="btn btn-ghost btn-sm" onclick={clearSession}>Sair</button>
  </nav>
  <main class="container">
    <Router {routes} />
  </main>
  <!-- Mounted outside the router so it keeps playing across navigation. -->
  <PlayerDrawer />
{:else if isReset}
  <main class="container"><Reset /></main>
{:else}
  <Login />
{/if}
