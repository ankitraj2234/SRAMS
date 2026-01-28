// Build timestamp: 2026-01-26T04:12:00 - Force rebuild with /api path
const API_BASE = '/api'
console.log('[SRAMS] API Service initialized at', new Date().toISOString())

interface RequestOptions {
    method?: string
    body?: unknown
    headers?: Record<string, string>
}

// Helper to get CSRF token from cookie
function getCsrfToken(): string | null {
    const cookies = document.cookie.split(';')
    for (const cookie of cookies) {
        const [name, value] = cookie.trim().split('=')
        if (name === 'csrf_token') {
            return decodeURIComponent(value)
        }
    }
    return null
}

class ApiService {
    private async request<T>(endpoint: string, options: RequestOptions = {}): Promise<{ data: T }> {
        const { method = 'GET', body, headers = {} } = options

        const token = localStorage.getItem('access_token')
        const deviceId = localStorage.getItem('device_id')
        const csrfToken = getCsrfToken()

        const requestHeaders: Record<string, string> = {
            'Content-Type': 'application/json',
            ...headers,
        }

        if (token) {
            requestHeaders['Authorization'] = `Bearer ${token}`
        }

        if (deviceId) {
            requestHeaders['X-Device-ID'] = deviceId
        }

        // Add CSRF token for mutating requests
        if (csrfToken && method !== 'GET') {
            requestHeaders['X-CSRF-Token'] = csrfToken
        }

        // Add desktop session token for Super Admin access gating
        const desktopSession = localStorage.getItem('desktop_session')
        if (desktopSession) {
            requestHeaders['X-Desktop-Session'] = desktopSession
        }

        const response = await fetch(`${API_BASE}${endpoint}`, {
            method,
            headers: requestHeaders,
            body: body ? JSON.stringify(body) : undefined,
            credentials: 'include',
        })

        // Handle token refresh
        if (response.status === 401) {
            const errorData = await response.json()
            if (errorData.code === 'TOKEN_EXPIRED') {
                const refreshed = await this.refreshToken()
                if (refreshed) {
                    return this.request(endpoint, options)
                }
            }
            throw new Error(errorData.error || 'Unauthorized')
        }

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}))
            throw new Error(errorData.error || `Request failed: ${response.status}`)
        }

        const data = await response.json()
        return { data }
    }

    private async refreshToken(): Promise<boolean> {
        const refreshToken = localStorage.getItem('refresh_token')
        if (!refreshToken) return false

        try {
            const response = await fetch(`${API_BASE}/auth/refresh`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ refresh_token: refreshToken }),
            })

            if (!response.ok) return false

            const data = await response.json()
            localStorage.setItem('access_token', data.access_token)
            localStorage.setItem('refresh_token', data.refresh_token)
            return true
        } catch {
            return false
        }
    }

    async get<T>(endpoint: string): Promise<{ data: T }> {
        return this.request<T>(endpoint)
    }

    async post<T>(endpoint: string, body?: unknown): Promise<{ data: T }> {
        return this.request<T>(endpoint, { method: 'POST', body })
    }

    async put<T>(endpoint: string, body?: unknown): Promise<{ data: T }> {
        return this.request<T>(endpoint, { method: 'PUT', body })
    }

    async delete<T>(endpoint: string): Promise<{ data: T }> {
        return this.request<T>(endpoint, { method: 'DELETE' })
    }

    // Download file as blob (for exports)
    async downloadBlob(endpoint: string): Promise<Blob> {
        const token = localStorage.getItem('access_token')
        const deviceId = localStorage.getItem('device_id')
        const desktopSession = localStorage.getItem('desktop_session')

        const headers: Record<string, string> = {}
        if (token) headers['Authorization'] = `Bearer ${token}`
        if (deviceId) headers['X-Device-ID'] = deviceId
        if (desktopSession) headers['X-Desktop-Session'] = desktopSession

        const response = await fetch(`${API_BASE}${endpoint}`, {
            method: 'GET',
            headers,
            credentials: 'include',
        })

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}))
            throw new Error(errorData.error || 'Download failed')
        }

        return response.blob()
    }

    // Upload FormData (for imports with multiple fields)
    async uploadFormData<T>(endpoint: string, formData: FormData): Promise<{ data: T }> {
        const token = localStorage.getItem('access_token')
        const deviceId = localStorage.getItem('device_id')
        const csrfToken = getCsrfToken()
        const desktopSession = localStorage.getItem('desktop_session')

        const headers: Record<string, string> = {}
        if (token) headers['Authorization'] = `Bearer ${token}`
        if (deviceId) headers['X-Device-ID'] = deviceId
        if (csrfToken) headers['X-CSRF-Token'] = csrfToken
        if (desktopSession) headers['X-Desktop-Session'] = desktopSession

        const response = await fetch(`${API_BASE}${endpoint}`, {
            method: 'POST',
            headers,
            body: formData,
            credentials: 'include',
        })

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}))
            throw new Error(errorData.error || 'Upload failed')
        }

        const data = await response.json()
        return { data }
    }

    async upload<T>(endpoint: string, file: File, title?: string): Promise<{ data: T }> {
        const token = localStorage.getItem('access_token')
        const deviceId = localStorage.getItem('device_id')
        const csrfToken = getCsrfToken()
        const desktopSession = localStorage.getItem('desktop_session')

        const formData = new FormData()
        formData.append('file', file)
        if (title) formData.append('title', title)

        // Build headers - don't set Content-Type for FormData, browser will set it with boundary
        const headers: Record<string, string> = {}

        if (token) {
            headers['Authorization'] = `Bearer ${token}`
        }

        if (deviceId) {
            headers['X-Device-ID'] = deviceId
        }

        // Add CSRF token for file uploads
        if (csrfToken) {
            headers['X-CSRF-Token'] = csrfToken
        }

        // Add desktop session for Super Admin access
        if (desktopSession) {
            headers['X-Desktop-Session'] = desktopSession
        }

        const response = await fetch(`${API_BASE}${endpoint}`, {
            method: 'POST',
            headers,
            body: formData,
            credentials: 'include',
        })

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}))
            throw new Error(errorData.error || 'Upload failed')
        }

        const data = await response.json()
        return { data }
    }

    // Log client-side events
    async logEvent(actionType: string, targetType?: string, targetId?: string, metadata?: Record<string, unknown>) {
        try {
            await this.post('/audit/log', {
                action_type: actionType,
                target_type: targetType,
                target_id: targetId,
                metadata,
            })
        } catch {
            // Silently fail
        }
    }

    // Document access management
    async getDocumentAccess(documentId: string) {
        return this.get<{ users: { id: string; full_name: string; email: string }[] }>(`/documents/id/${documentId}/access`)
    }

    async grantAccess(documentId: string, userId: string) {
        return this.post(`/documents/id/${documentId}/access`, { user_id: userId })
    }

    async revokeAccess(documentId: string, userId: string) {
        return this.delete(`/documents/id/${documentId}/access/${userId}`)
    }
}

export const api = new ApiService()
