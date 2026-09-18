import type { Peer, DropItem, TransferToast } from './types'

export class AppState {
  selfPeer = $state<Peer | null>(null)
  peers = $state<Peer[]>([])
  selectedPeerId = $state<string | null>(null)
  drops = $state<DropItem[]>([])
  activeDragPeerId = $state<string | null>(null)
  toasts = $state<TransferToast[]>([])
  isSending = $state<boolean>(false)

  // Derived reactive views
  remotePeers = $derived(this.peers.filter((p) => !p.isSelf))

  selectedPeer = $derived.by(() => {
    if (this.selectedPeerId) {
      const found = this.peers.find((p) => p.id === this.selectedPeerId && !p.isSelf)
      if (found) return found
    }
    // Default to first discovered remote peer if available
    return this.remotePeers.length > 0 ? this.remotePeers[0] : null
  })

  addToast(type: 'info' | 'success' | 'error', title: string, message: string) {
    const id = Math.random().toString(36).substring(2, 9)
    this.toasts = [...this.toasts, { id, type, title, message }]
    setTimeout(() => {
      this.toasts = this.toasts.filter((t) => t.id !== id)
    }, 4500)
  }

  dismissToast(id: string) {
    this.toasts = this.toasts.filter((t) => t.id !== id)
  }

  setPeers(newPeers: Peer[]) {
    this.peers = newPeers
    const self = newPeers.find((p) => p.isSelf)
    if (self) {
      this.selfPeer = self
    }
    // Auto-select remote peer if none currently selected
    if (!this.selectedPeerId && this.remotePeers.length > 0) {
      this.selectedPeerId = this.remotePeers[0].id
    }
  }

  addDrop(item: DropItem) {
    if (!this.drops.some((d) => d.id === item.id)) {
      this.drops = [item, ...this.drops]
      if (item.type === 'snippet') {
        this.addToast(
          'info',
          `Snippet from ${item.sender}`,
          `${item.snippet?.syntax.toUpperCase() || 'CODE'} snippet received`
        )
      } else if (item.type === 'file') {
        this.addToast(
          'success',
          `File from ${item.sender}`,
          `Saved to Castpipe: ${item.file?.fileName || 'file'}`
        )
      }
    }
  }

  selectPeer(peer: Peer | null) {
    this.selectedPeerId = peer ? peer.id : null
  }
}

export const appState = new AppState()
