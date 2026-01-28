import { useEffect, useState } from 'react'
import { api } from '../services/api_client'
import { useAuth } from '../hooks/useAuth'
import { ClipboardList, Search, Trash2, Filter, X } from 'lucide-react'
import { format } from 'date-fns'
import './AuditLogs.css'

interface AuditLog {
    id: string
    actor_id?: string
    actor_role: string
    action_type: string
    target_type: string
    target_id?: string
    metadata: Record<string, unknown>
    ip_address: string
    device_id: string
    user_agent: string
    created_at: string
    deleted_at?: string
    deletion_reason?: string
}

export default function AuditLogs() {
    const { user } = useAuth()
    const [logs, setLogs] = useState<AuditLog[]>([])
    const [loading, setLoading] = useState(true)
    const [total, setTotal] = useState(0)
    const [page, setPage] = useState(0)
    const [actionTypes, setActionTypes] = useState<string[]>([])
    const [filters, setFilters] = useState({
        action_type: '',
        start_date: '',
        end_date: '',
    })
    const [showDeleteModal, setShowDeleteModal] = useState(false)
    const [selectedLogs, setSelectedLogs] = useState<string[]>([])

    const isSuperAdmin = user?.role === 'super_admin'
    const limit = 50

    useEffect(() => {
        loadLogs()
        loadActionTypes()
    }, [page, filters])

    const loadLogs = async () => {
        setLoading(true)
        try {
            const params = new URLSearchParams()
            params.append('offset', String(page * limit))
            params.append('limit', String(limit))
            if (filters.action_type) params.append('action_type', filters.action_type)
            if (filters.start_date) params.append('start_date', filters.start_date)
            if (filters.end_date) params.append('end_date', filters.end_date)

            const response = await api.get<{ logs: AuditLog[]; total: number }>(`/audit?${params}`)
            setLogs(response.data.logs || [])
            setTotal(response.data.total)
        } catch (err) {
            console.error('Failed to load logs:', err)
        } finally {
            setLoading(false)
        }
    }

    const loadActionTypes = async () => {
        try {
            const response = await api.get<{ action_types: string[] }>('/audit/types')
            setActionTypes(response.data.action_types || [])
        } catch {
            // Ignore
        }
    }

    const handleDelete = async (reason: string) => {
        if (!selectedLogs.length || !reason) return

        try {
            await api.post('/audit/bulk-delete', {
                log_ids: selectedLogs,
                reason,
            })
            setSelectedLogs([])
            setShowDeleteModal(false)
            loadLogs()
        } catch (err) {
            alert(err instanceof Error ? err.message : 'Failed to delete logs')
        }
    }

    const toggleLogSelection = (id: string) => {
        setSelectedLogs(prev =>
            prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]
        )
    }

    const totalPages = Math.ceil(total / limit)

    return (
        <div className="audit-page animate-fadeIn">
            <header className="page-header">
                <div>
                    <h1>Audit Logs</h1>
                    <p>View system activity and security events</p>
                </div>

                {isSuperAdmin && selectedLogs.length > 0 && (
                    <button className="btn btn-danger" onClick={() => setShowDeleteModal(true)}>
                        <Trash2 size={18} />
                        Delete Selected ({selectedLogs.length})
                    </button>
                )}
            </header>

            {/* Filters */}
            <div className="filters glass-card">
                <div className="filter-group">
                    <Filter size={20} />
                    <span>Filters:</span>
                </div>

                <select
                    className="input"
                    value={filters.action_type}
                    onChange={(e) => setFilters({ ...filters, action_type: e.target.value })}
                >
                    <option value="">All Actions</option>
                    {actionTypes.map(type => (
                        <option key={type} value={type}>{formatActionType(type)}</option>
                    ))}
                </select>

                <input
                    type="date"
                    className="input"
                    value={filters.start_date}
                    onChange={(e) => setFilters({ ...filters, start_date: e.target.value })}
                    placeholder="Start Date"
                />

                <input
                    type="date"
                    className="input"
                    value={filters.end_date}
                    onChange={(e) => setFilters({ ...filters, end_date: e.target.value })}
                    placeholder="End Date"
                />

                {(filters.action_type || filters.start_date || filters.end_date) && (
                    <button
                        className="btn btn-secondary"
                        onClick={() => setFilters({ action_type: '', start_date: '', end_date: '' })}
                    >
                        Clear
                    </button>
                )}
            </div>

            {/* Logs Table */}
            <div className="table-container glass-card">
                {loading ? (
                    <div className="loading-container">
                        <div className="loading-spinner" />
                    </div>
                ) : (
                    <table className="table">
                        <thead>
                            <tr>
                                {isSuperAdmin && <th className="checkbox-col"></th>}
                                <th>Timestamp</th>
                                <th>Action</th>
                                <th>Actor</th>
                                <th>Target</th>
                                <th>IP Address</th>
                                <th>Details</th>
                            </tr>
                        </thead>
                        <tbody>
                            {logs.map(log => (
                                <tr key={log.id} className={log.deleted_at ? 'deleted' : ''}>
                                    {isSuperAdmin && (
                                        <td className="checkbox-col">
                                            {!log.deleted_at && (
                                                <input
                                                    type="checkbox"
                                                    checked={selectedLogs.includes(log.id)}
                                                    onChange={() => toggleLogSelection(log.id)}
                                                />
                                            )}
                                        </td>
                                    )}
                                    <td className="timestamp">
                                        {format(new Date(log.created_at), 'MMM d, yyyy HH:mm:ss')}
                                    </td>
                                    <td>
                                        <span className={`badge badge-${getActionBadge(log.action_type)}`}>
                                            {formatActionType(log.action_type)}
                                        </span>
                                    </td>
                                    <td>
                                        <span className="actor-role">{log.actor_role || 'System'}</span>
                                    </td>
                                    <td>{log.target_type || '-'}</td>
                                    <td className="ip-cell">{log.ip_address || '-'}</td>
                                    <td>
                                        {Object.keys(log.metadata || {}).length > 0 && (
                                            <code className="metadata">
                                                {JSON.stringify(log.metadata).slice(0, 50)}...
                                            </code>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                )}

                {logs.length === 0 && !loading && (
                    <div className="empty-state">
                        <ClipboardList size={48} />
                        <p>No audit logs found</p>
                    </div>
                )}

                {/* Pagination */}
                {totalPages > 1 && (
                    <div className="pagination">
                        <button
                            className="btn btn-secondary"
                            disabled={page === 0}
                            onClick={() => setPage(p => p - 1)}
                        >
                            Previous
                        </button>
                        <span>Page {page + 1} of {totalPages}</span>
                        <button
                            className="btn btn-secondary"
                            disabled={page >= totalPages - 1}
                            onClick={() => setPage(p => p + 1)}
                        >
                            Next
                        </button>
                    </div>
                )}
            </div>

            {/* Delete Modal */}
            {showDeleteModal && (
                <DeleteModal
                    count={selectedLogs.length}
                    onClose={() => setShowDeleteModal(false)}
                    onConfirm={handleDelete}
                />
            )}
        </div>
    )
}

function DeleteModal({ count, onClose, onConfirm }: {
    count: number
    onClose: () => void
    onConfirm: (reason: string) => void
}) {
    const [reason, setReason] = useState('')

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal glass-card" onClick={e => e.stopPropagation()}>
                <div className="modal-header">
                    <h2>Delete Audit Logs</h2>
                    <button className="close-btn" onClick={onClose}><X size={24} /></button>
                </div>

                <div className="modal-body">
                    <p className="warning-text">
                        You are about to delete {count} audit log(s). This action is logged and cannot be undone.
                    </p>

                    <div className="input-group">
                        <label className="input-label">Deletion Reason (required)</label>
                        <textarea
                            className="input"
                            value={reason}
                            onChange={(e) => setReason(e.target.value)}
                            placeholder="Explain why these logs are being deleted..."
                            rows={3}
                            required
                        />
                    </div>
                </div>

                <div className="modal-footer">
                    <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
                    <button
                        className="btn btn-danger"
                        onClick={() => onConfirm(reason)}
                        disabled={!reason.trim()}
                    >
                        Delete Logs
                    </button>
                </div>
            </div>
        </div>
    )
}

function formatActionType(type: string): string {
    return type.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ')
}

function getActionBadge(type: string): string {
    if (type.includes('login') || type.includes('logout')) return 'info'
    if (type.includes('failed') || type.includes('delete')) return 'error'
    if (type.includes('create') || type.includes('approve')) return 'success'
    return 'warning'
}
