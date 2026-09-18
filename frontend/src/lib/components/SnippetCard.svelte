<script lang="ts">
  import type { DropItem } from '../types'
  import { formatRelativeTime, highlightCode } from '../utils'
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime.js'
  import { appState } from '../state.svelte'
  import { Copy, Check, Code2, Terminal, Clock, User } from '@lucide/svelte'

  interface Props {
    item: DropItem
  }

  const { item }: Props = $props()
  const snippet = $derived(item.snippet)
  const codeContent = $derived(snippet?.content || '')
  const syntax = $derived(snippet?.syntax || 'plain')

  let copied = $state(false)
  let copyTimeout: any = null

  const highlightedHtml = $derived(highlightCode(codeContent, syntax))
  const lineCount = $derived(codeContent.split('\n').length)

  async function handleCopy() {
    try {
      await ClipboardSetText(codeContent)
      copied = true
      appState.addToast('success', 'Copied to Clipboard', `Copied ${syntax.toUpperCase()} snippet (${lineCount} lines)`)
      if (copyTimeout) clearTimeout(copyTimeout)
      copyTimeout = setTimeout(() => {
        copied = false
      }, 2000)
    } catch (e: any) {
      // Fallback to navigator.clipboard if runtime not in native window
      try {
        await navigator.clipboard.writeText(codeContent)
        copied = true
        setTimeout(() => { copied = false }, 2000)
      } catch (_) {
        appState.addToast('error', 'Copy Failed', 'Could not copy to clipboard')
      }
    }
  }

  // Syntax badge color styles
  const syntaxBadgeClass = $derived.by(() => {
    switch (syntax.toLowerCase()) {
      case 'json':
        return 'text-amber-400 bg-amber-950/40 border-amber-800/40'
      case 'yaml':
      case 'yml':
        return 'text-sky-400 bg-sky-950/40 border-sky-800/40'
      case 'bash':
      case 'sh':
      case 'zsh':
        return 'text-emerald-400 bg-emerald-950/40 border-emerald-800/40'
      case 'sql':
        return 'text-indigo-400 bg-indigo-950/40 border-indigo-800/40'
      case 'js':
      case 'ts':
      case 'javascript':
      case 'typescript':
        return 'text-yellow-400 bg-yellow-950/40 border-yellow-800/40'
      default:
        return 'text-zinc-400 bg-zinc-800/40 border-zinc-700/40'
    }
  })
</script>

<article class="group rounded-lg border border-[#30363d] bg-[#161b22]/70 shadow-lg backdrop-blur-sm overflow-hidden transition hover:border-[#484f58]">
  <!-- Card Header / Terminal Window Titlebar -->
  <div class="flex items-center justify-between border-b border-[#30363d] bg-[#0d1117]/80 px-3.5 py-2">
    <div class="flex items-center space-x-3">
      <!-- Window terminal dots -->
      <div class="flex items-center space-x-1.5">
        <span class="h-2.5 w-2.5 rounded-full bg-rose-500/80"></span>
        <span class="h-2.5 w-2.5 rounded-full bg-amber-500/80"></span>
        <span class="h-2.5 w-2.5 rounded-full bg-emerald-500/80"></span>
      </div>

      <!-- Syntax Pill -->
      <span class="rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider border {syntaxBadgeClass}">
        {syntax}
      </span>

      <!-- Sender Label -->
      <span class="flex items-center gap-1.5 text-xs text-zinc-300 font-semibold truncate max-w-[200px]" title={item.sender}>
        <User class="h-3 w-3 text-zinc-500" />
        {item.sender}
      </span>
      <span class="text-[10px] font-mono text-zinc-500">
        ({item.senderIp})
      </span>
    </div>

    <!-- Actions & Timestamp -->
    <div class="flex items-center space-x-3 text-xs">
      <span class="flex items-center gap-1 text-[11px] text-zinc-500">
        <Clock class="h-3 w-3" />
        {formatRelativeTime(item.timestamp)}
      </span>

      <!-- 1-Click Copy Raw Button -->
      <button
        onclick={handleCopy}
        class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition {
          copied
            ? 'bg-emerald-600/30 text-emerald-400 border border-emerald-500/40'
            : 'bg-[#21262d] text-zinc-300 hover:bg-[#30363d] hover:text-white border border-[#30363d]'
        }"
        title="Copy raw code to clipboard"
      >
        {#if copied}
          <Check class="h-3.5 w-3.5 text-emerald-400" />
          <span class="text-[11px] font-bold">COPIED</span>
        {:else}
          <Copy class="h-3.5 w-3.5" />
          <span class="text-[11px]">Copy Raw</span>
        {/if}
      </button>
    </div>
  </div>

  <!-- Code Block Viewport with Line Numbers -->
  <div class="relative overflow-x-auto bg-[#0b0f14] p-3 text-xs font-mono">
    <div class="flex">
      <!-- Line Number Gutter -->
      <div class="select-none pr-3 text-right text-zinc-600 border-r border-[#21262d] space-y-0 text-[11px] font-mono leading-relaxed">
        {#each Array(lineCount) as _, idx}
          <div>{idx + 1}</div>
        {/each}
      </div>

      <!-- Syntax-Highlighted Code -->
      <div class="flex-1 pl-3.5 overflow-x-auto select-text text-[11px] leading-relaxed">
        <pre class="m-0 font-mono whitespace-pre"><code class="hljs">{@html highlightedHtml}</code></pre>
      </div>
    </div>
  </div>

  <!-- Bottom Metadata Footer -->
  <div class="flex items-center justify-between border-t border-[#21262d] bg-[#161b22]/40 px-3.5 py-1 text-[10px] text-zinc-500">
    <span class="flex items-center gap-1">
      <Code2 class="h-3 w-3 text-zinc-600" />
      {lineCount} lines &bull; {codeContent.length} chars
    </span>
    <span class="font-mono text-[9px] text-zinc-600">ID: {item.id.substring(0, 8)}</span>
  </div>
</article>
