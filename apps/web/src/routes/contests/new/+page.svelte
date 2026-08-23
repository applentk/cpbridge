<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';

  onMount(() => {
    if ($auth.user?.role === 'ADMIN') {
      goto('/admin/contests/new');
    } else {
      goto('/contests');
    }
  });
</script>

<div class="p-12 text-center text-zinc-500">Redirecting...</div>
