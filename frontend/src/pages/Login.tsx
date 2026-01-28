import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { Shield, Eye, EyeOff, Loader } from 'lucide-react'
import './Login.css'

export default function Login() {
    const [email, setEmail] = useState('')
    const [password, setPassword] = useState('')
    const [totpCode, setTotpCode] = useState('')
    const [showPassword, setShowPassword] = useState(false)
    const [requiresTOTP, setRequiresTOTP] = useState(false)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState('')

    const { login, user } = useAuth()
    const navigate = useNavigate()
    const [searchParams] = useSearchParams()

    // Handle auto-login from desktop app (tokens passed in URL)
    useEffect(() => {
        const autoLogin = searchParams.get('auto')
        const tokenParam = searchParams.get('token')
        const desktopSession = searchParams.get('desktop_session')
        const refreshParam = searchParams.get('refresh')

        // If desktop app is passing tokens for auto-login
        if (autoLogin === '1' && tokenParam && desktopSession) {
            // Store tokens in localStorage
            localStorage.setItem('access_token', tokenParam)
            localStorage.setItem('desktop_session', desktopSession)
            if (refreshParam) {
                localStorage.setItem('refresh_token', refreshParam)
            }

            // Remove tokens from URL for security
            window.history.replaceState({}, document.title, '/login')

            // Use full page redirect to trigger AuthProvider to reload with new token
            // This ensures checkAuth() runs with the new stored token
            window.location.href = '/dashboard'
            return
        }

        // Just desktop_session without token (store for later)
        if (desktopSession && !tokenParam) {
            localStorage.setItem('desktop_session', desktopSession)
            window.history.replaceState({}, document.title, '/login')
        }

        // If already logged in, redirect to dashboard
        if (user) {
            navigate('/dashboard')
        }
    }, [searchParams, user, navigate])

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')
        setLoading(true)

        try {
            const result = await login(email, password, requiresTOTP ? totpCode : undefined)

            if (result.requiresTOTP) {
                setRequiresTOTP(true)
                setLoading(false)
                return
            }

            // Handle first-login enforcement redirects
            if (result.requiresPasswordChange) {
                navigate('/force-change-password')
                return
            }

            if (result.requiresMFAEnrollment) {
                navigate('/force-enroll-mfa')
                return
            }

            navigate('/dashboard')
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Login failed')
            setLoading(false)
        }
    }

    return (
        <div className="login-page">
            <div className="login-bg">
                <div className="bg-gradient" />
                <div className="bg-pattern" />
            </div>

            <div className="login-container animate-slideUp">
                <div className="login-card glass-card">
                    <div className="login-header">
                        <div className="logo-icon">
                            <Shield size={40} />
                        </div>
                        <h1>SRAMS</h1>
                        <p>Secure Role-Based Audit Management System</p>
                    </div>

                    <form onSubmit={handleSubmit} className="login-form">
                        <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
                            Sign in to SRAMS Enterprise
                        </h2>
                        {error && (
                            <div className="error-message">
                                {error}
                            </div>
                        )}

                        {!requiresTOTP ? (
                            <>
                                <div className="input-group">
                                    <label className="input-label">Email</label>
                                    <input
                                        type="email"
                                        className="input"
                                        value={email}
                                        onChange={(e) => setEmail(e.target.value)}
                                        placeholder="admin@example.com"
                                        required
                                        autoComplete="email"
                                    />
                                </div>

                                <div className="input-group">
                                    <label className="input-label">Password</label>
                                    <div className="password-input">
                                        <input
                                            type={showPassword ? 'text' : 'password'}
                                            className="input"
                                            value={password}
                                            onChange={(e) => setPassword(e.target.value)}
                                            placeholder="••••••••"
                                            required
                                            autoComplete="current-password"
                                        />
                                        <button
                                            type="button"
                                            className="toggle-password"
                                            onClick={() => setShowPassword(!showPassword)}
                                        >
                                            {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                                        </button>
                                    </div>
                                </div>
                            </>
                        ) : (
                            <div className="input-group">
                                <label className="input-label">Two-Factor Authentication Code</label>
                                <input
                                    type="text"
                                    className="input totp-input"
                                    value={totpCode}
                                    onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                                    placeholder="000000"
                                    required
                                    autoComplete="one-time-code"
                                    maxLength={6}
                                />
                                <p className="totp-hint">Enter the 6-digit code from your authenticator app</p>
                            </div>
                        )}

                        <button type="submit" className="btn btn-primary login-btn" disabled={loading}>
                            {loading ? (
                                <>
                                    <Loader size={18} className="animate-spin" />
                                    <span>Signing in...</span>
                                </>
                            ) : (
                                <span>{requiresTOTP ? 'Verify' : 'Sign In'}</span>
                            )}
                        </button>

                        {requiresTOTP && (
                            <button
                                type="button"
                                className="btn btn-secondary"
                                onClick={() => {
                                    setRequiresTOTP(false)
                                    setTotpCode('')
                                }}
                            >
                                Back to Login
                            </button>
                        )}
                    </form>

                    <div className="login-footer">
                        <p>Secure • Audited • Compliant</p>
                    </div>
                </div>
            </div>
        </div>
    )
}
