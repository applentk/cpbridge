<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { LanguageId } from '@cp-hub/contracts';

  export let value: string = '';
  export let language: LanguageId = 'cpp23';
  export let readonly: boolean = false;

  let editorContainer: HTMLDivElement;
  let editorInstance: any = null;

  const monacoLanguageMap: Record<LanguageId, string> = {
    cpp23: 'cpp',
    python3: 'python',
    java21: 'java',
    go: 'go',
    rust: 'rust'
  };

  onMount(async () => {
    if (typeof window === 'undefined') return;

    try {
      const monaco = await import('monaco-editor');
      
      editorInstance = monaco.editor.create(editorContainer, {
        value: value,
        language: monacoLanguageMap[language] || 'cpp',
        theme: 'vs-dark',
        readOnly: readonly,
        automaticLayout: true,
        fontSize: 14,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        padding: { top: 12, bottom: 12 },
        fontFamily: "'Fira Code', 'JetBrains Mono', Menlo, Monaco, 'Courier New', monospace"
      });

      editorInstance.onDidChangeModelContent(() => {
        value = editorInstance.getValue();
      });
    } catch (err) {
      console.error('Failed to load Monaco editor:', err);
    }
  });

  $: if (editorInstance && language) {
    import('monaco-editor').then((monaco) => {
      const model = editorInstance.getModel();
      if (model) {
        monaco.editor.setModelLanguage(model, monacoLanguageMap[language] || 'cpp');
      }
    });
  }

  $: if (editorInstance && value !== editorInstance.getValue()) {
    editorInstance.setValue(value);
  }

  onDestroy(() => {
    if (editorInstance) {
      editorInstance.dispose();
    }
  });
</script>

<div class="w-full h-full min-h-[350px] rounded-xl overflow-hidden border border-zinc-800 bg-[#1e1e1e]" bind:this={editorContainer}></div>
