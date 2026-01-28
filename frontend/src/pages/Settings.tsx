import { useState, useEffect } from 'react'
import { useAuth } from '../hooks/useAuth'
import { useTheme } from '../hooks/useTheme'
import { api } from '../services/api_client'
import { User, Lock, Moon, Sun, Shield, Smartphone, LogOut, Image, Upload, Trash2 } from 'lucide-react'
import './Settings.css'

export default function Settings() {
    const { user, refreshUser } = useAuth()
    const { theme, toggleTheme } = useTheme()
    const isSuperAdmin = user?.role === 'super_admin'

    return (
        <div className="settings-page animate-fadeIn">
            <header className="page-header">
                <h1>Settings</h1>
                <p>Manage your account and preferences</p>
            </header>

            <div className="settings-grid">
                {/* Profile Section */}
                <ProfileSection user={user} onUpdate={refreshUser} />

                {/* Security Section */}
                <SecuritySection user={user} onUpdate={refreshUser} />

                {/* Preferences Section */}
                <section className="settings-section glass-card">
                    <div className="section-header">
                        <Moon size={24} />
                        <h2>Preferences</h2>
                    </div>

                    <div className="preference-item">
                        <div className="preference-info">
                            <h3>Theme</h3>
                            <p>Choose your preferred color scheme</p>
                        </div>
                        <button className="theme-toggle" onClick={toggleTheme}>
                            {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
                            <span>{theme === 'dark' ? 'Light Mode' : 'Dark Mode'}</span>
                        </button>
                    </div>
                </section>

                {/* Company Logo Section - Super Admin only */}
                {isSuperAdmin && <LogoSection />}

                {/* Sessions Section */}
                <SessionsSection />
            </div>
        </div>
    )
}

function ProfileSection({ user, onUpdate }: { user: any; onUpdate: () => void }) {
    const [editing, setEditing] = useState(false)
    const [fullName, setFullName] = useState(user?.full_name || '')
    const [mobile, setMobile] = useState(user?.mobile || '')
    const [saving, setSaving] = useState(false)

    const handleSave = async () => {
        setSaving(true)
        try {
            await api.put(`/users/${user.id}`, { full_name: fullName, mobile })
            await onUpdate()
            setEditing(false)
        } catch (err) {
            alert('Failed to update profile')
        } finally {
            setSaving(false)
        }
    }

    return (
        <section className="settings-section glass-card">
            <div className="section-header">
                <User size={24} />
                <h2>Profile</h2>
            </div>

            <div className="profile-content">
                <div className="profile-avatar">
                    {(user?.full_name || '').charAt(0).toUpperCase()}
                </div>

                {editing ? (
                    <div className="profile-form">
                        <div className="input-group">
                            <label className="input-label">Full Name</label>
                            <input
                                type="text"
                                className="input"
                                value={fullName}
                                onChange={(e) => setFullName(e.target.value)}
                            />
                        </div>
                        <div className="input-group">
                            <label className="input-label">Mobile</label>
                            <input
                                type="tel"
                                className="input"
                                value={mobile}
                                onChange={(e) => setMobile(e.target.value)}
                            />
                        </div>
                        <div className="button-group">
                            <button className="btn btn-secondary" onClick={() => setEditing(false)}>Cancel</button>
                            <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
                                {saving ? 'Saving...' : 'Save'}
                            </button>
                        </div>
                    </div>
                ) : (
                    <div className="profile-info">
                        <div className="info-row">
                            <span className="label">Name</span>
                            <span className="value">{user?.full_name}</span>
                        </div>
                        <div className="info-row">
                            <span className="label">Email</span>
                            <span className="value">{user?.email}</span>
                        </div>
                        <div className="info-row">
                            <span className="label">Mobile</span>
                            <span className="value">{user?.mobile || 'Not set'}</span>
                        </div>
                        <div className="info-row">
                            <span className="label">Role</span>
                            <span className="value badge badge-info">{user?.role}</span>
                        </div>
                        <button className="btn btn-secondary" onClick={() => setEditing(true)}>Edit Profile</button>
                    </div>
                )}
            </div>
        </section>
    )
}

import { QRCodeSVG } from 'qrcode.react'

function SecuritySection({ user, onUpdate }: { user: any; onUpdate: () => void }) {
    const [showPasswordForm, setShowPasswordForm] = useState(false)
    const [currentPassword, setCurrentPassword] = useState('')
    const [newPassword, setNewPassword] = useState('')
    const [saving, setSaving] = useState(false)
    const [showTOTPSetup, setShowTOTPSetup] = useState(false)
    const [totpSecret, setTotpSecret] = useState('')
    const [totpUrl, setTotpUrl] = useState('')
    const [totpCode, setTotpCode] = useState('')

    const handlePasswordChange = async () => {
        setSaving(true)
        try {
            await api.post('/auth/change-password', {
                current_password: currentPassword,
                new_password: newPassword,
            })
            alert('Password changed successfully')
            setShowPasswordForm(false)
            setCurrentPassword('')
            setNewPassword('')
        } catch (err) {
            alert(err instanceof Error ? err.message : 'Failed to change password')
        } finally {
            setSaving(false)
        }
    }

    const setupTOTP = async () => {
        try {
            const response = await api.get<{ secret: string; qr_url: string }>('/auth/totp/setup')
            setTotpSecret(response.data.secret)
            setTotpUrl(response.data.qr_url)
            setShowTOTPSetup(true)
        } catch (err) {
            alert('Failed to setup 2FA')
        }
    }

    const enableTOTP = async () => {
        try {
            await api.post('/auth/totp/enable', { secret: totpSecret, code: totpCode })
            await onUpdate()
            setShowTOTPSetup(false)
            setTotpCode('')
            setTotpUrl('')
            alert('2FA enabled successfully')
        } catch (err) {
            alert('Invalid code')
        }
    }

    const disableTOTP = async () => {
        if (!confirm('Are you sure you want to disable 2FA?')) return
        try {
            await api.post('/auth/totp/disable', {})
            await onUpdate()
        } catch (err) {
            alert('Failed to disable 2FA')
        }
    }

    return (
        <section className="settings-section glass-card">
            <div className="section-header">
                <Shield size={24} />
                <h2>Security</h2>
            </div>

            {/* Password */}
            <div className="security-item">
                <div className="security-info">
                    <Lock size={20} />
                    <div>
                        <h3>Password</h3>
                        <p>Change your account password</p>
                    </div>
                </div>
                {showPasswordForm ? (
                    <div className="password-form">
                        <div className="input-group">
                            <input
                                type="password"
                                className="input"
                                placeholder="Current password"
                                value={currentPassword}
                                onChange={(e) => setCurrentPassword(e.target.value)}
                            />
                        </div>
                        <div className="input-group">
                            <input
                                type="password"
                                className="input"
                                placeholder="New password (min 8 characters)"
                                value={newPassword}
                                onChange={(e) => setNewPassword(e.target.value)}
                                minLength={8}
                            />
                        </div>
                        <div className="button-group">
                            <button className="btn btn-secondary" onClick={() => setShowPasswordForm(false)}>Cancel</button>
                            <button className="btn btn-primary" onClick={handlePasswordChange} disabled={saving || newPassword.length < 8}>
                                {saving ? 'Changing...' : 'Change Password'}
                            </button>
                        </div>
                    </div>
                ) : (
                    <button className="btn btn-secondary" onClick={() => setShowPasswordForm(true)}>Change</button>
                )}
            </div>

            {/* 2FA */}
            <div className="security-item">
                <div className="security-info">
                    <Smartphone size={20} />
                    <div>
                        <h3>Two-Factor Authentication</h3>
                        <p>{user?.totp_enabled ? 'Enabled' : 'Add extra security to your account'}</p>
                    </div>
                </div>
                {showTOTPSetup ? (
                    <div className="totp-form">
                        <div className="totp-qr">
                            {totpUrl && (
                                <div className="qr-container bg-white p-4 rounded-lg mb-4 inline-block">
                                    <QRCodeSVG
                                        value={totpUrl}
                                        size={192}
                                        level="L"
                                        includeMargin={true}
                                    />
                                </div>
                            )}
                        </div>
                        <p className="totp-instructions">
                            Scan the QR code with your authenticator app, or enter this secret manually:
                        </p>
                        <code className="totp-secret">{totpSecret}</code>
                        <div className="input-group mt-4">
                            <input
                                type="text"
                                className="input"
                                placeholder="Enter 6-digit code"
                                value={totpCode}
                                onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                                maxLength={6}
                            />
                        </div>
                        <div className="button-group">
                            <button className="btn btn-secondary" onClick={() => setShowTOTPSetup(false)}>Cancel</button>
                            <button className="btn btn-primary" onClick={enableTOTP} disabled={totpCode.length !== 6}>
                                Enable 2FA
                            </button>
                        </div>
                    </div>
                ) : user?.totp_enabled ? (
                    <button className="btn btn-danger" onClick={disableTOTP}>Disable</button>
                ) : (
                    <button className="btn btn-secondary" onClick={setupTOTP}>Setup</button>
                )}
            </div>
        </section>
    )
}

function SessionsSection() {
    const [sessions, setSessions] = useState<any[]>([])
    const [loading, setLoading] = useState(false)

    const loadSessions = async () => {
        setLoading(true)
        try {
            const response = await api.get<{ sessions: any[] }>('/auth/sessions')
            setSessions(response.data.sessions || [])
        } catch (err) {
            console.error('Failed to load sessions:', err)
        } finally {
            setLoading(false)
        }
    }

    return (
        <section className="settings-section glass-card">
            <div className="section-header">
                <LogOut size={24} />
                <h2>Active Sessions</h2>
                <button className="btn btn-secondary" onClick={loadSessions} disabled={loading}>
                    {loading ? 'Loading...' : 'View Sessions'}
                </button>
            </div>

            {sessions.length > 0 && (
                <div className="sessions-list">
                    {sessions.map((session, i) => (
                        <div key={session.id || i} className="session-item">
                            <div className="session-info">
                                <span className="session-ip">{session.ip_address}</span>
                                <span className="session-agent">{session.user_agent?.slice(0, 50)}...</span>
                                <span className="session-time">
                                    Last active: {new Date(session.last_activity).toLocaleString()}
                                </span>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </section>
    )
}

function LogoSection() {
    const [logoUrl, setLogoUrl] = useState<string | null>(null)
    const [loading, setLoading] = useState(true)
    const [uploading, setUploading] = useState(false)
    const [error, setError] = useState('')
    const [success, setSuccess] = useState('')
    const [opacity, setOpacity] = useState(20) // Default 20%
    const [savedOpacity, setSavedOpacity] = useState(20)
    const [savingOpacity, setSavingOpacity] = useState(false)

    useEffect(() => {
        loadLogo()
        loadOpacitySetting()
    }, [])

    const loadOpacitySetting = async () => {
        try {
            const token = localStorage.getItem('access_token')
            const desktopSession = localStorage.getItem('desktop_session')

            const headers: Record<string, string> = {
                Authorization: `Bearer ${token}`
            }
            if (desktopSession) {
                headers['X-Desktop-Session'] = desktopSession
            }

            const response = await fetch('/api/system/config', { headers })
            if (response.ok) {
                const data = await response.json()
                const opacityValue = parseInt(data.config?.watermark_opacity || '20', 10)
                setOpacity(opacityValue)
                setSavedOpacity(opacityValue)
            }
        } catch (err) {
            console.log('Failed to load opacity setting:', err)
        }
    }

    const handleApplyOpacity = async () => {
        setSavingOpacity(true)
        setError('')
        setSuccess('')

        try {
            const token = localStorage.getItem('access_token')
            const desktopSession = localStorage.getItem('desktop_session')

            const headers: Record<string, string> = {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${token || ''}`
            }
            if (desktopSession) {
                headers['X-Desktop-Session'] = desktopSession
            }

            const response = await fetch('/api/system/config', {
                method: 'PUT',
                headers,
                body: JSON.stringify({ key: 'watermark_opacity', value: String(opacity) })
            })

            if (response.ok) {
                setSavedOpacity(opacity)
                setSuccess(`Watermark opacity set to ${opacity}%`)
            } else {
                const data = await response.json()
                setError(data.error || 'Failed to save opacity')
            }
        } catch (err) {
            setError('Failed to save opacity setting')
        } finally {
            setSavingOpacity(false)
        }
    }

    const loadLogo = async () => {
        try {
            const token = localStorage.getItem('access_token')
            const desktopSession = localStorage.getItem('desktop_session')

            const headers: Record<string, string> = {
                Authorization: `Bearer ${token}`
            }
            if (desktopSession) {
                headers['X-Desktop-Session'] = desktopSession
            }

            const response = await fetch('/api/system/logo', { headers })
            if (response.ok) {
                const blob = await response.blob()
                setLogoUrl(URL.createObjectURL(blob))
                console.log('Logo loaded successfully')
            } else {
                setLogoUrl(null)
                console.log('No logo found:', response.status)
            }
        } catch (err) {
            setLogoUrl(null)
            console.log('Failed to load logo:', err)
        } finally {
            setLoading(false)
        }
    }

    const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return

        // Validate file type
        if (!['image/png', 'image/jpeg', 'image/gif'].includes(file.type)) {
            setError('Only PNG, JPG, or GIF files allowed')
            return
        }

        // Validate file size (max 500KB)
        if (file.size > 500 * 1024) {
            setError('Logo file too large (max 500KB)')
            return
        }

        setError('')
        setSuccess('')
        setUploading(true)

        try {
            const token = localStorage.getItem('access_token')
            const desktopSession = localStorage.getItem('desktop_session')
            const formData = new FormData()
            formData.append('logo', file)

            const headers: Record<string, string> = {
                Authorization: `Bearer ${token || ''}`
            }
            if (desktopSession) {
                headers['X-Desktop-Session'] = desktopSession
            }

            const response = await fetch('/api/system/logo', {
                method: 'POST',
                headers,
                body: formData
            })

            if (response.ok) {
                setSuccess('Logo uploaded successfully')
                loadLogo()
            } else {
                const data = await response.json()
                setError(data.error || 'Failed to upload logo')
            }
        } catch (err) {
            setError('Failed to upload logo')
        } finally {
            setUploading(false)
        }
    }

    const handleDelete = async () => {
        if (!confirm('Are you sure you want to delete the company logo?')) return

        setError('')
        setSuccess('')

        try {
            const token = localStorage.getItem('access_token')
            const desktopSession = localStorage.getItem('desktop_session')

            const headers: Record<string, string> = {
                Authorization: `Bearer ${token || ''}`
            }
            if (desktopSession) {
                headers['X-Desktop-Session'] = desktopSession
            }

            const response = await fetch('/api/system/logo', {
                method: 'DELETE',
                headers
            })

            if (response.ok) {
                setSuccess('Logo deleted')
                setLogoUrl(null)
            } else {
                const data = await response.json()
                setError(data.error || 'Failed to delete logo')
            }
        } catch (err) {
            setError('Failed to delete logo')
        }
    }

    return (
        <section className="settings-section glass-card">
            <div className="section-header">
                <Image size={24} />
                <h2>Company Logo</h2>
            </div>

            <p className="section-description">
                Upload a company logo to display in document watermarks. The logo will be visible on all documents viewed by users.
            </p>

            {error && <div className="error-message">{error}</div>}
            {success && <div className="success-message">{success}</div>}

            <div className="logo-preview-container">
                {loading ? (
                    <div className="loading-spinner" />
                ) : logoUrl ? (
                    <img src={logoUrl} alt="Company Logo" className="logo-preview" />
                ) : (
                    <div className="logo-placeholder">
                        <Image size={48} />
                        <span>No logo uploaded</span>
                    </div>
                )}
            </div>

            <div className="logo-actions">
                <label className="btn btn-primary upload-btn">
                    <Upload size={16} />
                    {uploading ? 'Uploading...' : 'Upload Logo'}
                    <input
                        type="file"
                        accept="image/png,image/jpeg,image/gif"
                        onChange={handleUpload}
                        disabled={uploading}
                        style={{ display: 'none' }}
                    />
                </label>

                {logoUrl && (
                    <button className="btn btn-danger" onClick={handleDelete}>
                        <Trash2 size={16} />
                        Delete Logo
                    </button>
                )}
            </div>

            <p className="help-text">
                Supported formats: PNG, JPG, GIF • Max size: 500KB
            </p>

            {/* Opacity Control */}
            <div className="opacity-control">
                <h3>Watermark Opacity</h3>
                <p className="section-description">
                    Adjust how visible the watermark appears on documents (1% = very faint, 100% = solid)
                </p>
                <div className="opacity-slider-container">
                    <input
                        type="range"
                        min="1"
                        max="100"
                        value={opacity}
                        onChange={(e) => setOpacity(parseInt(e.target.value, 10))}
                        className="opacity-slider"
                    />
                    <span className="opacity-value">{opacity}%</span>
                </div>
                <div className="opacity-actions">
                    <button
                        className="btn btn-primary"
                        onClick={handleApplyOpacity}
                        disabled={savingOpacity || opacity === savedOpacity}
                    >
                        {savingOpacity ? 'Applying...' : 'Apply Opacity'}
                    </button>
                    {opacity !== savedOpacity && (
                        <span className="unsaved-indicator">Unsaved changes</span>
                    )}
                </div>
            </div>
        </section>
    )
}
