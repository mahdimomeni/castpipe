<script lang="ts">
  import { appState } from '../state.svelte'
  import SnippetCard from './SnippetCard.svelte'
  import FileCard from './FileCard.svelte'
  import { Activity, Inbox, Terminal, Radio } from '@lucide/svelte'
</script>

<div class="flex flex-1 flex-col overflow-y-auto p-4 space-y-3">
  {#if appState.drops.length === 0}
    <!-- Empty Inbound Feed State -->
    <div class="my-auto flex flex-col items-center justify-center rounded-xl border border-dashed border-[#30363d] bg-[#161b22]/20 p-12 text-center">
      <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-emerald-950/30 border border-emerald-500/20 text-emerald-400">
        <Inbox class="h-7 w-7 text-emerald-400/80 animate-pulse" />
      </div>
      <h3 class="text-sm font-bold text-white tracking-wide">Live Inbound Stream</h3>
      <p class="mt-1.5 max-w-sm text-xs text-zinc-400 leading-relaxed">
        Incoming code snippets, commands, and file transfers dispatched to this machine over the local subnet will appear here in real time.
      </p>
      <div class="mt-5 flex items-center space-x-2 text-[11px] text-zinc-500 bg-[#0b0f14] px-3 py-1.5 rounded-md border border-[#21262d]">
        <span class="h-2 w-2 rounded-full bg-emerald-400 animate-ping"></span>
        <span class="font-mono">Auto-accepting incoming drops to %USERPROFILE%\Downloads\DevDrop\</span>
      </div>
    </div>
  {:else}
    <!-- Stream Header -->
    <div class="flex items-center justify-between px-1 text-[11px] font-semibold uppercase tracking-wider text-zinc-400">
      <span class="flex items-center gap-1.5">
        <Activity class="h-3.5 w-3.5 text-emerald-400" />
        Activity Stream ({appState.drops.length})
      </span>
      <span class="text-[10px] text-zinc-500">Auto-saved</span>
    </div>

    <!-- Feed Cards -->
    <div class="space-y-3">
      {#each appState.drops as item (item.id)}
        {#if item.type === 'snippet'}
          <SnippetCard {item} />
        {:else if item.type === 'file'}
          <FileCard {item} />
        {/if}
      {/each}
    </div>
  {/if}
</div>
