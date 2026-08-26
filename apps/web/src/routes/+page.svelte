<script lang="ts">
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';

  let redirected = false;

  $: if (browser && !$auth.loading && !redirected) {
    redirected = true;
    void goto($auth.user?.role === 'ADMIN' ? '/admin' : '/contests', {
      replaceState: true
    });
  }
</script>

<div class="p-12 text-center text-zinc-500">Redirecting...</div>
