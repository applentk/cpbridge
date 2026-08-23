<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { AuthResponse } from '@cp-hub/contracts';
  import { LogIn, AlertCircle } from 'lucide-svelte';

  let emailOrUsername = '';
  let password = '';
  let error = '';
  let loading = false;

  async function handleSubmit() {
    error = '';
    loading = true;
    try {
      const res = await api.post<AuthResponse>('/auth/login', {
        emailOrUsername,
        password
      });
      auth.setAuth(res.user, res.token);
      goto('/dashboard');
    } catch (err: any) {
      error = err.message || 'Login failed';
    } finally {
      loading = false;
    }
  }
</script>

<div class="max-w-md mx-auto py-12">
  <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/60 shadow-xl space-y-6">
    <div class="text-center space-y-1.5">
      <h1 class="text-2xl font-bold text-white">Welcome Back</h1>
      <p class="text-sm text-zinc-400">Sign in to your CP Hub account</p>
    </div>

    {#if error}
      <div class="p-3 rounded-xl bg-zinc-900 border border-zinc-700 text-zinc-200 text-sm flex items-center space-x-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-white" />
        <span>{error}</span>
      </div>
    {/if}

    <form on:submit|preventDefault={handleSubmit} class="space-y-4">
      <div>
        <label for="email" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Email or Username</label>
        <input
          id="email"
          type="text"
          bind:value={emailOrUsername}
          required
          placeholder="user@example.com or username"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 placeholder-zinc-600 text-sm transition"
        />
      </div>

      <div>
        <label for="password" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          required
          placeholder="••••••••"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 placeholder-zinc-600 text-sm transition"
        />
      </div>

      <button
        type="submit"
        disabled={loading}
        class="w-full py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black shadow-sm transition flex items-center justify-center space-x-2"
      >
        <LogIn class="w-4 h-4" />
        <span>{loading ? 'Signing in...' : 'Sign In'}</span>
      </button>
    </form>

    <div class="text-center text-xs text-zinc-400 pt-2 border-t border-zinc-800">
      Don't have an account?
      <a href="/register" class="text-white hover:underline font-semibold ml-1">Create one</a>
    </div>
  </div>
</div>
