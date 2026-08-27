<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { AuthResponse } from '@cpbridge/contracts';
  import { UserPlus, AlertCircle } from 'lucide-svelte';

  let email = $state('');
  let username = $state('');
  let password = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(event?: SubmitEvent) {
    if (event) event.preventDefault();
    error = '';
    loading = true;
    try {
      const res = await api.post<AuthResponse>('/auth/register', {
        email,
        username,
        password
      });
      auth.setAuth(res.user, res.accessToken);
      await goto(res.user.role === 'ADMIN' ? '/admin' : '/contests');
    } catch (err) {
      error = err instanceof Error ? err.message : 'Registration failed';
    } finally {
      loading = false;
    }
  }
</script>

<div class="max-w-md mx-auto py-12">
  <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/60 shadow-xl space-y-6">
    <div class="text-center space-y-1.5">
      <h1 class="text-2xl font-bold text-white">Create Account</h1>
      <p class="text-sm text-zinc-400">Join cpbridge with a single unified account</p>
    </div>

    {#if error}
      <div class="p-3 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm flex items-center space-x-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-red-400" />
        <span>{error}</span>
      </div>
    {/if}

    <form onsubmit={handleSubmit} class="space-y-4">
      <div>
        <label for="reg-email" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Email Address</label>
        <input
          id="reg-email"
          type="email"
          bind:value={email}
          required
          placeholder="alex@example.com"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 placeholder-zinc-600 text-sm transition"
        />
      </div>

      <div>
        <label for="reg-username" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Username</label>
        <input
          id="reg-username"
          type="text"
          bind:value={username}
          required
          placeholder="alex_coder"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 placeholder-zinc-600 text-sm transition"
        />
      </div>

      <div>
        <label for="reg-password" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Password</label>
        <input
          id="reg-password"
          type="password"
          bind:value={password}
          required
          placeholder="At least 6 characters"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 placeholder-zinc-600 text-sm transition"
        />
      </div>

      <button
        type="submit"
        disabled={loading}
        class="w-full py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black shadow-sm transition flex items-center justify-center space-x-2"
      >
        <UserPlus class="w-4 h-4" />
        <span>{loading ? 'Creating account...' : 'Create Account'}</span>
      </button>
    </form>

    <div class="text-center text-xs text-zinc-400 pt-2 border-t border-zinc-800">
      Already have an account?
      <a href="/login" class="text-white hover:underline font-semibold ml-1">Sign in</a>
    </div>
  </div>
</div>
