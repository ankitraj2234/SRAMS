import { useState, useRef } from 'react'
import { X, Upload, FileSpreadsheet, AlertCircle, CheckCircle2 } from 'lucide-react'
import { api } from '../services/api_client'
import './ImportUsersModal.css'

interface ParsedUser {
    row: number
    email: string
    full_name: string
    mobile: string
    role: string
    error?: string
}

interface ImportResult {
    total_rows: number
    imported: number
    skipped: number
    failed: number
    errors?: ParsedUser[]
}

interface ImportUsersModalProps {
    onClose: () => void
    onSuccess: () => void
}

export default function ImportUsersModal({ onClose, onSuccess }: ImportUsersModalProps) {
    const [step, setStep] = useState<'upload' | 'preview' | 'configure' | 'result'>('upload')
    const [file, setFile] = useState<File | null>(null)
    const [parsedUsers, setParsedUsers] = useState<ParsedUser[]>([])
    const [validCount, setValidCount] = useState(0)
    const [invalidCount, setInvalidCount] = useState(0)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState('')
    const [result, setResult] = useState<ImportResult | null>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)

    // Configuration options
    const [password, setPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [forcePasswordChange, setForcePasswordChange] = useState(true)
    const [forceMfaEnrollment, setForceMfaEnrollment] = useState(false)

    const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const selectedFile = e.target.files?.[0]
        if (!selectedFile) return

        if (!selectedFile.name.endsWith('.xlsx') && !selectedFile.name.endsWith('.xls')) {
            setError('Please upload an Excel file (.xlsx or .xls)')
            return
        }

        setFile(selectedFile)
        setError('')
        setLoading(true)

        try {
            const formData = new FormData()
            formData.append('file', selectedFile)

            const response = await api.uploadFormData<{ total: number; valid: number; invalid: number; rows: ParsedUser[] }>(
                '/users/bulk/preview',
                formData
            )

            setParsedUsers(response.data.rows || [])
            setValidCount(response.data.valid)
            setInvalidCount(response.data.invalid)
            setStep('preview')
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to parse file')
        } finally {
            setLoading(false)
        }
    }

    const handleImport = async () => {
        if (password !== confirmPassword) {
            setError('Passwords do not match')
            return
        }
        if (password.length < 8) {
            setError('Password must be at least 8 characters')
            return
        }

        setLoading(true)
        setError('')

        try {
            const formData = new FormData()
            formData.append('file', file!)
            formData.append('default_password', password)
            formData.append('force_password_change', forcePasswordChange.toString())
            formData.append('force_mfa_enrollment', forceMfaEnrollment.toString())
            formData.append('skip_duplicates', 'true')

            const response = await api.uploadFormData<ImportResult>(
                '/users/bulk/import',
                formData
            )

            setResult(response.data)
            setStep('result')
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to import users')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal import-modal glass-card" onClick={e => e.stopPropagation()}>
                <div className="modal-header">
                    <h2>
                        <Upload size={20} />
                        Import Users
                    </h2>
                    <button className="close-btn" onClick={onClose}><X size={24} /></button>
                </div>

                <div className="modal-body">
                    {error && <div className="error-message"><AlertCircle size={16} /> {error}</div>}

                    {step === 'upload' && (
                        <div className="upload-step">
                            <div
                                className="upload-zone"
                                onClick={() => fileInputRef.current?.click()}
                            >
                                <FileSpreadsheet size={48} />
                                <h3>Drop Excel file here or click to browse</h3>
                                <p>Supported formats: .xlsx, .xls</p>
                                <input
                                    ref={fileInputRef}
                                    type="file"
                                    accept=".xlsx,.xls"
                                    onChange={handleFileSelect}
                                    style={{ display: 'none' }}
                                />
                            </div>

                            <div className="upload-tips">
                                <h4>Required Columns:</h4>
                                <ul>
                                    <li><strong>Email</strong> - Valid email address (required)</li>
                                    <li><strong>Full Name</strong> - User's display name (required)</li>
                                    <li><strong>Mobile</strong> - Phone number (optional)</li>
                                    <li><strong>Role</strong> - Must be 'user' or 'admin' (required)</li>
                                </ul>
                            </div>
                        </div>
                    )}

                    {step === 'preview' && (
                        <div className="preview-step">
                            <div className="preview-stats">
                                <div className="stat valid">
                                    <CheckCircle2 size={20} />
                                    <span>{validCount} Valid</span>
                                </div>
                                <div className="stat invalid">
                                    <AlertCircle size={20} />
                                    <span>{invalidCount} Invalid</span>
                                </div>
                            </div>

                            <div className="preview-table-container">
                                <table className="preview-table">
                                    <thead>
                                        <tr>
                                            <th>Row</th>
                                            <th>Email</th>
                                            <th>Full Name</th>
                                            <th>Mobile</th>
                                            <th>Role</th>
                                            <th>Status</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {parsedUsers.slice(0, 10).map((user, i) => (
                                            <tr key={i} className={user.error ? 'invalid-row' : ''}>
                                                <td>{user.row}</td>
                                                <td>{user.email}</td>
                                                <td>{user.full_name}</td>
                                                <td>{user.mobile || '-'}</td>
                                                <td>{user.role}</td>
                                                <td>
                                                    {user.error ? (
                                                        <span className="badge badge-error">{user.error}</span>
                                                    ) : (
                                                        <span className="badge badge-success">Valid</span>
                                                    )}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                                {parsedUsers.length > 10 && (
                                    <p className="preview-note">Showing 10 of {parsedUsers.length} rows</p>
                                )}
                            </div>

                            <div className="modal-footer">
                                <button className="btn btn-secondary" onClick={() => setStep('upload')}>
                                    Back
                                </button>
                                <button
                                    className="btn btn-primary"
                                    onClick={() => setStep('configure')}
                                    disabled={validCount === 0}
                                >
                                    Continue ({validCount} users)
                                </button>
                            </div>
                        </div>
                    )}

                    {step === 'configure' && (
                        <div className="configure-step">
                            <h3>Set Password for Imported Users</h3>
                            <p className="configure-description">
                                All {validCount} users will be created with this password.
                            </p>

                            <div className="input-group">
                                <label className="input-label">Common Password</label>
                                <input
                                    type="password"
                                    className="input"
                                    value={password}
                                    onChange={(e) => setPassword(e.target.value)}
                                    placeholder="Enter password for all users"
                                    minLength={8}
                                    required
                                />
                            </div>

                            <div className="input-group">
                                <label className="input-label">Confirm Password</label>
                                <input
                                    type="password"
                                    className="input"
                                    value={confirmPassword}
                                    onChange={(e) => setConfirmPassword(e.target.value)}
                                    placeholder="Confirm password"
                                    required
                                />
                            </div>

                            <div className="options-section">
                                <h4>First Login Options</h4>

                                <label className="checkbox-label">
                                    <input
                                        type="checkbox"
                                        checked={forcePasswordChange}
                                        onChange={(e) => setForcePasswordChange(e.target.checked)}
                                    />
                                    <span>Force password change on first login</span>
                                </label>

                                <label className="checkbox-label">
                                    <input
                                        type="checkbox"
                                        checked={forceMfaEnrollment}
                                        onChange={(e) => setForceMfaEnrollment(e.target.checked)}
                                    />
                                    <span>Require MFA enrollment (Google Authenticator)</span>
                                </label>
                            </div>

                            <div className="modal-footer">
                                <button className="btn btn-secondary" onClick={() => setStep('preview')}>
                                    Back
                                </button>
                                <button
                                    className="btn btn-primary"
                                    onClick={handleImport}
                                    disabled={loading || !password || password !== confirmPassword}
                                >
                                    {loading ? 'Importing...' : `Import ${validCount} Users`}
                                </button>
                            </div>
                        </div>
                    )}

                    {step === 'result' && result && (
                        <div className="result-step">
                            <div className="result-icon success">
                                <CheckCircle2 size={48} />
                            </div>
                            <h3>Import Complete!</h3>

                            <div className="result-stats">
                                <div className="result-stat">
                                    <span className="label">Imported</span>
                                    <span className="value success">{result.imported}</span>
                                </div>
                                <div className="result-stat">
                                    <span className="label">Skipped</span>
                                    <span className="value warning">{result.skipped}</span>
                                </div>
                                <div className="result-stat">
                                    <span className="label">Failed</span>
                                    <span className="value error">{result.failed}</span>
                                </div>
                            </div>

                            {result.errors && result.errors.length > 0 && (
                                <div className="result-errors">
                                    <h4>Errors:</h4>
                                    <ul>
                                        {result.errors.slice(0, 5).map((err, i) => (
                                            <li key={i}>Row {err.row}: {err.error}</li>
                                        ))}
                                    </ul>
                                </div>
                            )}

                            <div className="modal-footer">
                                <button className="btn btn-primary" onClick={onSuccess}>
                                    Done
                                </button>
                            </div>
                        </div>
                    )}
                </div>

                {loading && step === 'upload' && (
                    <div className="loading-overlay">
                        <div className="loading-spinner" />
                        <p>Parsing file...</p>
                    </div>
                )}
            </div>
        </div>
    )
}
