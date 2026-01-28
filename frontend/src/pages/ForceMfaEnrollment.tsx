import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../services/api_client'
import { useAuth } from '../hooks/useAuth'
import { Shield, AlertCircle, CheckCircle2, Smartphone, Copy, Check, ArrowRight, Lock } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import './ForceMfaEnrollment.css'

interface TOTPSetupData {
    secret: string
    qr_url: string
    backup_codes: string[]
}

export default function ForceMfaEnrollment() {
    const navigate = useNavigate()
    const { refreshUser } = useAuth()
    const [step, setStep] = useState<'intro' | 'setup' | 'verify' | 'backup'>('intro')
    const [setupData, setSetupData] = useState<TOTPSetupData | null>(null)
    const [verificationCode, setVerificationCode] = useState('')
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)
    const [copied, setCopied] = useState(false)

    useEffect(() => {
        if (step === 'setup') {
            initializeTOTP()
        }
    }, [step])

    const initializeTOTP = async () => {
        setLoading(true)
        setError('')
        try {
            const response = await api.get<TOTPSetupData>('/auth/totp/setup')
            setSetupData(response.data)
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to initialize MFA')
        } finally {
            setLoading(false)
        }
    }

    const handleVerify = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')
        setLoading(true)

        if (!setupData) return

        try {
            const response = await api.post<{ backup_codes: string[] }>('/auth/totp/enable', {
                secret: setupData.secret,
                code: verificationCode
            })

            setSetupData(prev => prev ? ({ ...prev, backup_codes: response.data.backup_codes }) : null)
            setStep('backup')
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Invalid verification code')
        } finally {
            setLoading(false)
        }
    }

    const handleComplete = async () => {
        await refreshUser()
        navigate('/dashboard')
    }

    const copyBackupCodes = async () => {
        if (!setupData?.backup_codes) return

        const textToCopy = setupData.backup_codes.join('\n')

        try {
            // Priority 1: Modern Async Clipboard API (Secure Contexts: HTTPS/Localhost)
            if (navigator.clipboard && navigator.clipboard.writeText) {
                await navigator.clipboard.writeText(textToCopy)
                setCopied(true)
            } else {
                throw new Error('Clipboard API unavailable')
            }
        } catch (err) {
            // Priority 2: Fallback for HTTP/LAN (Legacy execCommand)
            try {
                const textArea = document.createElement("textarea")
                textArea.value = textToCopy

                // Ensure it's not visible but part of DOM
                textArea.style.position = "fixed"
                textArea.style.left = "-9999px"
                textArea.style.top = "0"
                document.body.appendChild(textArea)

                textArea.focus()
                textArea.select()

                const successful = document.execCommand('copy')
                document.body.removeChild(textArea)

                if (successful) {
                    setCopied(true)
                } else {
                    console.error('Fallback copy failed')
                    // alert('Press Ctrl+C to copy codes') // Optional: minimal UI interruption
                }
            } catch (fallbackErr) {
                console.error('Copy failed', fallbackErr)
            }
        }

        setTimeout(() => setCopied(false), 2000)
    }

    return (
        <div className="mfa-page">
            <div className="mfa-card">
                {step === 'intro' && (
                    <div className="mfa-content">
                        <div className="mfa-header">
                            <div className="mfa-icon-wrapper">
                                <Shield size={40} strokeWidth={1.5} />
                            </div>
                            <h1 className="mfa-title">Secure Your Account</h1>
                            <p className="mfa-subtitle">
                                Protecting your data is our priority. Set up Two-Factor Authentication (2FA) to continue.
                            </p>
                        </div>

                        <ul className="mfa-list">
                            <li className="mfa-list-item">
                                <div className="step-number">1</div>
                                <span className="mfa-list-text">Download <strong>Google Authenticator</strong> or a similar app on your mobile device.</span>
                            </li>
                            <li className="mfa-list-item">
                                <div className="step-number">2</div>
                                <span className="mfa-list-text">Scan the specific QR code we'll provide in the next step.</span>
                            </li>
                            <li className="mfa-list-item">
                                <div className="step-number">3</div>
                                <span className="mfa-list-text">Enter the 6-digit verification code generated by the app.</span>
                            </li>
                        </ul>

                        <button
                            className="btn-premium btn-primary-gradient"
                            onClick={() => setStep('setup')}
                        >
                            Get Started <ArrowRight size={20} />
                        </button>
                    </div>
                )}

                {step === 'setup' && (
                    <div className="mfa-content" style={{ textAlign: 'center' }}>
                        <div className="mfa-header">
                            <h2 className="mfa-title">Scan QR Code</h2>
                            <p className="mfa-subtitle">Open your authenticator app and scan the code below.</p>
                        </div>

                        {loading || !setupData ? (
                            <div className="loading-dots">
                                <div className="dot"></div>
                                <div className="dot"></div>
                                <div className="dot"></div>
                            </div>
                        ) : (
                            <>
                                <div className="qr-section">
                                    <QRCodeSVG
                                        value={setupData.qr_url}
                                        size={200}
                                        level="L"
                                        includeMargin={true}
                                    />
                                </div>

                                <p className="mfa-subtitle" style={{ marginBottom: '1rem' }}>
                                    Or enter this code manually:
                                </p>
                                <div className="secret-container">
                                    {setupData.secret}
                                </div>

                                <button
                                    className="btn-premium btn-primary-gradient"
                                    onClick={() => setStep('verify')}
                                >
                                    I've Scanned It <ArrowRight size={20} />
                                </button>
                            </>
                        )}

                        {error && (
                            <div className="error-banner" style={{ marginTop: '1rem' }}>
                                <AlertCircle size={16} /> {error}
                            </div>
                        )}
                    </div>
                )}

                {step === 'verify' && (
                    <form onSubmit={handleVerify} className="mfa-content">
                        <div className="mfa-header">
                            <div className="mfa-icon-wrapper">
                                <Lock size={40} strokeWidth={1.5} />
                            </div>
                            <h2 className="mfa-title">Verify Setup</h2>
                            <p className="mfa-subtitle">Enter the 6-digit code from your authenticator app to enable 2FA.</p>
                        </div>

                        <div className="code-input-group">
                            <input
                                type="text"
                                className="totp-input-premium"
                                placeholder="000 000"
                                value={verificationCode}
                                onChange={(e) => setVerificationCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                                maxLength={6}
                                pattern="[0-9]{6}"
                                required
                                autoFocus
                            />
                        </div>

                        {error && (
                            <div className="error-banner">
                                <AlertCircle size={16} /> {error}
                            </div>
                        )}

                        <button
                            type="submit"
                            className="btn-premium btn-primary-gradient"
                            disabled={loading || verificationCode.length !== 6}
                        >
                            {loading ? 'Verifying...' : 'Verify & Enable'}
                        </button>

                        <button
                            type="button"
                            className="btn-premium btn-secondary-ghost"
                            onClick={() => setStep('setup')}
                        >
                            Back to QR Code
                        </button>
                    </form>
                )}

                {step === 'backup' && (
                    <div className="mfa-content">
                        <div className="mfa-header">
                            <div className="mfa-icon-wrapper" style={{ color: '#10b981', background: 'rgba(16, 185, 129, 0.1)', boxShadow: '0 0 0 8px rgba(16, 185, 129, 0.05)' }}>
                                <CheckCircle2 size={40} strokeWidth={1.5} />
                            </div>
                            <h2 className="mfa-title">Setup Complete!</h2>
                            <p className="mfa-subtitle">
                                Save these backup codes in a secure place. You can use them to login if you lose access to your device.
                            </p>
                        </div>

                        <div className="backup-codes" style={{
                            background: 'rgba(0,0,0,0.3)',
                            padding: '1.5rem',
                            borderRadius: '12px',
                            display: 'grid',
                            gridTemplateColumns: '1fr 1fr',
                            gap: '1rem',
                            marginBottom: '2rem',
                            fontFamily: 'monospace'
                        }}>
                            {setupData?.backup_codes.map((code, i) => (
                                <div key={i} style={{ color: '#e2e8f0', textAlign: 'center', letterSpacing: '0.05em' }}>
                                    {code}
                                </div>
                            ))}
                        </div>

                        <button
                            className="btn-premium btn-secondary-ghost"
                            onClick={copyBackupCodes}
                            style={{ marginBottom: '1rem', marginTop: 0 }}
                        >
                            {copied ? <><Check size={18} /> Copied to Clipboard</> : <><Copy size={18} /> Copy All Codes</>}
                        </button>

                        <button
                            className="btn-premium btn-primary-gradient"
                            onClick={handleComplete}
                        >
                            Continue to Dashboard
                        </button>
                    </div>
                )}
            </div>
        </div>
    )
}
