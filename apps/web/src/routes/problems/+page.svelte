<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { Problem, PlatformType, SampleCase } from '@cp-hub/contracts';
  import ProblemCard from '$lib/components/ProblemCard.svelte';
  import { Search, Plus, ExternalLink, Filter, AlertCircle, CheckCircle2, Wand2, FileText, Globe, Trash2 } from 'lucide-svelte';

  let problems: Problem[] = [];
  let total = 0;
  let query = '';
  let selectedPlatform: string = '';
  let loading = true;

  let showModal = false;
  let modalTab: 'url' | 'paste' = 'url';

  // Import by URL
  let importUrl = '';
  let importError = '';
  let importLoading = false;
  let importSuccess = '';

  // Custom Problem / Paste Statement
  let customPlatform: PlatformType = 'CODEFORCES';
  let customExternalId = '';
  let customTitle = '';
  let customUrl = '';
  let customDifficulty: number | null = 800;
  let customTagsStr = 'implementation, math';
  let customStatement = '';
  let customTimeLimit = '1.0 sec';
  let customMemoryLimit = '256 MB';
  let customSampleCases: SampleCase[] = [
    { input: '', output: '', explanation: '' }
  ];

  // Auto-extraction tool
  let rawCopiedText = '';
  let extracting = false;
  let extractNotice = '';

  async function loadProblems() {
    loading = true;
    try {
      let path = `/problems?query=${encodeURIComponent(query)}`;
      if (selectedPlatform) path += `&platform=${selectedPlatform}`;
      const res = await api.get<{ problems: Problem[]; total: number }>(path);
      problems = res.problems || [];
      total = res.total || 0;
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  }

  async function handleImportUrl() {
    importError = '';
    importSuccess = '';
    importLoading = true;
    try {
      const p = await api.post<Problem>('/problems/import', { url: importUrl });
      importSuccess = `Successfully imported "${p.title}"!`;
      importUrl = '';
      await loadProblems();
      setTimeout(() => {
        showModal = false;
        importSuccess = '';
      }, 1200);
    } catch (err: any) {
      importError = err.message || 'Import failed';
    } finally {
      importLoading = false;
    }
  }

  async function handleAutoExtract() {
    if (!rawCopiedText.trim()) return;
    extracting = true;
    extractNotice = '';
    try {
      const res = await api.post<{
        title?: string;
        statement: string;
        timeLimit?: string;
        memoryLimit?: string;
        sampleCases: SampleCase[];
      }>('/problems/extract-text', { rawContent: rawCopiedText });

      if (res.title) customTitle = res.title;
      if (res.statement) customStatement = res.statement;
      if (res.timeLimit) customTimeLimit = res.timeLimit;
      if (res.memoryLimit) customMemoryLimit = res.memoryLimit;
      if (res.sampleCases && res.sampleCases.length > 0) {
        customSampleCases = res.sampleCases;
      }
      extractNotice = `Extracted statement & ${res.sampleCases?.length || 0} sample cases!`;
    } catch (err: any) {
      extractNotice = 'Extraction notice: Could not auto-parse all fields.';
    } finally {
      extracting = false;
    }
  }

  async function handleCreateCustom() {
    if (!customTitle.trim()) {
      importError = 'Problem title is required';
      return;
    }

    importLoading = true;
    importError = '';
    importSuccess = '';

    try {
      const tags = customTagsStr
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean);

      const p = await api.post<Problem>('/problems', {
        platform: customPlatform,
        externalId: customExternalId || `custom_${Date.now()}`,
        title: customTitle,
        url: customUrl || `https://cphub.dev/problems/${customExternalId}`,
        difficulty: customDifficulty ? Number(customDifficulty) : null,
        tags,
        statement: customStatement,
        timeLimit: customTimeLimit,
        memoryLimit: customMemoryLimit,
        sampleCases: customSampleCases.filter((c) => c.input.trim() || c.output.trim())
      });

      importSuccess = `Successfully created problem "${p.title}"!`;
      await loadProblems();
      setTimeout(() => {
        showModal = false;
        importSuccess = '';
      }, 1200);
    } catch (err: any) {
      importError = err.message || 'Creation failed';
    } finally {
      importLoading = false;
    }
  }

  function addSampleCase() {
    customSampleCases = [...customSampleCases, { input: '', output: '', explanation: '' }];
  }

  function removeSampleCase(idx: number) {
    customSampleCases = customSampleCases.filter((_, i) => i !== idx);
  }

  onMount(() => {
    loadProblems();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white">Problem Library</h1>
      <p class="text-sm text-zinc-400">Explore, import, or create problems with full statements from Codeforces and AtCoder.</p>
    </div>

    <button
      on:click={() => {
        showModal = true;
        importError = '';
        importSuccess = '';
      }}
      class="px-4 py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-2 shrink-0 self-start sm:self-auto"
    >
      <Plus class="w-4 h-4" />
      <span>Add / Import Problem</span>
    </button>
  </div>

  <!-- Search and Filters -->
  <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
    <div class="sm:col-span-2 relative">
      <Search class="w-4 h-4 text-zinc-500 absolute left-3.5 top-1/2 -translate-y-1/2" />
      <input
        type="text"
        bind:value={query}
        on:input={loadProblems}
        placeholder="Search problems by title, ID, or tag..."
        class="w-full pl-10 pr-4 py-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 placeholder-zinc-500 text-sm transition"
      />
    </div>

    <div>
      <select
        bind:value={selectedPlatform}
        on:change={loadProblems}
        class="w-full px-4 py-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
      >
        <option value="">All Platforms</option>
        <option value="CODEFORCES">Codeforces</option>
        <option value="ATCODER">AtCoder</option>
        <option value="LEETCODE">LeetCode</option>
      </select>
    </div>
  </div>

  <!-- Problems List -->
  {#if loading}
    <div class="space-y-3">
      {#each Array(5) as _}
        <div class="h-20 rounded-xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if problems.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-4">
      <p class="text-zinc-400 text-base">No problems found matching your criteria.</p>
      <button
        on:click={() => (showModal = true)}
        class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition inline-flex items-center space-x-1.5"
      >
        <Plus class="w-4 h-4" />
        <span>Add a Problem</span>
      </button>
    </div>
  {:else}
    <div class="space-y-3">
      {#each problems as prob}
        <ProblemCard problem={prob} />
      {/each}
    </div>
  {/if}

  <!-- Add / Import Problem Modal -->
  {#if showModal}
    <div class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4">
      <div class="max-w-2xl w-full rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl space-y-5 max-h-[90vh] overflow-y-auto">
        <div class="space-y-1">
          <h3 class="text-xl font-bold text-white">Add Problem to Library</h3>
          <p class="text-xs text-zinc-400">Import automatically by URL or paste problem statement text/HTML directly.</p>
        </div>

        <!-- Mode Selector Tabs -->
        <div class="flex items-center space-x-2 border-b border-zinc-800 pb-3">
          <button
            on:click={() => (modalTab = 'url')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-1.5 {
              modalTab === 'url' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <Globe class="w-3.5 h-3.5" />
            <span>Import by URL</span>
          </button>
          <button
            on:click={() => (modalTab = 'paste')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-1.5 {
              modalTab === 'paste' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <FileText class="w-3.5 h-3.5" />
            <span>Paste Statement / Create Custom</span>
          </button>
        </div>

        {#if importError}
          <div class="p-3 rounded-xl bg-zinc-900 border border-zinc-700 text-zinc-200 text-sm flex items-center space-x-2">
            <AlertCircle class="w-4 h-4 shrink-0 text-white" />
            <span>{importError}</span>
          </div>
        {/if}

        {#if importSuccess}
          <div class="p-3 rounded-xl bg-zinc-100 border border-white text-black text-sm font-semibold flex items-center space-x-2">
            <CheckCircle2 class="w-4 h-4 shrink-0 text-black" />
            <span>{importSuccess}</span>
          </div>
        {/if}

        {#if modalTab === 'url'}
          <!-- 1. URL Mode -->
          <div class="space-y-4">
            <div>
              <label for="import-url" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Official Problem URL</label>
              <input
                id="import-url"
                type="url"
                bind:value={importUrl}
                placeholder="https://codeforces.com/problemset/problem/1900/A"
                class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600 transition"
              />
            </div>
            <div class="text-[11px] text-zinc-500 space-y-0.5">
              <div>Examples:</div>
              <div>• https://codeforces.com/problemset/problem/1900/A</div>
              <div>• https://atcoder.jp/contests/abc350/tasks/abc350_f</div>
              <div>• https://leetcode.com/problems/two-sum/</div>
            </div>
            <div class="flex items-center justify-end space-x-3 pt-2">
              <button
                on:click={() => (showModal = false)}
                class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
              >
                Cancel
              </button>
              <button
                on:click={handleImportUrl}
                disabled={importLoading || !importUrl}
                class="px-5 py-2.5 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black transition"
              >
                {importLoading ? 'Importing...' : 'Import'}
              </button>
            </div>
          </div>

        {:else}
          <!-- 2. Statement Paste / Custom Mode -->
          <div class="space-y-5">
            <!-- Extraction Tool Box -->
            <div class="p-4 rounded-xl border border-zinc-700 bg-zinc-950 space-y-3">
              <div class="flex items-center justify-between">
                <div class="flex items-center space-x-1.5 text-xs font-bold text-white">
                  <Wand2 class="w-4 h-4 text-white" />
                  <span>Auto-Extract from Copied Webpage Content</span>
                </div>
                {#if extractNotice}
                  <span class="text-[11px] text-zinc-300 font-medium">{extractNotice}</span>
                {/if}
              </div>

              <textarea
                bind:value={rawCopiedText}
                rows="3"
                placeholder="Paste raw text or HTML from Codeforces, AtCoder, or LeetCode here to auto-fill title, limits, and sample testcases..."
                class="w-full px-3 py-2 rounded-lg bg-zinc-900 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-200 text-xs font-mono placeholder-zinc-600 transition"
              ></textarea>

              <div class="flex justify-end">
                <button
                  type="button"
                  on:click={handleAutoExtract}
                  disabled={extracting || !rawCopiedText.trim()}
                  class="px-3.5 py-1.5 rounded-lg text-xs font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black transition flex items-center space-x-1.5"
                >
                  <Wand2 class="w-3.5 h-3.5" />
                  <span>{extracting ? 'Extracting...' : 'Auto-Extract Fields'}</span>
                </button>
              </div>
            </div>

            <!-- Form Fields -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label for="c-title" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Problem Title</label>
                <input
                  id="c-title"
                  type="text"
                  bind:value={customTitle}
                  placeholder="e.g. A. Cover in Water"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>

              <div>
                <label for="c-plat" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Platform</label>
                <select
                  id="c-plat"
                  bind:value={customPlatform}
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                >
                  <option value="CODEFORCES">Codeforces</option>
                  <option value="ATCODER">AtCoder</option>
                  <option value="LEETCODE">LeetCode</option>
                </select>
              </div>

              <div>
                <label for="c-extid" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">External ID / Code</label>
                <input
                  id="c-extid"
                  type="text"
                  bind:value={customExternalId}
                  placeholder="e.g. 1900/A or abc350_a"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>

              <div>
                <label for="c-url" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Official Source URL</label>
                <input
                  id="c-url"
                  type="url"
                  bind:value={customUrl}
                  placeholder="https://codeforces.com/problemset/problem/1900/A"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>

              <div>
                <label for="c-diff" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Difficulty Rating</label>
                <input
                  id="c-diff"
                  type="number"
                  bind:value={customDifficulty}
                  placeholder="e.g. 800"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>

              <div>
                <label for="c-tags" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Tags (comma separated)</label>
                <input
                  id="c-tags"
                  type="text"
                  bind:value={customTagsStr}
                  placeholder="dp, math, greedy"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>

              <div>
                <label for="c-tl" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Time Limit</label>
                <input
                  id="c-tl"
                  type="text"
                  bind:value={customTimeLimit}
                  placeholder="1.0 sec"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>

              <div>
                <label for="c-ml" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Memory Limit</label>
                <input
                  id="c-ml"
                  type="text"
                  bind:value={customMemoryLimit}
                  placeholder="256 MB"
                  class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs transition"
                />
              </div>
            </div>

            <!-- Problem Statement Textarea -->
            <div>
              <label for="c-stmt" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Problem Statement (Text or HTML)</label>
              <textarea
                id="c-stmt"
                bind:value={customStatement}
                rows="6"
                placeholder="Paste the problem statement description, input and output format, and constraints here..."
                class="w-full px-3 py-2 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-xs font-mono transition"
              ></textarea>
            </div>

            <!-- Sample Test Cases Builder -->
            <div class="space-y-3 border-t border-zinc-800 pt-4">
              <div class="flex items-center justify-between">
                <span class="text-xs font-semibold uppercase text-zinc-400">Sample Test Cases ({customSampleCases.length})</span>
                <button
                  type="button"
                  on:click={addSampleCase}
                  class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 border border-zinc-700 transition flex items-center space-x-1"
                >
                  <Plus class="w-3 h-3" />
                  <span>Add Sample Case</span>
                </button>
              </div>

              {#each customSampleCases as sc, idx}
                <div class="p-3 rounded-xl border border-zinc-800 bg-zinc-950/80 space-y-2.5 relative">
                  <div class="flex items-center justify-between text-xs text-zinc-400 font-bold">
                    <span>Example #{idx + 1}</span>
                    {#if customSampleCases.length > 1}
                      <button
                        type="button"
                        on:click={() => removeSampleCase(idx)}
                        class="text-zinc-500 hover:text-white p-1"
                        title="Remove"
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    {/if}
                  </div>

                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div>
                      <label for={`sample-in-${idx}`} class="block text-[10px] text-zinc-500 uppercase font-semibold mb-1">Sample Input</label>
                      <textarea
                        id={`sample-in-${idx}`}
                        bind:value={sc.input}
                        rows="3"
                        placeholder="e.g. 5\n1 2 3 4 5"
                        class="w-full px-2.5 py-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-xs font-mono text-zinc-200 focus:outline-none"
                      ></textarea>
                    </div>

                    <div>
                      <label for={`sample-out-${idx}`} class="block text-[10px] text-zinc-500 uppercase font-semibold mb-1">Sample Output</label>
                      <textarea
                        id={`sample-out-${idx}`}
                        bind:value={sc.output}
                        rows="3"
                        placeholder="e.g. 15"
                        class="w-full px-2.5 py-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-xs font-mono text-zinc-200 focus:outline-none"
                      ></textarea>
                    </div>
                  </div>
                </div>
              {/each}
            </div>

            <!-- Submit Button -->
            <div class="flex items-center justify-end space-x-3 pt-3 border-t border-zinc-800">
              <button
                type="button"
                on:click={() => (showModal = false)}
                class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
              >
                Cancel
              </button>
              <button
                type="button"
                on:click={handleCreateCustom}
                disabled={importLoading || !customTitle.trim()}
                class="px-5 py-2.5 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black transition"
              >
                {importLoading ? 'Creating...' : 'Save Problem & Statement'}
              </button>
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
