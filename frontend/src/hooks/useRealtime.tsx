import { useEffect, useRef, useCallback } from 'react'
import { useAuth } from './useAuth'

// SSE Event Types (must match backend)
export const SSE_EVENTS = {
    CONFIG_UPDATE: 'CONFIG_UPDATE',
    FORCE_LOGOUT: 'FORCE_LOGOUT',
    SESSION_REVOKED: 'SESSION_REVOKED',
    USER_DELETED: 'USER_DELETED',
    USER_DEACTIVATED: 'USER_DEACTIVATED',
}

interface SSEEvent {
    type: string
    payload?: Record<string, unknown>
    timestamp: string
}

interface SSEEventHandlers {
    onConfigUpdate?: (key: string, value: unknown) => void
    onForceLogout?: (reason: string) => void
    onConnected?: (clientId: string) => void
    onError?: (error: Event) => void
}

export function useRealtime(handlers: SSEEventHandlers = {}) {
    const { user, logout } = useAuth()
    const eventSourceRef = useRef<EventSource | null>(null)
    const reconnectTimeoutRef = useRef<number | null>(null)
    const reconnectAttempts = useRef(0)
    const maxReconnectAttempts = 5

    const connect = useCallback(() => {
        if (!user) return

        // Close existing connection
        if (eventSourceRef.current) {
            eventSourceRef.current.close()
        }

        // Get token from localStorage for SSE auth
        const token = localStorage.getItem('access_token')
        if (!token) return

        // Create SSE connection with auth token in URL
        const url = `/api/realtime/subscribe?token=${encodeURIComponent(token)}`
        const eventSource = new EventSource(url, { withCredentials: true })
        eventSourceRef.current = eventSource

        // Handle connection opened
        eventSource.addEventListener('connected', (e: MessageEvent) => {
            reconnectAttempts.current = 0
            try {
                const data = JSON.parse(e.data)
                handlers.onConnected?.(data.client_id)
            } catch {
                // Ignore parse errors
            }
        })

        // Handle config updates
        eventSource.addEventListener(SSE_EVENTS.CONFIG_UPDATE, (e: MessageEvent) => {
            try {
                const data: SSEEvent = JSON.parse(e.data)
                if (data.payload) {
                    handlers.onConfigUpdate?.(
                        data.payload.key as string,
                        data.payload.value
                    )
                }
            } catch {
                console.error('Failed to parse CONFIG_UPDATE event')
            }
        })

        // Handle force logout
        eventSource.addEventListener(SSE_EVENTS.FORCE_LOGOUT, (e: MessageEvent) => {
            try {
                const data: SSEEvent = JSON.parse(e.data)
                const reason = (data.payload?.reason as string) || 'Your session has been terminated'
                handlers.onForceLogout?.(reason)
                // Auto logout
                logout()
                alert(reason)
            } catch {
                logout()
            }
        })

        // Handle session revoked
        eventSource.addEventListener(SSE_EVENTS.SESSION_REVOKED, (e: MessageEvent) => {
            try {
                const data: SSEEvent = JSON.parse(e.data)
                const reason = (data.payload?.reason as string) || 'Session revoked by administrator'
                handlers.onForceLogout?.(reason)
                logout()
                alert(reason)
            } catch {
                logout()
            }
        })

        // Handle connection errors
        eventSource.onerror = (error) => {
            handlers.onError?.(error)
            eventSource.close()

            // Attempt reconnect with exponential backoff
            if (reconnectAttempts.current < maxReconnectAttempts) {
                const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current), 30000)
                reconnectAttempts.current++
                reconnectTimeoutRef.current = window.setTimeout(connect, delay)
            }
        }
    }, [user, logout, handlers])

    // Connect on mount, disconnect on unmount
    useEffect(() => {
        connect()

        return () => {
            if (eventSourceRef.current) {
                eventSourceRef.current.close()
            }
            if (reconnectTimeoutRef.current) {
                window.clearTimeout(reconnectTimeoutRef.current)
            }
        }
    }, [connect])

    // Expose reconnect function
    const reconnect = useCallback(() => {
        reconnectAttempts.current = 0
        connect()
    }, [connect])

    return { reconnect }
}

