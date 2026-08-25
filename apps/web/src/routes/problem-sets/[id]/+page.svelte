<script lang="ts">
  import { browser } from '$app/environment';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';

  let setId = $page.params.id;

  $: {
    if (browser && !$auth.loading) {
      if ($auth.user?.role === 'ADMIN') {
        goto(`/admin/problem-sets/${setId}`);
      } else {
        goto('/contests');
      }
    }
  }
</script>

<div class="p-12 text-center text-zinc-500">Redirecting...</div>

