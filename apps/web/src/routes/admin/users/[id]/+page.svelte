<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import type { User, UserRole } from '@cpbridge/contracts';
  import { ArrowLeft, Shield, ShieldCheck, Check, UserCheck, UserX } from 'lucide-svelte';

  let userId = $page.params.id;
  let user: User | null = null;
  let loading = true;
  let error = '';
  let successMsg = '';

  async function loadUser() {
    loading = true;
    error = '';
    try {
      user = await api.get<User>(`/admin/users/${userId}`);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load user';
    } finally {
      loading = false;
    }
  }

  async function setRole(newRole: UserRole) {
    if (!user || user.role === newRole) return;
    try {
      await api.patch(`/admin/users/${userId}/role`, { role: newRole });
      successMsg = `Role changed to ${newRole}!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadUser();
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : '';
      if (errMsg === 'LAST_ADMIN') {
        alert('Action rejected: You cannot demote the last active administrator.');
      } else {
        alert(errMsg || 'Failed to change role');
      }
    }
  }

  async function toggleStatus() {
    if (!user) return;
    const nextStatus = !user.isActive;
    const actionName = nextStatus ? 'enable' : 'disable';
    if (!confirm(`Are you sure you want to ${actionName} account "${user.username}"?`)) return;

    try {
      await api.patch(`/admin/users/${userId}/status`, { isActive: nextStatus });
      successMsg = `Account "${user.username}" is now ${nextStatus ? 'active' : 'disabled'}!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadUser();
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : '';
      if (errMsg === 'LAST_ADMIN') {
        alert('Action rejected: You cannot disable the last active administrator.');
      } else {
        alert(errMsg || 'Failed to change status');
      }
    }
  }

  onMount(() => {
    loadUser();
  });
</script>

{#if loading}
  <div class="h-64 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !user}
  <div class="p-8 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300">
    <h2 class="font-bold text-lg">Error</h2>
    <p class="text-sm">{error || 'User not found'}</p>
  </div>
{:else}
  <div class="max-w-2xl mx-auto space-y-6">
    <div class="flex items-center space-x-3">
      <a
        href="/admin/users"
        class="p-2 rounded-xl text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
      >
        <ArrowLeft class="w-4 h-4" />
      </a>
      <div>
        <h1 class="text-2xl font-bold text-white">User Profile</h1>
        <p class="text-xs text-zinc-400">Manage account permissions and active standing.</p>
      </div>
    </div>

    {#if successMsg}
      <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
        <Check class="w-4 h-4 text-emerald-400" />
        <span>{successMsg}</span>
      </div>
    {/if}

    <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/40 space-y-6 shadow-xl">
      <div class="flex items-center justify-between">
        <div class="flex items-center space-x-4">
          <div class="w-14 h-14 rounded-2xl bg-zinc-800 border border-zinc-700 flex items-center justify-center text-xl font-bold uppercase text-white">
            {user.username.slice(0, 2)}
          </div>
          <div>
            <h2 class="text-xl font-bold text-white">{user.username}</h2>
            <p class="text-xs text-zinc-400 font-mono">{user.email}</p>
          </div>
        </div>

        <span class="text-xs px-3 py-1 rounded-full font-bold {
          user.isActive
            ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
            : 'bg-rose-500/15 text-rose-300 border border-rose-500/30'
        }">
          {user.isActive ? 'Active' : 'Disabled'}
        </span>
      </div>

      <div class="grid grid-cols-2 gap-4 pt-4 border-t border-zinc-800/80 text-xs">
        <div class="space-y-1">
          <span class="text-zinc-500 uppercase font-semibold">User ID</span>
          <p class="font-mono text-zinc-300">{user.id}</p>
        </div>
        <div class="space-y-1">
          <span class="text-zinc-500 uppercase font-semibold">Current Role</span>
          <p class="font-bold {user.role === 'ADMIN' ? 'text-amber-400' : 'text-zinc-300'}">{user.role}</p>
        </div>
        <div class="space-y-1">
          <span class="text-zinc-500 uppercase font-semibold">Registered At</span>
          <p class="text-zinc-300">{new Date(user.createdAt).toLocaleString()}</p>
        </div>
        <div class="space-y-1">
          <span class="text-zinc-500 uppercase font-semibold">Last Updated</span>
          <p class="text-zinc-300">{new Date(user.updatedAt).toLocaleString()}</p>
        </div>
      </div>

      <!-- Actions -->
      <div class="pt-6 border-t border-zinc-800 space-y-4">
        <h3 class="text-xs font-semibold uppercase text-zinc-400">Account Management</h3>

        <div class="flex flex-wrap gap-3">
          {#if user.role === 'USER'}
            <button
              on:click={() => setRole('ADMIN')}
              class="px-4 py-2 rounded-xl text-xs font-bold bg-amber-500/20 text-amber-300 border border-amber-500/30 hover:bg-amber-500/30 transition flex items-center space-x-1.5"
            >
              <ShieldCheck class="w-4 h-4" />
              <span>Promote to ADMIN</span>
            </button>
          {:else}
            <button
              on:click={() => setRole('USER')}
              class="px-4 py-2 rounded-xl text-xs font-bold bg-zinc-800 hover:bg-zinc-700 text-zinc-300 transition flex items-center space-x-1.5"
            >
              <Shield class="w-4 h-4" />
              <span>Demote to USER</span>
            </button>
          {/if}

          <button
            on:click={toggleStatus}
            class="px-4 py-2 rounded-xl text-xs font-bold transition flex items-center space-x-1.5 {
              user.isActive
                ? 'bg-rose-500/10 text-rose-300 border border-rose-500/30 hover:bg-rose-500/20'
                : 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/30 hover:bg-emerald-500/20'
            }"
          >
            {#if user.isActive}
              <UserX class="w-4 h-4" />
              <span>Disable Account</span>
            {:else}
              <UserCheck class="w-4 h-4" />
              <span>Enable Account</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
