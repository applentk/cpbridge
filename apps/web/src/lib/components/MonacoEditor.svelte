<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { LanguageId } from '@cpbridge/contracts';
  import type { editor } from 'monaco-editor';

  export let value: string = '';
  export let language: LanguageId = 'cpp23';
  export let readonly: boolean = false;
  export let disabled: boolean = false;

  let editorContainer: HTMLDivElement;
  let editorInstance: editor.IStandaloneCodeEditor | null = null;

  const monacoLanguageMap: Record<LanguageId, string> = {
    cpp23: 'cpp',
    python3: 'python',
    java21: 'java'
  };

  onMount(async () => {
    if (typeof window === 'undefined') return;

    try {
      const monaco = await import('monaco-editor');
      
      const inst = monaco.editor.create(editorContainer, {
        value: value,
        language: monacoLanguageMap[language] || 'cpp',
        theme: 'vs-dark',
        readOnly: readonly || disabled,
        domReadOnly: readonly || disabled,
        tabIndex: disabled ? -1 : 0,
        automaticLayout: true,
        fontSize: 14,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        padding: { top: 12, bottom: 12 },
        fontFamily: "'JetBrains Mono', Menlo, Monaco, 'Courier New', monospace"
      });
      editorInstance = inst;

      inst.onDidChangeModelContent(() => {
        value = inst.getValue();
      });
    } catch (err) {
      console.error('Failed to load Monaco editor:', err);
    }
  });

  $: if (editorInstance && language) {
    const inst = editorInstance;
    import('monaco-editor').then((monaco) => {
      const model = inst.getModel();
      if (model) {
        monaco.editor.setModelLanguage(model, monacoLanguageMap[language] || 'cpp');
      }
    });
  }

  $: if (editorInstance) {
    const isReadOnly = readonly || disabled;
    editorInstance.updateOptions({
      readOnly: isReadOnly,
      domReadOnly: isReadOnly,
      tabIndex: disabled ? -1 : 0
    });

    if (disabled) {
      editorInstance.getContainerDomNode().querySelector('textarea')?.blur();
    }
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

<div
  class="w-full h-full min-h-87.5 rounded-xl overflow-hidden border border-zinc-800 bg-[#1e1e1e] transition {disabled ? 'opacity-50 grayscale pointer-events-none cursor-not-allowed' : ''}"
  aria-disabled={disabled}
  bind:this={editorContainer}
></div>
