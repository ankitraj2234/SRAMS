import { useEffect, useState } from 'react'
import { useAuth } from '../hooks/useAuth'
import { api } from '../services/api_client'
import { FileText, Upload, Search, Plus, X, Users, Check } from 'lucide-react'
import './Documents.css'

interface Document {
    id: string
    title: string
    filename: string
    file_size: number
    created_at: string
    can_reassign?: boolean
}

interface User {
    id: string
    full_name: string
    email: string
    role?: string
}

export default function Documents() {
    const { user } = useAuth()
    const [documents, setDocuments] = useState<Document[]>([])
    const [allDocuments, setAllDocuments] = useState<Document[]>([])
    const [loading, setLoading] = useState(true)
    const [search, setSearch] = useState('')
    const [showUpload, setShowUpload] = useState(false)
    const [showRequest, setShowRequest] = useState(false)
    const [showAssign, setShowAssign] = useState(false)
    const [selectedDoc, setSelectedDoc] = useState<Document | null>(null)

    const isSuperAdmin = user?.role === 'super_admin'

    useEffect(() => {
        loadDocuments()
    }, [])


    const loadDocuments = async () => {
        try {
            const myDocsRes = await api.get<{ documents: Document[] }>('/documents/my')
            setDocuments(myDocsRes.data.documents || [])

            if (isSuperAdmin) {
                const allDocsRes = await api.get<{ documents: Document[] }>('/documents')
                setAllDocuments(allDocsRes.data.documents || [])
            }
        } catch (err) {
            console.error('Failed to load documents:', err)
        } finally {
            setLoading(false)
        }
    }

    const formatFileSize = (bytes: number) => {
        if (bytes < 1024) return bytes + ' B'
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
        return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
    }

    const filteredDocs = documents.filter(doc =>
        doc.title.toLowerCase().includes(search.toLowerCase()) ||
        doc.filename.toLowerCase().includes(search.toLowerCase())
    )

    const filteredAllDocs = allDocuments.filter(doc =>
        doc.title.toLowerCase().includes(search.toLowerCase()) ||
        doc.filename.toLowerCase().includes(search.toLowerCase())
    )

    const handleAssignClick = (e: React.MouseEvent, doc: Document) => {
        e.preventDefault()
        e.stopPropagation()
        setSelectedDoc(doc)
        setShowAssign(true)
    }

    if (loading) {
        return (
            <div className="loading-container">
                <div className="loading-spinner" />
            </div>
        )
    }

    return (
        <div className="documents-page animate-fadeIn">
            <header className="page-header">
                <div>
                    <h1>Documents</h1>
                    <p>View and manage your documents</p>
                </div>

                <div className="header-actions">
                    <button className="btn btn-primary" onClick={() => setShowUpload(true)}>
                        <Upload size={18} />
                        Upload
                    </button>
                    {!isSuperAdmin && (
                        <button className="btn btn-secondary" onClick={() => setShowRequest(true)}>
                            <Plus size={18} />
                            Request Access
                        </button>
                    )}
                </div>
            </header>

            <div className="search-bar glass-card">
                <Search size={20} />
                <input
                    type="text"
                    placeholder="Search documents..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                />
            </div>

            {/* My Documents (Hidden for Super Admin) */}
            {!isSuperAdmin && (
                <section className="document-section">
                    <h2>My Documents</h2>
                    {filteredDocs.length > 0 ? (
                        <div className="document-grid">
                            {filteredDocs.map(doc => (
                                <div key={doc.id} className="document-card glass-card">
                                    <a href={`/documents/${doc.id}`} className="doc-link">
                                        <div className="doc-icon">
                                            <FileText size={32} />
                                        </div>
                                        <div className="doc-details">
                                            <h3>{doc.title}</h3>
                                            <p>{doc.filename}</p>
                                            <span className="doc-meta">
                                                {formatFileSize(doc.file_size)} • {new Date(doc.created_at).toLocaleDateString()}
                                            </span>
                                        </div>
                                    </a>
                                    {(isSuperAdmin || doc.can_reassign) && (
                                        <button
                                            className="btn btn-small btn-secondary assign-btn"
                                            onClick={(e) => handleAssignClick(e, doc)}
                                        >
                                            <Users size={14} />
                                            Assign Users
                                        </button>
                                    )}
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="empty-state glass-card">
                            <FileText size={48} />
                            <h3>No documents</h3>
                            <p>You don't have any assigned documents yet</p>
                        </div>
                    )}
                </section>
            )}

            {/* All Documents (Super Admin Only) */}
            {isSuperAdmin && (
                <section className="document-section">
                    <h2>All Documents</h2>
                    {filteredAllDocs.length > 0 ? (
                        <div className="document-grid">
                            {filteredAllDocs.map(doc => (
                                <div key={doc.id} className="document-card glass-card">
                                    <a href={`/documents/${doc.id}`} className="doc-link">
                                        <div className="doc-icon">
                                            <FileText size={32} />
                                        </div>
                                        <div className="doc-details">
                                            <h3>{doc.title}</h3>
                                            <p>{doc.filename}</p>
                                            <span className="doc-meta">
                                                {formatFileSize(doc.file_size)} • {new Date(doc.created_at).toLocaleDateString()}
                                            </span>
                                        </div>
                                    </a>
                                    <button
                                        className="btn btn-small btn-secondary assign-btn"
                                        onClick={(e) => handleAssignClick(e, doc)}
                                    >
                                        <Users size={14} />
                                        Assign Users
                                    </button>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="empty-state glass-card">
                            <FileText size={48} />
                            <h3>No documents uploaded</h3>
                            <p>Upload your first document to get started</p>
                        </div>
                    )}
                </section>
            )}

            {/* Upload Modal */}
            {showUpload && (
                <UploadModal onClose={() => setShowUpload(false)} onSuccess={loadDocuments} />
            )}

            {/* Request Modal */}
            {showRequest && (
                <RequestModal
                    documents={documents}
                    onClose={() => setShowRequest(false)}
                    onSuccess={loadDocuments}
                />
            )}

            {/* Assign Users Modal */}
            {showAssign && selectedDoc && (
                <AssignUsersModal
                    document={selectedDoc}
                    currentUserRole={user?.role || ''}
                    onClose={() => {
                        setShowAssign(false)
                        setSelectedDoc(null)
                    }}
                    onSuccess={loadDocuments}
                />
            )}
        </div>
    )
}

function UploadModal({ onClose, onSuccess }: { onClose: () => void; onSuccess: () => void }) {
    const [file, setFile] = useState<File | null>(null)
    const [title, setTitle] = useState('')
    const [uploading, setUploading] = useState(false)
    const [error, setError] = useState('')

    const handleUpload = async () => {
        if (!file) return

        setUploading(true)
        setError('')

        try {
            await api.upload('/documents/upload', file, title || file.name)
            onSuccess()
            onClose()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Upload failed')
        } finally {
            setUploading(false)
        }
    }

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal glass-card" onClick={e => e.stopPropagation()}>
                <div className="modal-header">
                    <h2>Upload Document</h2>
                    <button className="close-btn" onClick={onClose}>
                        <X size={24} />
                    </button>
                </div>

                <div className="modal-body">
                    {error && <div className="error-message">{error}</div>}

                    <div className="input-group">
                        <label className="input-label">Title</label>
                        <input
                            type="text"
                            className="input"
                            value={title}
                            onChange={(e) => setTitle(e.target.value)}
                            placeholder="Document title"
                        />
                    </div>

                    <div className="file-input">
                        <input
                            type="file"
                            accept=".pdf"
                            onChange={(e) => setFile(e.target.files?.[0] || null)}
                        />
                        {file && <span className="file-name">{file.name}</span>}
                    </div>
                </div>

                <div className="modal-footer">
                    <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
                    <button
                        className="btn btn-primary"
                        onClick={handleUpload}
                        disabled={!file || uploading}
                    >
                        {uploading ? 'Uploading...' : 'Upload'}
                    </button>
                </div>
            </div>
        </div>
    )
}

function RequestModal({ documents, onClose, onSuccess }: {
    documents: Document[];
    onClose: () => void;
    onSuccess: () => void
}) {
    const [docName, setDocName] = useState('')
    const [reason, setReason] = useState('')
    const [targetAdmin, setTargetAdmin] = useState('')
    const [admins, setAdmins] = useState<User[]>([])
    const [submitting, setSubmitting] = useState(false)
    const [error, setError] = useState('')

    useEffect(() => {
        // Fetch admins for targeting
        api.get<{ users: User[] }>('/users').then(res => {
            const adminList = (res.data.users || []).filter((u: any) => u.role === 'admin' || u.role === 'super_admin')
            setAdmins(adminList)
        }).catch(console.error)
    }, [])

    const handleSubmit = async () => {
        if (!docName.trim()) return

        setSubmitting(true)
        setError('')

        try {
            let finalReason = reason
            if (targetAdmin) {
                const adminName = admins.find(a => a.id === targetAdmin)?.full_name || 'Admin'
                finalReason = `[Target: ${adminName}] ${reason}`
            }

            // Send document_id as null, use document_name
            await api.post('/requests', { document_name: docName, reason: finalReason })
            onSuccess()
            onClose()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Request failed')
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal glass-card" onClick={e => e.stopPropagation()}>
                <div className="modal-header">
                    <h2>Request Document Access</h2>
                    <button className="close-btn" onClick={onClose}>
                        <X size={24} />
                    </button>
                </div>

                <div className="modal-body">
                    {error && <div className="error-message">{error}</div>}

                    <div className="input-group">
                        <label className="input-label">Document Name</label>
                        <input
                            type="text"
                            className="input"
                            value={docName}
                            onChange={(e) => setDocName(e.target.value)}
                            placeholder="Enter document name (e.g., Q3 Report)"
                        />
                    </div>

                    <div className="input-group">
                        <label className="input-label">Reason</label>
                        <textarea
                            className="input"
                            value={reason}
                            onChange={(e) => setReason(e.target.value)}
                            placeholder="Why do you need access?"
                            rows={2}
                        />
                    </div>

                    <div className="input-group">
                        <label className="input-label">Target Admin (Optional)</label>
                        <select
                            className="input"
                            value={targetAdmin}
                            onChange={(e) => setTargetAdmin(e.target.value)}
                        >
                            <option value="">Broadcast to All Admins</option>
                            {admins.map(admin => (
                                <option key={admin.id} value={admin.id}>
                                    {admin.full_name} ({admin.role === 'super_admin' ? 'Super Admin' : 'Admin'})
                                </option>
                            ))}
                        </select>
                        <p className="hint-text">Select an admin to notify specifically.</p>
                    </div>
                </div>

                <div className="modal-footer">
                    <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
                    <button
                        className="btn btn-primary"
                        onClick={handleSubmit}
                        disabled={!docName.trim() || submitting}
                    >
                        {submitting ? 'Submitting...' : 'Submit Request'}
                    </button>
                </div>
            </div>
        </div>
    )
}

function AssignUsersModal({ document, currentUserRole, onClose, onSuccess }: {
    document: Document
    currentUserRole: string
    onClose: () => void
    onSuccess: () => void
}) {
    const { user } = useAuth()
    const [allUsers, setAllUsers] = useState<User[]>([])
    const [assignedUserIds, setAssignedUserIds] = useState<Set<string>>(new Set())
    const [originalAssignedIds, setOriginalAssignedIds] = useState<Set<string>>(new Set())

    // Track per-user reassign permission
    const [reassignPermissions, setReassignPermissions] = useState<Record<string, boolean>>({})
    const [originalPermissions, setOriginalPermissions] = useState<Record<string, boolean>>({})

    // Track detailed granter info
    const [granterDetails, setGranterDetails] = useState<Record<string, { name: string, email: string, role: string }>>({})

    // Popover state for "Assigned By"
    const [activePopover, setActivePopover] = useState<string | null>(null)

    const [loading, setLoading] = useState(true)
    const [saving, setSaving] = useState(false)
    const [error, setError] = useState('')

    const isSuperAdmin = currentUserRole === 'super_admin'

    useEffect(() => {
        loadData()
    }, [])

    const loadData = async () => {
        try {
            // Fetch all users
            const usersRes = await api.get<{ users: User[] }>('/users')
            const assignableUsers = (usersRes.data.users || []).filter(u =>
                (u as any).role !== 'super_admin' && u.id !== user?.id
            )
            setAllUsers(assignableUsers)

            // Fetch current document access
            const accessRes = await api.getDocumentAccess(document.id)

            const accessIds = new Set<string>()
            const perms: Record<string, boolean> = {}
            const granters: Record<string, { name: string, email: string, role: string }> = {}

                (accessRes.data.users || []).forEach((u: any) => {
                    accessIds.add(u.id)
                    perms[u.id] = u.can_reassign || false

                    if (u.granted_by_name) {
                        granters[u.id] = {
                            name: u.granted_by_name,
                            email: u.granted_by_email || 'N/A',
                            role: u.granted_by_role || 'N/A'
                        }
                    }
                })

            setAssignedUserIds(accessIds)
            setOriginalAssignedIds(new Set(accessIds))

            setReassignPermissions(perms)
            setOriginalPermissions({ ...perms })

            setGranterDetails(granters)

        } catch (err) {
            console.error('Failed to load data:', err)
            setError('Failed to load users')
        } finally {
            setLoading(false)
        }
    }

    const toggleUser = (userId: string) => {
        setAssignedUserIds(prev => {
            const newSet = new Set(prev)
            if (newSet.has(userId)) {
                newSet.delete(userId)
                // Also reset permission? Optional.
            } else {
                newSet.add(userId)
                // Default permission: false
            }
            return newSet
        })
    }

    const toggleReassign = (e: React.MouseEvent, userId: string) => {
        e.stopPropagation() // Prevent row click
        setReassignPermissions(prev => ({
            ...prev,
            [userId]: !prev[userId]
        }))
    }

    const handleSave = async () => {
        setSaving(true)
        setError('')

        try {
            // Find users to add OR update (if permission changed)
            const promises = []

            // 1. New assignments or Permission updates
            for (const userId of assignedUserIds) {
                const isNew = !originalAssignedIds.has(userId)
                const permChanged = reassignPermissions[userId] !== originalPermissions[userId]

                if (isNew || permChanged) {
                    // Update access (Grant updates existing too)
                    promises.push(api.post(`/documents/id/${document.id}/access`, {
                        user_id: userId,
                        can_reassign: !!reassignPermissions[userId]
                    }))
                }
            }

            // 2. Revocations
            for (const userId of originalAssignedIds) {
                if (!assignedUserIds.has(userId)) {
                    promises.push(api.revokeAccess(document.id, userId))
                }
            }

            await Promise.all(promises)

            onSuccess()
            onClose()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to save changes')
        } finally {
            setSaving(false)
        }
    }

    const hasChanges = () => {
        // Check assignments
        if (assignedUserIds.size !== originalAssignedIds.size) return true
        for (const id of assignedUserIds) {
            // Check if removed
            if (!originalAssignedIds.has(id)) return true
            // Check modified permissions
            if (reassignPermissions[id] !== originalPermissions[id]) return true
        }
        // Check if any removed
        for (const id of originalAssignedIds) {
            if (!assignedUserIds.has(id)) return true
        }
        return false
    }

    // Role formatter
    const formatRole = (role?: string) => {
        if (!role) return ''
        if (role === 'super_admin') return 'SUPER ADMIN'
        return role.toUpperCase()
    }

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal glass-card modal-large" onClick={e => {
                e.stopPropagation()
                setActivePopover(null) // Close popover on background click
            }}>
                <div className="modal-header">
                    <h2>Assign Users to: {document.title}</h2>
                    <button className="close-btn" onClick={onClose}>
                        <X size={24} />
                    </button>
                </div>

                <div className="modal-body">
                    {error && <div className="error-message">{error}</div>}

                    <div className="list-header">
                        <span>Select User</span>
                        <span>Permissions</span>
                    </div>

                    {loading ? (
                        <div className="loading-container">
                            <div className="loading-spinner" />
                        </div>
                    ) : allUsers.length === 0 ? (
                        <div className="empty-state">
                            <Users size={32} />
                            <p>No users available to assign</p>
                        </div>
                    ) : (
                        <div className="user-list">
                            {allUsers.map(user => {
                                const isSelected = assignedUserIds.has(user.id)
                                const userRole = (user as any).role || 'user'
                                const granter = granterDetails[user.id]

                                return (
                                    <div
                                        key={user.id}
                                        className={`user-item ${isSelected ? 'selected' : ''}`}
                                        onClick={() => toggleUser(user.id)}
                                    >
                                        <div className="user-left">
                                            <div className="user-checkbox">
                                                {isSelected && <Check size={16} />}
                                            </div>
                                            <div className="user-info">
                                                <div className="user-primary">
                                                    <span className="user-name">{user.full_name}</span>
                                                    <span className="user-role-badge">({formatRole(userRole)})</span>
                                                </div>
                                                <span className="user-email">{user.email}</span>
                                            </div>
                                        </div>

                                        <div className="user-right" onClick={e => e.stopPropagation()}>
                                            {/* Assigned By Info */}
                                            {isSelected && granter && (
                                                <div className="assigned-by-container">
                                                    <button
                                                        className="btn-text"
                                                        onClick={(e) => {
                                                            e.stopPropagation()
                                                            setActivePopover(activePopover === user.id ? null : user.id)
                                                        }}
                                                    >
                                                        Assigned by...
                                                    </button>

                                                    {activePopover === user.id && (
                                                        <div className="popover glass-card">
                                                            <h4>Assigned By:</h4>
                                                            <div className="popover-row"><strong>Name:</strong> {granter.name}</div>
                                                            <div className="popover-row"><strong>Email:</strong> {granter.email}</div>
                                                            <div className="popover-row"><strong>Role:</strong> {formatRole(granter.role)}</div>
                                                        </div>
                                                    )}
                                                </div>
                                            )}

                                            {/* Re-assign Permission Checkbox (Row-specific) */}
                                            {isSelected && (
                                                <label className="reassign-label" title="Allow this user to re-assign/share this document">
                                                    <input
                                                        type="checkbox"
                                                        checked={!!reassignPermissions[user.id]}
                                                        onChange={(e) => toggleReassign(e, user.id)}
                                                    />
                                                    <span>Allow Re-assign</span>
                                                </label>
                                            )}
                                        </div>
                                    </div>
                                )
                            })}
                        </div>
                    )}
                </div>

                <div className="modal-footer">
                    <span className="selection-count">
                        {assignedUserIds.size} user{assignedUserIds.size !== 1 ? 's' : ''} selected
                    </span>
                    <div className="footer-actions">
                        <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
                        <button
                            className="btn btn-primary"
                            onClick={handleSave}
                            disabled={!hasChanges() || saving}
                        >
                            {saving ? 'Saving...' : 'Save Changes'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
