import type { StreamEvent, StreamType } from '@/types/domain'
import { getAuthToken } from '@/lib/api'

type EventHandler = (event: StreamEvent) => void
type ErrorHandler = (error: Event) => void

function normalizeType(type: string): StreamType {
  const upper = type.toUpperCase()
  if (
    upper === 'THINK' ||
    upper === 'TOOL' ||
    upper === 'CHUNK' ||
    upper === 'STATUS' ||
    upper === 'FINISH'
  ) {
    return upper
  }
  return 'STATUS'
}

export function createSSE(
  url: string,
  key: string,
  onEvent: EventHandler,
  onError?: ErrorHandler,
) {
  let source: EventSource | null = null
  let reconnectTimer: number | null = null
  let closed = false
  let reconnectDelay = 1000
  let lastFinishAt = 0
  let disconnectedNotified = false

  const buildUrl = () => {
    const lastSeq = window.localStorage.getItem(key) ?? '0'
    const fullUrl = new URL(url)
    fullUrl.searchParams.set('last_seq', lastSeq)
    const token = getAuthToken()
    if (token) {
      fullUrl.searchParams.set('access_token', token)
    }
    return fullUrl.toString()
  }

  const onMessage = (message: MessageEvent<string>) => {
    const type = normalizeType(message.type || 'STATUS')
    let payload: { seq?: number; msgType?: string; type?: string; content?: string } = {}
    try {
      payload = JSON.parse(message.data)
    } catch {
      payload = { content: message.data }
    }
    const seq = Number(payload.seq ?? message.lastEventId ?? 0)
    if (seq > 0) {
      window.localStorage.setItem(key, String(seq))
    }
    const streamType = normalizeType(payload.msgType ?? payload.type ?? type)
    if (streamType === 'FINISH') {
      lastFinishAt = Date.now()
    }
    onEvent({
      seq,
      msgType: streamType,
      content: payload.content ?? '',
    })
  }

  const bindEvent = (target: EventSource, eventName: StreamType) => {
    target.addEventListener(eventName.toLowerCase(), onMessage as EventListener)
    target.addEventListener(eventName, onMessage as EventListener)
  }

  const scheduleReconnect = () => {
    if (closed || reconnectTimer) {
      return
    }
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      if (!closed) {
        connect()
      }
    }, reconnectDelay)
    reconnectDelay = Math.min(8000, reconnectDelay * 2)
  }

  const connect = () => {
    if (closed) {
      return
    }
    const target = new EventSource(buildUrl())
    source = target
    bindEvent(target, 'THINK')
    bindEvent(target, 'TOOL')
    bindEvent(target, 'CHUNK')
    bindEvent(target, 'STATUS')
    bindEvent(target, 'FINISH')
    target.onmessage = onMessage
    target.onopen = () => {
      reconnectDelay = 1000
      disconnectedNotified = false
    }
    target.onerror = (event) => {
      target.close()
      if (closed) {
        return
      }
      if (Date.now() - lastFinishAt > 1200 && !disconnectedNotified) {
        disconnectedNotified = true
        onError?.(event)
      }
      scheduleReconnect()
    }
  }

  connect()

  return () => {
    closed = true
    if (reconnectTimer) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    source?.close()
    source = null
  }
}
