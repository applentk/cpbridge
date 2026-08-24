<script lang="ts">
  import '../app.css';
  import 'katex/dist/katex.min.css';
  import { onMount } from 'svelte';
  import { auth } from '$lib/stores/auth';
  import { reconcileExtensionSubmissions } from '$lib/extension/reconcile';
  import Navbar from '$lib/components/Navbar.svelte';

  onMount(async () => {
    await auth.init();
    await reconcileExtensionSubmissions();
  });
</script>

<div class="min-h-screen flex flex-col bg-zinc-950 text-zinc-100">
  <Navbar />
  <main class="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <slot />
  </main>
  <footer class="border-t border-zinc-900 bg-zinc-950/80 py-6 text-center text-xs text-zinc-500">
    <p>CP Hub &copy; 2026 — Unified Competitive Programming Interface</p>
  </footer>
</div>
