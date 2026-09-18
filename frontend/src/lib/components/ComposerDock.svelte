<script lang="ts">
  import { appState } from '../state.svelte'
  import { SendSnippet, SendPaths } from '../../../wailsjs/go/main/App.js'
  import { 
    Send, 
    Code2, 
    FileUp, 
    FolderUp, 
    Sparkles, 
    Terminal, 
    UploadCloud, 
    X,
    ChevronDown,
    CornerDownLeft
  } from '@lucide/svelte'

  let activeTab = $state<'snippet' | 'files'>('snippet')
  let content = $state<string>('')
  let syntax = $state<string>('bash')
  let isSending = $state<boolean>(false)
  let isDragHover = $state<boolean>(false)

  let fileInput = $state<HTMLInputElement | null>(null)
  let folderInput = $state<HTMLInputElement | null>(null)

  const targetPeer = $derived(appState.selectedPeer)
  const canDispatch = $derived(
    Boolean(targetPeer) && content.trim().length > 0 && !isSending
  )

  async function handleDispatchSnippet() {
    if (!canDispatch || !targetPeer) return

    isSending = true
    appState.isSending = true
    const currentContent = content
    const currentSyntax = syntax

    try {
      appState.addToast(
        'info',
        'Dispatching Snippet',
        `Sending ${currentSyntax.toUpperCase()} snippet to ${targetPeer.hostname}...`
      )
      await SendSnippet(targetPeer.ip, targetPeer.port, currentContent, currentSyntax)
      appState.addToast(
        'success',
        'Snippet Sent',
        `Successfully delivered to ${targetPeer.hostname}`
      )
      content = ''
    } catch (err: any) {
      appState.addToast('error', 'Dispatch Failed', err?.toString() || 'Failed sending snippet')
    } finally {
      isSending = false
      appState.isSending = false
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    // Ctrl+Enter or Cmd+Enter to dispatch
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      handleDispatchSnippet()
    }
  }

  function handleDropOver(e: DragEvent) {
    e.preventDefault()
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'copy'
    }
    isDragHover = true
  }

  function handleDropLeave(e: DragEvent) {
    e.preventDefault()
    isDragHover = false
  }

  async function handleDropFiles(e: DragEvent) {
    e.preventDefault()
    isDragHover = false

    if (!targetPeer) {
      appState.addToast('error', 'No Target Peer', 'Select a peer from the sidebar first')
      return
    }

    if (!e.dataTransfer || !e.dataTransfer.files || e.dataTransfer.files.length === 0) {
      return
    }

    const files = Array.from(e.dataTransfer.files)
    const paths: string[] = []
    for (const f of files) {
      const p = (f as any).path
      if (p) paths.push(p)
    }

    if (paths.length > 0) {
      await sendPathsList(paths)
    } else {
      appState.addToast('info', 'Drop Received', `Transferring ${files.length} item(s)...`)
    }
  }

  async function handleFileInputChange(e: Event) {
    const input = e.target as HTMLInputElement
    if (!input.files || input.files.length === 0 || !targetPeer) return

    const paths: string[] = []
    for (let i = 0; i < input.files.length; i++) {
      const f = input.files[i] as any
      if (f.path) paths.push(f.path)
    }

    if (paths.length > 0) {
      await sendPathsList(paths)
    }
    input.value = ''
  }

  async function sendPathsList(paths: string[]) {
    if (!targetPeer) return
    isSending = true
    appState.isSending = true

    try {
      appState.addToast(
        'info',
        'Streaming Drop',
        `Sending ${paths.length} item(s) to ${targetPeer.hostname}...`
      )
      await SendPaths(targetPeer.ip, targetPeer.port, paths)
      appState.addToast(
        'success',
        'Drop Complete',
        `Transferred ${paths.length} item(s) to ${targetPeer.hostname}`
      )
    } catch (err: any) {
      appState.addToast('error', 'Transfer Error', err?.toString() || 'Transfer failed')
    } finally {
      isSending = false
      appState.isSending = false
    }
  }
</script>

