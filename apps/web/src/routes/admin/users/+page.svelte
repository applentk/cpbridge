<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { User, UserRole } from '@cpbridge/contracts';
  import { Search, Check, ArrowRight } from 'lucide-svelte';

  let users: User[] = [];
  let loading = true;
  let error = '';
  let successMsg = '';
  let searchQuery = '';

  async function loadUsers() {
    loading = true;
    error = '';
    try {
      let url = '/admin/users';
      if (searchQuery.trim()) url += `?search=${encodeURIComponent(searchQuery.trim())}`;
      users = await api.get<User[]>(url);
    } catch (err: any) {
      error = err.message || 'Failed to load users';
    } finally {
      loading = false;
    }
  }

  async function handleRoleChange(user: User, newRole: UserRole) {
    if (user.role === newRole) return;
    try {
      await api.patch(`/admin/users/${user.id}/role`, { role: newRole });
      successMsg = `Updated ${user.username}'s role to ${newRole}!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadUsers();
    } catch (err: any) {
      if (err.message === 'LAST_ADMIN') {
        alert('Action rejected: You cannot demote the last active administrator.');
      } else {
        alert(err.message || 'Failed to update user role');
      }
    }
  }

  async function handleToggleStatus(user: User) {
    const nextStatus = !user.isActive;
    const actionName = nextStatus ? 'enable' : 'disable';
    if (!confirm(`Are you sure you want to ${actionName} account "${user.username}"?`)) return;

    try {
      await api.patch(`/admin/users/${user.id}/status`, { isActive: nextStatus });
      successMsg = `Account "${user.username}" is now ${nextStatus ? 'active' : 'disabled'}!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadUsers();
    } catch (err: any) {
      if (err.message === 'LAST_ADMIN') {
        alert('Action rejected: You cannot disable the last active administrator.');
      } else {
        alert(err.message || 'Failed to update user status');
      }
    }
  }

  onMount(() => {
    loadUsers();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold text-white">User Management</h1>
      <p class="text-sm text-zinc-400">View registered accounts, manage administrator roles, and enable/disable access.</p>
    </div>
  </div>

  {#if successMsg}
    <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
      <Check class="w-4 h-4 text-emerald-400" />
      <span>{successMsg}</span>
    </div>
  {/if}

  <!-- Search Filter -->
  <div class="relative">
    <Search class="w-4 h-4 absolute left-3.5 top-3 text-zinc-500" />
    <input
      type="text"
      bind:value={searchQuery}
      on:input={loadUsers}
      placeholder="Search users by username or email..."
      class="w-full pl-10 pr-4 py-2 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-500 focus:outline-none text-zinc-100 text-sm placeholder-zinc-500"
    />
  </div>

  <!-- Users Table -->
  {#if loading}
    <div class="h-64 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
  {:else if error}
    <div class="p-6 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
      {error}
    </div>
  {:else if users.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center text-sm text-zinc-500">
      No users found.
    </div>
  {:else}
    <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 overflow-hidden">
      <table class="w-full text-left text-sm text-zinc-300">
        <thead class="bg-zinc-900/80 border-b border-zinc-800 text-xs text-zinc-400 uppercase font-semibold">
          <tr>
            <th class="px-5 py-3.5">User</th>
            <th class="px-5 py-3.5">Email</th>
            <th class="px-5 py-3.5">Role</th>
            <th class="px-5 py-3.5">Status</th>
            <th class="px-5 py-3.5">Registered</th>
            <th class="px-5 py-3.5 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-zinc-800/60 font-medium">
          {#each users as u}
            <tr class="hover:bg-zinc-800/30 transition {u.isActive ? '' : 'opacity-60 bg-zinc-950/40'}">
              <!-- Username -->
              <td class="px-5 py-3.5">
                <div class="flex items-center space-x-2.5">
                  <div class="w-7 h-7 rounded-full bg-zinc-800 border border-zinc-700 flex items-center justify-center text-xs text-white font-semibold uppercase shrink-0">
                    {u.username.slice(0, 2)}
                  </div>
                  <span class="text-white font-semibold">{u.username}</span>
                </div>
              </td>

              <!-- Email -->
              <td class="px-5 py-3.5 text-zinc-400 text-xs font-mono">
                {u.email}
              </td>

              <!-- Role Selector -->
              <td class="px-5 py-3.5 whitespace-nowrap">
                <select
                  value={u.role}
                  on:change={(e) => handleRoleChange(u, e.currentTarget.value as UserRole)}
                  class="px-2.5 py-1 rounded-lg text-xs font-bold border transition {
                    u.role === 'ADMIN'
                      ? 'bg-amber-500/15 text-amber-300 border-amber-500/30'
                      : 'bg-zinc-800 text-zinc-300 border-zinc-700'
                  }"
                >
                  <option value="USER">USER</option>
                  <option value="ADMIN">ADMIN</option>
                </select>
              </td>

              <!-- Status Badge -->
              <td class="px-5 py-3.5 whitespace-nowrap">
                <span class="text-xs px-2.5 py-0.5 rounded-full font-bold {
                  u.isActive
                    ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                    : 'bg-rose-500/15 text-rose-300 border border-rose-500/30'
                }">
                  {u.isActive ? 'Active' : 'Disabled'}
                </span>
              </td>

              <!-- Registered Date -->
              <td class="px-5 py-3.5 whitespace-nowrap text-xs text-zinc-500">
                {new Date(u.createdAt).toLocaleDateString()}
              </td>

              <!-- Actions -->
              <td class="px-5 py-3.5 text-right whitespace-nowrap">
                <div class="flex items-center justify-end space-x-2">
                  <button
                    on:click={() => handleToggleStatus(u)}
                    class="px-2.5 py-1 rounded-lg text-xs font-semibold transition {
                      u.isActive
                        ? 'text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10'
                        : 'text-emerald-400 hover:bg-emerald-500/10'
                    }"
                    title={u.isActive ? 'Disable account' : 'Enable account'}
                  >
                    {u.isActive ? 'Disable' : 'Enable'}
                  </button>

                  <a
                    href={`/admin/users/${u.id}`}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
                    title="View details"
                  >
                    <ArrowRight class="w-4 h-4" />
                  </a>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
