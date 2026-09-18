<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { appState } from './lib/state.svelte'
  import PeerSidebar from './lib/components/PeerSidebar.svelte'
  import Feed from './lib/components/Feed.svelte'
  import ComposerDock from './lib/components/ComposerDock.svelte'
  import ToastContainer from './lib/components/ToastContainer.svelte'
  import { GetSelf, SendPaths } from '../wailsjs/go/main/App.js'
  import { EventsOn, OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime.js'
  import type { Peer, DropItem } from './lib/types'
  import { Terminal, Send, ArrowRight, ShieldCheck, Zap } from '@lucide/svelte'

  let unlistenPeers: (() => void) | null = null
  let unlistenDrops: (() => void) | null = null

  onMount(async () => {
    // 1. Initial Self Peer fetch
    try {
      const self = await GetSelf()
      if (self) {
        appState.selfPeer = self as Peer
      }
    } catch (e) {
      console.error('Failed to get self peer:', e)
    }

    // 2. Listen for peer updates from mDNS discovery
    unlistenPeers = EventsOn('peers:updated', (peerList: Peer[]) => {
      appState.setPeers(peerList)
    })

    // 3. Listen for inbound snippets and file drops
    unlistenDrops = EventsOn('drop:received', (item: DropItem) => {
      appState.addDrop(item)
    })

    // 4. Native drag and drop integration
    try {
      OnFileDrop(async (x: number, y: number, paths: string[]) => {
        if (!paths || paths.length === 0) return

        // Check if dropped directly onto a peer card
        const elementUnderCursor = document.elementFromPoint(x, y)
        const peerCard = elementUnderCursor?.closest('[data-peer-id]')
        const peerId = peerCard?.getAttribute('data-peer-id')

        let target = peerId ? appState.peers.find((p) => p.id === peerId) : null
        if (!target) {
          target = appState.selectedPeer
        }

        if (target) {
          appState.isSending = true
          appState.addToast(
            'info',
            'Streaming Drop',
            `Sending ${paths.length} item(s) to ${target.hostname}...`
          )
          try {
            await SendPaths(target.ip, target.port, paths)
            appState.addToast(
              'success',
              'Drop Complete',
              `Sent ${paths.length} item(s) to ${target.hostname}`
            )
          } catch (err: any) {
            appState.addToast('error', 'Drop Failed', err?.toString() || 'Transfer error')
          } finally {
            appState.isSending = false
          }
        } else {
          appState.addToast(
            'error',
            'No Target Peer',
            'Select a peer card first or drop directly onto a peer in the sidebar'
          )
        }
      }, false)
    } catch (e) {
      console.warn('Native OnFileDrop not supported in browser dev server mode')
    }
  })

  onDestroy(() => {
    if (unlistenPeers) unlistenPeers()
    if (unlistenDrops) unlistenDrops()
    try {
      OnFileDropOff()
    } catch (_) {}
  })
</script>

<div class="flex h-screen w-screen overflow-hidden bg-[#0b0f14] text-[#e6edf3] font-mono select-none">
  <!-- Left Sidebar (Peer List & Status) -->
  <PeerSidebar />

  <!-- Main Content Area (Feed & Composer placeholder for Milestones 6 & 7) -->
  <div class="flex flex-1 flex-col overflow-hidden bg-[#0d1117]/60">
    <!-- Top Active Target Bar -->
    <header class="flex h-12 items-center justify-between border-b border-[#30363d] bg-[#161b22]/90 px-4">
      <div class="flex items-center space-x-3 text-xs">
        <div class="flex items-center space-x-1.5 text-zinc-400">
          <Terminal class="h-4 w-4 text-emerald-400" />
          <span class="font-bold text-zinc-300">ACTIVE TARGET:</span>
        </div>
        {#if appState.selectedPeer}
          <div class="flex items-center space-x-2 rounded bg-emerald-950/40 px-2.5 py-1 text-emerald-400 border border-emerald-800/40">
            <span class="h-2 w-2 rounded-full bg-emerald-400 animate-pulse"></span>
            <span class="font-bold text-white">{appState.selectedPeer.hostname}</span>
            <span class="text-zinc-500 font-mono text-[11px]">({appState.selectedPeer.ip}:{appState.selectedPeer.port})</span>
          </div>
        {:else}
          <div class="flex items-center space-x-1.5 text-zinc-500 italic text-xs">
            <span>No peer selected. Select a node from the sidebar to stream drops.</span>
          </div>
        {/if}
      </div>

      <div class="flex items-center space-x-2 text-[11px] text-zinc-400">
        <span class="flex items-center gap-1 rounded bg-[#21262d] px-2 py-0.5 border border-[#30363d]">
          <ShieldCheck class="h-3 w-3 text-emerald-400" />
          Trusted LAN
        </span>
        <span class="flex items-center gap-1 rounded bg-[#21262d] px-2 py-0.5 border border-[#30363d]">
          <Zap class="h-3 w-3 text-cyan-400" />
          Zero-Config
        </span>
      </div>
    </header>

    <!-- Main Live Inbound Feed Viewport -->
    <Feed />

    <!-- Bottom Composer Dock -->
    <ComposerDock />
  </div>

  <!-- Toast Notification System -->
  <ToastContainer />
</div>
