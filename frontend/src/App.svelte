<script lang="ts">
  import { onMount } from 'svelte'
  import { GetSelf, OpenDownloadsFolder } from '../wailsjs/go/main/App.js'
  import type { backend } from '../wailsjs/go/models'

  let selfPeer = $state<backend.Peer | null>(null)
  let statusText = $state<string>('Initializing Castpipe engine...')

  onMount(async () => {
    try {
      selfPeer = await GetSelf()
      statusText = 'Castpipe engine online'
    } catch (e) {
      statusText = `Error: ${e}`
    }
  })
</script>

<main class="flex h-screen w-screen flex-col bg-[#0b0f14] text-[#e6edf3] font-mono select-none">
  <div class="flex items-center justify-between border-b border-[#30363d] bg-[#161b22] px-4 py-2 text-xs">
    <div class="flex items-center space-x-2">
      <span class="inline-block h-2.5 w-2.5 rounded-full bg-emerald-500 animate-pulse"></span>
      <span class="font-bold tracking-wider text-emerald-400">CASTPIPE</span>
      <span class="text-zinc-500">|</span>
      <span class="text-zinc-400">{statusText}</span>
    </div>
    {#if selfPeer}
      <div class="flex items-center space-x-3 text-zinc-400 text-[11px]">
        <span>HOST: <strong class="text-white">{selfPeer.hostname}</strong></span>
        <span>IP: <strong class="text-emerald-400">{selfPeer.ip}:{selfPeer.port}</strong></span>
        <button
          onclick={() => OpenDownloadsFolder()}
          class="rounded bg-[#21262d] px-2 py-1 text-zinc-300 hover:bg-[#30363d] hover:text-white transition"
        >
          Open DevDrop
        </button>
      </div>
    {/if}
  </div>
  <div class="flex flex-1 items-center justify-center p-8 text-zinc-500 text-sm">
    Backend bridge connected. Preparing UI components...
  </div>
</main>
