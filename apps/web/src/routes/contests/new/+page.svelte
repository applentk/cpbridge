<script lang="ts">
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';

  $: {
    if (browser && !$auth.loading) {
      if ($auth.user?.role === 'ADMIN') {
        goto('/admin/contests/new');
      } else {
        goto('/contests');
      }
    }
  }
</script>

<div class="p-12 text-center text-zinc-500">Redirecting...</div>

