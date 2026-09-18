export interface Peer {
  id: string
  hostname: string
  ip: string
  port: number
  isSelf: boolean
  lastSeen?: number
}

export interface DropSnippetPayload {
  id: string
  sender: string
  senderIp: string
  syntax: string
  content: string
  timestamp: number
}

export interface DropFilePayload {
  id: string
  sender: string
  senderIp: string
  fileName: string
  fileSize: number
  filePath: string
  isArchive: boolean
  timestamp: number
}

export interface DropItem {
  id: string
  type: 'snippet' | 'file'
  sender: string
  senderIp: string
  timestamp: number
  snippet?: DropSnippetPayload
  file?: DropFilePayload
}

export interface TransferToast {
  id: string
  type: 'info' | 'success' | 'error'
  title: string
  message: string
}
