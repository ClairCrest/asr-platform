import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { apiBaseUrl } from '../api/client'
import { createWsTicket } from '../api/auth'
import { useAuth } from '../auth/AuthContext'

const POLL_INTERVAL_MS = 5000
const MAX_BACKOFF_MS = 30000

interface WsEvent {
  job_id: string
  event_type: string
  payload: Record<string, unknown>
  created_at: string
}

// Subscribes to the authenticated user's job events over WebSocket and
// invalidates the affected React Query cache entries so job status
// changes appear without a page refresh. Reconnects with exponential
// backoff, and polls every 5s whenever the socket is down so the
// dashboard still catches up if WebSocket connectivity never recovers.
export function useJobStream(): void {
  const { user } = useAuth()
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!user) return

    const state = { closed: false, backoffMs: 1000 }
    let ws: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined
    let pollTimer: ReturnType<typeof setInterval> | undefined

    const startPolling = () => {
      if (pollTimer) return
      pollTimer = setInterval(() => {
        void queryClient.invalidateQueries({ queryKey: ['jobs'] })
      }, POLL_INTERVAL_MS)
    }
    const stopPolling = () => {
      if (pollTimer) {
        clearInterval(pollTimer)
        pollTimer = undefined
      }
    }

    const scheduleReconnect = () => {
      if (state.closed) return
      startPolling()
      reconnectTimer = setTimeout(() => void connect(), state.backoffMs)
      state.backoffMs = Math.min(state.backoffMs * 2, MAX_BACKOFF_MS)
    }

    const connect = async () => {
      if (state.closed) return
      try {
        const { ticket } = await createWsTicket()
        if (state.closed) return

        const wsUrl = `${apiBaseUrl().replace(/^http/, 'ws')}/ws?ticket=${encodeURIComponent(ticket)}`
        ws = new WebSocket(wsUrl)

        ws.onopen = () => {
          state.backoffMs = 1000
          stopPolling()
        }
        ws.onmessage = (event) => {
          let parsed: WsEvent
          try {
            parsed = JSON.parse(event.data as string) as WsEvent
          } catch {
            return
          }
          void queryClient.invalidateQueries({ queryKey: ['jobs'] })
          void queryClient.invalidateQueries({ queryKey: ['job', parsed.job_id] })
        }
        ws.onclose = () => scheduleReconnect()
        ws.onerror = () => ws?.close()
      } catch {
        scheduleReconnect()
      }
    }

    void connect()

    return () => {
      state.closed = true
      ws?.close()
      if (reconnectTimer) clearTimeout(reconnectTimer)
      stopPolling()
    }
  }, [user, queryClient])
}
