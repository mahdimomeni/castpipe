<script lang="ts">
  import { appState } from '../state.svelte'
  import { CheckCircle2, AlertCircle, Info, X } from '@lucide/svelte'
</script>

{#if appState.toasts.length > 0}
  <div class="fixed bottom-4 right-4 z-50 flex flex-col space-y-2 max-w-sm pointer-events-none">
    {#each appState.toasts as toast (toast.id)}
      <div
        class="pointer-events-auto flex items-start gap-2.5 rounded-lg border p-3 shadow-xl backdrop-blur-md transition transform duration-200 {
          toast.type === 'success'
            ? 'border-emerald-500/40 bg-[#0d1117]/95 text-emerald-300 ring-1 ring-emerald-500/20 shadow-[0_0_15px_rgba(16,185,129,0.15)]'
            : toast.type === 'error'
            ? 'border-rose-500/40 bg-[#0d1117]/95 text-rose-300 ring-1 ring-rose-500/20 shadow-[0_0_15px_rgba(244,63,94,0.15)]'
            : 'border-cyan-500/40 bg-[#0d1117]/95 text-cyan-300 ring-1 ring-cyan-500/20 shadow-[0_0_15px_rgba(6,182,212,0.15)]'
        }"
      >
        <div class="mt-0.5 shrink-0">
          {#if toast.type === 'success'}
            <CheckCircle2 class="h-4 w-4 text-emerald-400" />
          {:else if toast.type === 'error'}
            <AlertCircle class="h-4 w-4 text-rose-400" />
          {:else}
            <Info class="h-4 w-4 text-cyan-400" />
          {/if}
        </div>
        <div class="flex-1 min-w-0">
          <h4 class="text-xs font-bold text-white tracking-wide">{toast.title}</h4>
          <p class="mt-0.5 text-[11px] text-zinc-400 leading-snug break-words font-mono">
            {toast.message}
          </p>
        </div>
        <button
          onclick={() => appState.dismissToast(toast.id)}
          class="shrink-0 text-zinc-500 hover:text-zinc-300 transition"
        >
          <X class="h-3.5 w-3.5" />
        </button>
      </div>
    {/each}
  </div>
{/if}
