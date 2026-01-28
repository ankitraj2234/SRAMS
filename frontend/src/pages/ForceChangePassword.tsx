import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../services/api_client'
import { useAuth } from '../hooks/useAuth'
import { Lock, Eye, EyeOff, AlertCircle, CheckCircle } from 'lucide-react'
import '../pages/Login.css'

export default function ForceChangePassword() {
    const navigate = useNavigate()
    const { refreshUser } = useAuth()
    const [currentPassword, setCurrentPassword] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [showCurrent, setShowCurrent] = useState(false)
    const [showNew, setShowNew] = useState(false)
    const [showConfirm, setShowConfirm] = useState(false)
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        if (newPassword !== confirmPassword) {
            setError('New passwords do not match')
            return
        }

        if (newPassword.length < 8) {
            setError('Password must be at least 8 characters')
            return
        }

        if (newPassword === currentPassword) {
            setError('New password must be different from current password')
            return
        }

        setLoading(true)
        try {
            await api.post('/auth/change-password', {
                current_password: currentPassword,
                new_password: newPassword
            })

            // Refresh user data to clear the must_change_password flag
            await refreshUser()

            // Check if user needs to enroll in MFA
            // We need to fetch the fresh profile again or rely on refreshUser updating the context
            // Since refreshUser updates the context asynchronously, we might need to manually check the condition
            // or trust that the next route transition will handle it. 
            // However, a direct navigation is safer.

            // To be safe, we can check the response or just navigate to dashboard 
            // and let the Dashboard/Auth protection redirect if needed.
            // But the user specifically mentioned skipping the enrollment.

            // Let's force a check using the API we know exists
            const profileRes = await api.get<{ must_enroll_mfa: boolean }>('/auth/profile')
            if (profileRes.data.must_enroll_mfa) {
                navigate('/force-enroll-mfa')
            } else {
                navigate('/dashboard')
            }
        } catch (err: unknown) {
            const error = err as { response?: { data?: { error?: string } } }
            setError(error?.response?.data?.error || 'Failed to change password')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="login-page">
            <div className="login-container">
                <div className="login-card glass-card">
                    <div className="login-header">
                        <div className="login-icon">
                            <Lock size={32} />
                        </div>
                        <h1>Password Change Required</h1>
                        <p>For security reasons, you must change your password before continuing.</p>
                    </div>

                    <form onSubmit={handleSubmit} className="login-form">
                        {error && (
                            <div className="error-message">
                                <AlertCircle size={16} />
                                <span>{error}</span>
                            </div>
                        )}

                        <div className="input-group">
                            <label className="input-label">Current Password</label>
                            <div className="input-wrapper">
                                <input
                                    type={showCurrent ? 'text' : 'password'}
                                    className="input"
                                    placeholder="Enter your current password"
                                    value={currentPassword}
                                    onChange={(e) => setCurrentPassword(e.target.value)}
                                    required
                                    autoComplete="current-password"
                                />
                                <button
                                    type="button"
                                    className="input-icon-btn"
                                    onClick={() => setShowCurrent(!showCurrent)}
                                    tabIndex={-1}
                                >
                                    {showCurrent ? <EyeOff size={18} /> : <Eye size={18} />}
                                </button>
                            </div>
                        </div>

                        <div className="input-group">
                            <label className="input-label">New Password</label>
                            <div className="input-wrapper">
                                <input
                                    type={showNew ? 'text' : 'password'}
                                    className="input"
                                    placeholder="Enter your new password"
                                    value={newPassword}
                                    onChange={(e) => setNewPassword(e.target.value)}
                                    required
                                    minLength={8}
                                    autoComplete="new-password"
                                />
                                <button
                                    type="button"
                                    className="input-icon-btn"
                                    onClick={() => setShowNew(!showNew)}
                                    tabIndex={-1}
                                >
                                    {showNew ? <EyeOff size={18} /> : <Eye size={18} />}
                                </button>
                            </div>
                        </div>

                        <div className="input-group">
                            <label className="input-label">Confirm New Password</label>
                            <div className="input-wrapper">
                                <input
                                    type={showConfirm ? 'text' : 'password'}
                                    className="input"
                                    placeholder="Confirm your new password"
                                    value={confirmPassword}
                                    onChange={(e) => setConfirmPassword(e.target.value)}
                                    required
                                    minLength={8}
                                    autoComplete="new-password"
                                />
                                <button
                                    type="button"
                                    className="input-icon-btn"
                                    onClick={() => setShowConfirm(!showConfirm)}
                                    tabIndex={-1}
                                >
                                    {showConfirm ? <EyeOff size={18} /> : <Eye size={18} />}
                                </button>
                            </div>
                        </div>

                        {newPassword.length >= 8 && newPassword === confirmPassword && (
                            <div className="success-hint">
                                <CheckCircle size={16} />
                                <span>Passwords match!</span>
                            </div>
                        )}

                        <button
                            type="submit"
                            className="btn btn-primary btn-full"
                            disabled={loading || !currentPassword || !newPassword || !confirmPassword}
                        >
                            {loading ? 'Changing Password...' : 'Change Password & Continue'}
                        </button>
                    </form>
                </div>
            </div>
        </div>
    )
}
