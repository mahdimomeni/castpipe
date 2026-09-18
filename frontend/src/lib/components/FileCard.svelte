<script lang="ts">
  import type { DropItem } from '../types'
  import { formatBytes, formatRelativeTime } from '../utils'
  import { OpenDownloadsFolder } from '../../../wailsjs/go/main/App.js'
  import { appState } from '../state.svelte'
  import { 
    Archive, 
    FileText, 
    FolderArchive, 
    ExternalLink, 
    Clock, 
    User, 
    CheckCircle2, 
    HardDriveDownload 
  } from '@lucide/svelte'

  interface Props {
    item: DropItem
  }

  const { item }: Props = $props()
  const file = $derived(item.file)
  const fileName = $derived(file?.fileName || 'unnamed-drop')
  const fileSize = $derived(file?.fileSize || 0)
  const isArchive = $derived(file?.isArchive || false)
  const filePath = $derived(file?.filePath || '')

  async function handleOpenInExplorer() {
    try {
      await OpenDownloadsFolder()
    } catch (e: any) {
      appState.addToast('error', 'Open Explorer Failed', e?.toString() || 'Could not open folder')
    }
  }
</script>

<article class="group rounded-lg border border-[#30363d] bg-[#161b22]/70 shadow-lg backdrop-blur-sm overflow-hidden transition hover:border-[#484f58]">
  <!-- Card Header -->
  <div class="flex items-center justify-between border-b border-[#30363d] bg-[#0d1117]/80 px-3.5 py-2">
    <div class="flex items-center space-x-2.5">
      <!-- File type pill -->
      {#if isArchive}
        <span class="flex items-center gap-1 rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-purple-400 bg-purple-950/40 border border-purple-800/40">
          <FolderArchive class="h-3 w-3" />
          ZIP ARCHIVE
        </span>
      {:else}
        <span class="flex items-center gap-1 rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-cyan-400 bg-cyan-950/40 border border-cyan-800/40">
          <FileText class="h-3 w-3" />
          FILE
        </span>
      {/if}

      <!-- Sender Info -->
      <span class="flex items-center gap-1.5 text-xs text-zinc-300 font-semibold truncate max-w-[200px]" title={item.sender}>
        <User class="h-3 w-3 text-zinc-500" />
        {item.sender}
      </span>
      <span class="text-[10px] font-mono text-zinc-500">
        ({item.senderIp})
      </span>
    </div>

    <!-- Timestamp -->
    <div class="flex items-center space-x-3 text-xs">
      <span class="flex items-center gap-1 text-[11px] text-zinc-500">
        <Clock class="h-3 w-3" />
        {formatRelativeTime(item.timestamp)}
      </span>
    </div>
  </div>

  <!-- Body Content -->
  <div class="p-3.5 bg-[#0b0f14]/80 flex items-center justify-between">
    <div class="flex items-center space-x-3 min-w-0">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border {
        isArchive
          ? 'border-purple-500/30 bg-purple-950/20 text-purple-400'
          : 'border-cyan-500/30 bg-cyan-950/20 text-cyan-400'
      }">
        {#if isArchive}
          <Archive class="h-5 w-5" />
        {:else}
          <FileText class="h-5 w-5" />
        {/if}
      </div>

      <div class="min-w-0">
        <h4 class="text-xs font-bold text-white truncate max-w-sm select-text" title={fileName}>
          {fileName}
        </h4>
        <div class="mt-0.5 flex items-center space-x-2 text-[11px] font-mono text-zinc-400">
          <span class="font-semibold text-emerald-400">{formatBytes(fileSize)}</span>
          <span class="text-zinc-600">&bull;</span>
          <span class="text-[10px] text-zinc-500 truncate max-w-xs" title={filePath}>
            Saved to DevDrop
          </span>
        </div>
      </div>
    </div>

    <!-- Quick Link Action -->
    <button
      onclick={handleOpenInExplorer}
      class="flex items-center gap-1.5 rounded-md bg-[#21262d] hover:bg-[#30363d] text-zinc-300 hover:text-white px-3 py-1.5 text-xs font-medium transition border border-[#30363d] hover:border-zinc-500 shrink-0 shadow-sm"
      title="Open destination folder in File Explorer"
    >
      <ExternalLink class="h-3.5 w-3.5 text-emerald-400" />
      <span>Open in Explorer</span>
    </button>
  </div>

  <!-- Footer Path Bar -->
  <div class="flex items-center justify-between border-t border-[#21262d] bg-[#161b22]/40 px-3.5 py-1 text-[10px] font-mono text-zinc-500">
    <span class="flex items-center gap-1.5 truncate max-w-md select-text" title={filePath}>
      <CheckCircle2 class="h-3 w-3 text-emerald-500 shrink-0" />
      <span class="truncate">{filePath}</span>
    </span>
    <span class="text-[9px] text-zinc-600 shrink-0 ml-2">ID: {item.id.substring(0, 8)}</span>
  </div>
</article>