<section class="border-t border-[#30363d] bg-[#161b22]/95 backdrop-blur-md p-3 shadow-2xl">
  <!-- Mode Selector & Header Toolbar -->
  <div class="mb-2.5 flex items-center justify-between">
    <div class="flex items-center space-x-2">
      <!-- Mode Tabs -->
      <div class="flex items-center rounded-lg bg-[#0b0f14] p-0.5 border border-[#30363d]">
        <button
          onclick={() => (activeTab = 'snippet')}
          class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold transition {
            activeTab === 'snippet'
              ? 'bg-[#21262d] text-emerald-400 shadow-sm'
              : 'text-zinc-400 hover:text-zinc-200'
          }"
        >
          <Code2 class="h-3.5 w-3.5" />
          <span>Code / Command</span>
        </button>

        <button
          onclick={() => (activeTab = 'files')}
          class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold transition {
            activeTab === 'files'
              ? 'bg-[#21262d] text-cyan-400 shadow-sm'
              : 'text-zinc-400 hover:text-zinc-200'
          }"
        >
          <UploadCloud class="h-3.5 w-3.5" />
          <span>File Drop Zone</span>
        </button>
      </div>

      <!-- Syntax Dropdown (Visible in Snippet Tab) -->
      {#if activeTab === 'snippet'}
        <div class="relative flex items-center">
          <select
            bind:value={syntax}
            class="h-7 appearance-none rounded-md bg-[#0b0f14] pl-2.5 pr-7 text-xs font-mono font-medium text-emerald-400 border border-[#30363d] hover:border-zinc-500 focus:outline-none focus:border-emerald-500 transition cursor-pointer"
          >
            <option value="bash">bash / shell</option>
            <option value="json">json</option>
            <option value="yaml">yaml</option>
            <option value="sql">sql</option>
            <option value="javascript">javascript</option>
            <option value="typescript">typescript</option>
            <option value="plain">plain text</option>
          </select>
          <ChevronDown class="pointer-events-none absolute right-2 h-3.5 w-3.5 text-zinc-500" />
        </div>
      {/if}
    </div>

    <!-- Active Target Badge in Dock -->
    <div class="flex items-center space-x-2 text-xs">
      {#if targetPeer}
        <span class="flex items-center gap-1.5 rounded-md bg-emerald-950/50 px-2.5 py-1 text-[11px] font-semibold text-emerald-300 border border-emerald-800/40">
          <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
          <span>Target: <strong>{targetPeer.hostname}</strong></span>
        </span>
      {:else}
        <span class="rounded-md bg-rose-950/40 px-2.5 py-1 text-[11px] font-medium text-rose-300 border border-rose-800/40">
          Select a peer to dispatch
        </span>
      {/if}
    </div>
  </div>

  <!-- Content Areas -->
  {#if activeTab === 'snippet'}
    <!-- Textarea Input Area -->
    <div class="relative rounded-lg border border-[#30363d] bg-[#0b0f14] focus-within:border-emerald-500/70 transition shadow-inner">
      <textarea
        bind:value={content}
        onkeydown={handleKeydown}
        placeholder="Paste code snippet, JSON payload, or shell command... (Ctrl+Enter to dispatch)"
        rows="3"
        class="w-full resize-none bg-transparent p-3 text-xs font-mono text-[#e6edf3] placeholder:text-zinc-600 focus:outline-none leading-relaxed"
      ></textarea>

      <!-- Textarea Footer Toolbar -->
      <div class="flex items-center justify-between border-t border-[#21262d] px-3 py-1.5 text-[11px] bg-[#0d1117]/60">
        <div class="flex items-center space-x-3 text-zinc-500">
          <span>{content.length} chars</span>
          <span>&bull;</span>
          <span>{content.split('\n').length} lines</span>
          {#if content.length > 0}
            <button
              onclick={() => (content = '')}
              class="hover:text-zinc-300 transition"
              title="Clear input"
            >
              Clear
            </button>
          {/if}
        </div>

        <button
          onclick={handleDispatchSnippet}
          disabled={!canDispatch}
          class="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-bold transition shadow-sm {
            canDispatch
              ? 'bg-emerald-500 hover:bg-emerald-400 text-black cursor-pointer shadow-[0_0_12px_rgba(16,185,129,0.3)]'
              : 'bg-[#21262d] text-zinc-500 cursor-not-allowed border border-[#30363d]'
          }"
        >
          {#if isSending}
            <span class="inline-block h-3 w-3 animate-spin rounded-full border-2 border-black border-t-transparent"></span>
            <span>Sending...</span>
          {:else}
            <Send class="h-3.5 w-3.5" />
            <span>Dispatch</span>
            <span class="ml-1 text-[9px] font-normal opacity-70 flex items-center">
              (Ctrl+<CornerDownLeft class="h-2.5 w-2.5 inline" />)
            </span>
          {/if}
        </button>
      </div>
    </div>
  {:else}
    <!-- Dedicated File & Folder Drop Zone -->
    <!-- Hidden file inputs for manual click browsing -->
    <input
      type="file"
      multiple
      bind:this={fileInput}
      onchange={handleFileInputChange}
      class="hidden"
    />
    <input
      type="file"
      bind:this={folderInput}
      webkitdirectory
      onchange={handleFileInputChange}
      class="hidden"
    />

    <div
      role="region"
      aria-label="File transfer drop zone"
      ondragover={handleDropOver}
      ondragleave={handleDropLeave}
      ondrop={handleDropFiles}
      class="relative flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-6 transition {
        isDragHover
          ? 'border-cyan-400 bg-cyan-950/40 shadow-[0_0_20px_rgba(6,182,212,0.3)]'
          : 'border-[#30363d] bg-[#0b0f14] hover:border-zinc-500'
      }"
    >
      <div class="flex h-10 w-10 items-center justify-center rounded-full bg-cyan-950/30 text-cyan-400 border border-cyan-500/20 mb-2">
        <UploadCloud class="h-5 w-5 animate-pulse" />
      </div>

      <h4 class="text-xs font-bold text-white">
        {#if targetPeer}
          Drop files or folders to stream to <span class="text-cyan-400">{targetPeer.hostname}</span>
        {:else}
          Select a peer in the sidebar to stream files
        {/if}
      </h4>

      <p class="mt-1 text-[11px] text-zinc-500 text-center max-w-sm">
        Folders are zipped on-the-fly and streamed directly over the subnet without temp files.
      </p>

      <div class="mt-3.5 flex items-center space-x-2.5">
        <button
          onclick={() => fileInput?.click()}
          disabled={!targetPeer || isSending}
          class="flex items-center gap-1.5 rounded-md bg-[#21262d] hover:bg-[#30363d] text-zinc-200 px-3 py-1.5 text-xs font-medium transition border border-[#30363d] disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
        >
          <FileUp class="h-3.5 w-3.5 text-cyan-400" />
          <span>Browse Files</span>
        </button>

        <button
          onclick={() => folderInput?.click()}
          disabled={!targetPeer || isSending}
          class="flex items-center gap-1.5 rounded-md bg-[#21262d] hover:bg-[#30363d] text-zinc-200 px-3 py-1.5 text-xs font-medium transition border border-[#30363d] disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
        >
          <FolderUp class="h-3.5 w-3.5 text-purple-400" />
          <span>Browse Folder (Auto-Zip)</span>
        </button>
      </div>
    </div>
  {/if}
</section>
