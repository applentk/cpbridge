<script lang="ts">
  import { ChevronLeft, ChevronRight } from 'lucide-svelte';
  import { createEventDispatcher } from 'svelte';

  export let currentPage: number = 1;
  export let pageSize: number = 20;
  export let totalItems: number = 0;
  export let pageSizeOptions: number[] = [10, 20, 50, 100];
  export let showPageSizeSelector: boolean = true;
  export let itemName: string = 'items';

  const dispatch = createEventDispatcher<{
    pageChange: number;
    pageSizeChange: number;
  }>();

  $: totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  $: fromItem = totalItems === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  $: toItem = Math.min(totalItems, currentPage * pageSize);

  function goToPage(p: number) {
    if (p < 1 || p > totalPages || p === currentPage) return;
    dispatch('pageChange', p);
  }

  function handlePageSizeChange(e: Event) {
    const target = e.target as HTMLSelectElement;
    const newSize = parseInt(target.value, 10);
    if (!isNaN(newSize) && newSize > 0) {
      dispatch('pageSizeChange', newSize);
    }
  }

  // Calculate visible page range (e.g. 1 ... 4 5 6 ... 10)
  $: pageNumbers = (() => {
    if (totalPages <= 7) {
      return Array.from({ length: totalPages }, (_, i) => i + 1);
    }
    const pages: (number | 'ellipsis')[] = [];
    pages.push(1);

    const start = Math.max(2, currentPage - 1);
    const end = Math.min(totalPages - 1, currentPage + 1);

    if (start > 2) {
      pages.push('ellipsis');
    }

    for (let i = start; i <= end; i++) {
      pages.push(i);
    }

    if (end < totalPages - 1) {
      pages.push('ellipsis');
    }

    pages.push(totalPages);
    return pages;
  })();
</script>

<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 py-3 text-xs text-zinc-400">
  <div class="flex items-center space-x-3">
    <span>
      Showing <strong class="text-zinc-200">{fromItem}</strong> to <strong class="text-zinc-200">{toItem}</strong> of <strong class="text-zinc-200">{totalItems}</strong> {itemName}
    </span>

    {#if showPageSizeSelector && pageSizeOptions.length > 1}
      <div class="flex items-center space-x-1.5 pl-2 border-l border-zinc-800">
        <span>Show</span>
        <select
          value={pageSize}
          on:change={handlePageSizeChange}
          class="bg-zinc-900 border border-zinc-800 rounded-lg px-2 py-1 text-xs text-zinc-200 focus:outline-none focus:border-zinc-700"
        >
          {#each pageSizeOptions as opt}
            <option value={opt}>{opt}</option>
          {/each}
        </select>
        <span>per page</span>
      </div>
    {/if}
  </div>

  {#if totalPages > 1}
    <nav class="flex items-center space-x-1" aria-label="Pagination">
      <button
        type="button"
        on:click={() => goToPage(currentPage - 1)}
        disabled={currentPage <= 1}
        class="p-1.5 rounded-lg border border-zinc-800 bg-zinc-900 text-zinc-300 hover:text-white hover:bg-zinc-800 disabled:opacity-40 disabled:pointer-events-none transition flex items-center justify-center cursor-pointer"
        title="Previous page"
        aria-label="Previous page"
      >
        <ChevronLeft class="w-4 h-4" />
      </button>

      {#each pageNumbers as p, idx (idx + '-' + p)}
        {#if p === 'ellipsis'}
          <span class="px-2 py-1 text-zinc-600 font-mono select-none">…</span>
        {:else}
          <button
            type="button"
            on:click={() => goToPage(p)}
            class="min-w-[2rem] h-8 px-2.5 rounded-lg text-xs font-semibold font-mono transition flex items-center justify-center cursor-pointer {
              p === currentPage
                ? 'bg-white text-black font-bold shadow-sm'
                : 'border border-zinc-800 bg-zinc-900 text-zinc-300 hover:text-white hover:bg-zinc-800'
            }"
            aria-current={p === currentPage ? 'page' : undefined}
          >
            {p}
          </button>
        {/if}
      {/each}

      <button
        type="button"
        on:click={() => goToPage(currentPage + 1)}
        disabled={currentPage >= totalPages}
        class="p-1.5 rounded-lg border border-zinc-800 bg-zinc-900 text-zinc-300 hover:text-white hover:bg-zinc-800 disabled:opacity-40 disabled:pointer-events-none transition flex items-center justify-center cursor-pointer"
        title="Next page"
        aria-label="Next page"
      >
        <ChevronRight class="w-4 h-4" />
      </button>
    </nav>
  {/if}
</div>
