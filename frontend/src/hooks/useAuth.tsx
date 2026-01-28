import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { api } from '../services/api_client'

interface User {
    id: string
    email: string
    full_name: string
    mobile: string
    role: 'super_admin' | 'admin' | 'user'
    totp_enabled: boolean
}

interface AuthContextType {
    user: User | null
    loading: boolean
    login: (email: string, password: string, totpCode?: string) => Promise<{
        requiresTOTP?: boolean
        requiresPasswordChange?: boolean
        requiresMFAEnrollment?: boolean
    }>
    logout: () => Promise<void>
    refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
    const [user, setUser] = useState<User | null>(null)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        checkAuth()
    }, [])

    const checkAuth = async () => {
        const token = localStorage.getItem('access_token')
        if (!token) {
            setLoading(false)
            return
        }

        try {
            const response = await api.get('/users/me')
            setUser(response.data)
        } catch {
            localStorage.removeItem('access_token')
            localStorage.removeItem('refresh_token')
        } finally {
            setLoading(false)
        }
    }

    const login = async (email: string, password: string, totpCode?: string) => {
        const deviceId = getDeviceId()
        const response = await api.post('/auth/login', {
            email,
            password,
            totp_code: totpCode,
            device_id: deviceId,
        })

        if (response.data.requires_totp) {
            return { requiresTOTP: true }
        }

        localStorage.setItem('access_token', response.data.access_token)
        localStorage.setItem('refresh_token', response.data.refresh_token)
        if (response.data.client_ip) {
            localStorage.setItem('client_ip', response.data.client_ip)
        }
        setUser(response.data.user)

        // Return flags for forced first-login actions
        return {
            requiresPasswordChange: response.data.requires_password_change || false,
            requiresMFAEnrollment: response.data.requires_mfa_enrollment || false,
        }
    }

    const logout = async () => {
        try {
            await api.post('/auth/logout')
        } catch {
            // Ignore errors
        }
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        setUser(null)
    }

    const refreshUser = async () => {
        const response = await api.get('/users/me')
        setUser(response.data)
    }

    return (
        <AuthContext.Provider value={{ user, loading, login, logout, refreshUser }}>
            {children}
        </AuthContext.Provider>
    )
}

export function useAuth() {
    const context = useContext(AuthContext)
    if (!context) {
        throw new Error('useAuth must be used within AuthProvider')
    }
    return context
}

// Generate a stable device ID with fallback for older browsers
function getDeviceId(): string {
    let deviceId = localStorage.getItem('device_id')
    if (!deviceId) {
        deviceId = 'web-' + generateUUID()
        localStorage.setItem('device_id', deviceId)
    }
    return deviceId
}

// UUID generator with fallback for browsers without crypto.randomUUID
function generateUUID(): string {
    // Use crypto.randomUUID if available
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
        return crypto.randomUUID()
    }

    // Fallback: generate UUID v4 manually
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
        const r = Math.random() * 16 | 0
        const v = c === 'x' ? r : (r & 0x3 | 0x8)
        return v.toString(16)
    })
}
