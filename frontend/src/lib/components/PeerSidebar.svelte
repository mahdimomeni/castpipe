<script lang="ts">
  import { appState } from '../state.svelte'
  import { OpenDownloadsFolder, SendPaths } from '../../../wailsjs/go/main/App.js'
  import type { Peer } from '../types'
  import { 
    Monitor, 
    Laptop, 
    Wifi, 
    FolderDown, 
    Radio, 
    CheckCircle2, 
    ArrowUpRight,
    UploadCloud,
    Folder
  } from '@lucide/svelte'

  let dragOverPeerId = $state<string | null>(null)
  let isDropping = $state(false)

  async function handleOpenDownloads() {
    try {
      await OpenDownloadsFolder()
    } catch (e: any) {
      appState.addToast('error', 'Open DevDrop Failed', e?.toString() || 'Unknown error')
    }
  }

  function handlePeerDragOver(e: DragEvent, peer: Peer) {
    e.preventDefault()
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'copy'
    }
    dragOverPeerId = peer.id
    appState.activeDragPeerId = peer.id
  }

  function handlePeerDragLeave(e: DragEvent, peer: Peer) {
    e.preventDefault()
    if (dragOverPeerId === peer.id) {
      dragOverPeerId = null
      appState.activeDragPeerId = null
    }
  }

  async function handlePeerDrop(e: DragEvent, peer: Peer) {
    e.preventDefault()
    dragOverPeerId = null
    appState.activeDragPeerId = null

    // Check if files were dropped via DOM event
    if (!e.dataTransfer || !e.dataTransfer.files || e.dataTransfer.files.length === 0) {
      return
    }

    const files = Array.from(e.dataTransfer.files)
    // On desktop webviews (Wails), files often have a path property
    const paths: string[] = []
    for (const f of files) {
      const p = (f as any).path
      if (p) paths.push(p)
    }

    if (paths.length > 0) {
      await dispatchFilesToPeer(peer, paths)
    } else {
      appState.addToast(
        'info',
        'Drop Received',
        `Sending ${files.length} file(s) to ${peer.hostname}`
      )
    }
  }

  async function dispatchFilesToPeer(peer: Peer, paths: string[]) {
    isDropping = true
    appState.isSending = true
    try {
      appState.addToast('info', 'Transferring', `Sending ${paths.length} item(s) to ${peer.hostname}...`)
      await SendPaths(peer.ip, peer.port, paths)
      appState.addToast('success', 'Transfer Complete', `Sent ${paths.length} item(s) to ${peer.hostname}`)
    } catch (err: any) {
      appState.addToast('error', 'Transfer Failed', err?.toString() || 'Upload error')
    } finally {
      isDropping = false
      appState.isSending = false
    }
  }
</script>

