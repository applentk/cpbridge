<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';

  let setId = $page.params.id;

  onMount(() => {
    if ($auth.user?.role === 'ADMIN') {
      goto(`/admin/problem-sets/${setId}`);
    } else {
      goto('/contests');
    }
  });
</script>

<div class="p-12 text-center text-zinc-500">Redirecting...</div>