<aside class="flex w-80 flex-col border-r border-[#30363d] bg-[#0d1117] select-none h-full">
  <!-- Top Application Header -->
  <div class="flex items-center justify-between border-b border-[#30363d] p-3.5 bg-[#161b22]/70">
    <div class="flex items-center space-x-2.5">
      <div class="flex h-7 w-7 items-center justify-center rounded border border-emerald-500/40 bg-emerald-950/40 text-emerald-400 shadow-[0_0_10px_rgba(16,185,129,0.2)]">
        <Radio class="h-4 w-4 animate-pulse text-emerald-400" />
      </div>
      <div>
        <h1 class="text-xs font-black tracking-widest text-white flex items-center gap-1.5">
          CASTPIPE
          <span class="rounded bg-emerald-500/10 px-1 py-0.2 text-[9px] font-semibold text-emerald-400 border border-emerald-500/20">
            LAN P2P
          </span>
        </h1>
        <p class="text-[10px] text-zinc-400 tracking-tight">Zero-Config Dev Drops</p>
      </div>
    </div>
    <div class="flex items-center space-x-1.5 text-[10px] text-emerald-400 bg-emerald-950/30 px-2 py-0.5 rounded border border-emerald-800/40">
      <span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-400 animate-ping"></span>
      <span class="font-semibold uppercase tracking-wider text-[9px]">ONLINE</span>
    </div>
  </div>

  <!-- Local Node Card -->
  <div class="p-3 border-b border-[#30363d] bg-[#161b22]/40">
    <div class="flex items-center justify-between text-[10px] font-semibold tracking-wider text-zinc-400 mb-1.5 uppercase">
      <span class="flex items-center gap-1">
        <Monitor class="h-3 w-3 text-emerald-400" />
        This Device (Self)
      </span>
      <span class="text-[9px] text-emerald-400/80 bg-emerald-950/40 px-1.5 py-0.5 rounded border border-emerald-800/30">
        BROADCASTING
      </span>
    </div>

    {#if appState.selfPeer}
      <div class="rounded-md border border-[#30363d] bg-[#0b0f14] p-2.5 shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-bold text-white truncate max-w-[160px]" title={appState.selfPeer.hostname}>
            {appState.selfPeer.hostname}
          </span>
          <span class="text-[10px] font-mono text-zinc-400 bg-[#161b22] px-1.5 py-0.5 rounded border border-[#30363d]">
            :{appState.selfPeer.port}
          </span>
        </div>
        <div class="mt-1 flex items-center justify-between text-[11px] font-mono text-zinc-400">
          <span class="text-emerald-400/90 font-medium">{appState.selfPeer.ip}</span>
          <button
            onclick={handleOpenDownloads}
            class="flex items-center gap-1 rounded bg-[#21262d] hover:bg-[#30363d] text-zinc-300 hover:text-white px-2 py-0.5 text-[10px] transition border border-[#30363d] hover:border-zinc-500 shadow-sm"
            title="Open DevDrop downloads folder"
          >
            <FolderDown class="h-3 w-3 text-emerald-400" />
            <span>DevDrop</span>
          </button>
        </div>
      </div>
    {:else}
      <div class="rounded-md border border-dashed border-[#30363d] p-3 text-center text-xs text-zinc-500 animate-pulse">
        Binding network interface...
      </div>
    {/if}
  </div>

  <!-- Discovered Peer Nodes List -->
  <div class="flex-1 overflow-y-auto p-3 flex flex-col">
    <div class="flex items-center justify-between text-[10px] font-semibold tracking-wider text-zinc-400 mb-2 uppercase">
      <span class="flex items-center gap-1.5">
        <Wifi class="h-3 w-3 text-cyan-400" />
        Discovered Peers ({appState.remotePeers.length})
      </span>
      <span class="text-[9px] text-zinc-500">mDNS 5353</span>
    </div>

    {#if appState.remotePeers.length === 0}
      <!-- Empty Scanning Radar State -->
      <div class="my-auto flex flex-col items-center justify-center rounded-lg border border-dashed border-[#30363d] bg-[#161b22]/20 p-6 text-center">
        <div class="relative mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-cyan-950/30 border border-cyan-500/20 text-cyan-400">
          <Radio class="h-6 w-6 text-cyan-400 animate-pulse" />
          <span class="absolute inset-0 rounded-full border border-cyan-400/30 animate-ping"></span>
        </div>
        <p class="text-xs font-bold text-zinc-300">Listening on Subnet</p>
        <p class="mt-1 text-[11px] text-zinc-500 leading-relaxed max-w-[200px]">
          Peers running Castpipe on your Wi-Fi or local network appear automatically.
        </p>
        <div class="mt-3 flex items-center gap-1.5 text-[9px] text-zinc-500 bg-[#0b0f14] px-2 py-1 rounded border border-[#21262d]">
          <span class="h-1.5 w-1.5 rounded-full bg-cyan-400 animate-pulse"></span>
          <span>_devdrop._tcp.local</span>
        </div>
      </div>
    {:else}
      <div class="space-y-2">
        {#each appState.remotePeers as peer (peer.id)}
          {@const isSelected = appState.selectedPeer?.id === peer.id}
          {@const isDragHover = dragOverPeerId === peer.id}

          <div
            data-peer-id={peer.id}
            role="button"
            tabindex="0"
            onclick={() => appState.selectPeer(peer)}
            onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') appState.selectPeer(peer) }}
            ondragover={(e) => handlePeerDragOver(e, peer)}
            ondragleave={(e) => handlePeerDragLeave(e, peer)}
            ondrop={(e) => handlePeerDrop(e, peer)}
            class="group relative flex flex-col rounded-md border p-2.5 transition cursor-pointer text-left {
              isDragHover
                ? 'border-cyan-400 bg-cyan-950/40 ring-2 ring-cyan-400/40 shadow-[0_0_15px_rgba(6,182,212,0.3)]'
                : isSelected
                ? 'border-emerald-500/80 bg-emerald-950/20 shadow-[0_0_12px_rgba(16,185,129,0.15)] ring-1 ring-emerald-500/30'
                : 'border-[#30363d] bg-[#161b22]/60 hover:border-zinc-500 hover:bg-[#161b22]'
            }"
          >
            <!-- Drag & Drop Hover Overlay -->
            {#if isDragHover}
              <div class="absolute inset-0 z-10 flex items-center justify-center rounded-md bg-cyan-950/90 backdrop-blur-sm border-2 border-dashed border-cyan-400 text-cyan-200">
                <div class="flex items-center gap-2 text-xs font-bold animate-bounce">
                  <UploadCloud class="h-4 w-4 text-cyan-400" />
                  <span>DROP TO SEND TO {peer.hostname.toUpperCase()}</span>
                </div>
              </div>
            {/if}

            <div class="flex items-center justify-between">
              <div class="flex items-center space-x-2 min-w-0">
                <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded {
                  isSelected ? 'bg-emerald-500/20 text-emerald-400' : 'bg-[#21262d] text-zinc-400 group-hover:text-zinc-200'
                }">
                  <Laptop class="h-3.5 w-3.5" />
                </div>
                <span class="text-xs font-bold text-white truncate max-w-[130px]" title={peer.hostname}>
                  {peer.hostname}
                </span>
              </div>

              {#if isSelected}
                <span class="flex items-center gap-1 rounded bg-emerald-500/20 px-1.5 py-0.5 text-[9px] font-bold text-emerald-400 border border-emerald-500/30">
                  <CheckCircle2 class="h-2.5 w-2.5" />
                  TARGET
                </span>
              {:else}
                <span class="text-[9px] text-zinc-500 group-hover:text-zinc-400 transition">
                  Select
                </span>
              {/if}
            </div>

            <!-- Network Details -->
            <div class="mt-2 flex items-center justify-between text-[11px] font-mono">
              <span class="text-zinc-400 font-medium">{peer.ip}</span>
              <span class="text-zinc-500">:{peer.port}</span>
            </div>

            <!-- Drag hint indicator -->
            <div class="mt-1.5 flex items-center justify-between pt-1.5 border-t border-[#21262d]/60 text-[9px] text-zinc-500">
              <span class="flex items-center gap-1">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                <span>Ready</span>
              </span>
              <span class="group-hover:text-cyan-400/80 transition flex items-center gap-0.5">
                Drop files here
                <ArrowUpRight class="h-2.5 w-2.5" />
              </span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Bottom Subnet Info Footer -->
  <div class="border-t border-[#30363d] bg-[#161b22]/80 p-2.5 text-[10px] text-zinc-500 flex items-center justify-between">
    <span>Castpipe v1.0.0</span>
    <span class="flex items-center gap-1">
      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>
      Subnet Active
    </span>
  </div>
</aside>
