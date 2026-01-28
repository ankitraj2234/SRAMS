import { useEffect, useState } from 'react'
import { useAuth } from '../hooks/useAuth'
import { api } from '../services/api_client'
import {
    Users,
    FileText,
    Activity,
    Clock,
    TrendingUp,
    AlertCircle,
    CheckCircle,
    XCircle
} from 'lucide-react'
import './Dashboard.css'

interface DashboardStats {
    total_users: number
    active_users: number
    total_documents: number
    pending_requests: number
    active_sessions: number
    today_logins: number
    total_audit_logs: number
}

interface UserDocument {
    id: string
    title: string
    filename: string
    created_at: string
}

interface UserRequest {
    id: string
    document_id: string
    status: string
    reason: string
    created_at: string
}

export default function Dashboard() {
    const { user } = useAuth()
    const [stats, setStats] = useState<DashboardStats | null>(null)
    const [documents, setDocuments] = useState<UserDocument[]>([])
    const [requests, setRequests] = useState<UserRequest[]>([])
    const [loading, setLoading] = useState(true)

    const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'

    useEffect(() => {
        loadDashboard()
        api.logEvent('page_view', 'dashboard')
    }, [])

    const loadDashboard = async () => {
        try {
            if (isAdmin) {
                const statsRes = await api.get<DashboardStats>('/dashboard/stats')
                setStats(statsRes.data)
            }

            const docsRes = await api.get<{ documents: UserDocument[] }>('/documents/my?limit=5')
            setDocuments(docsRes.data.documents || [])

            const reqRes = await api.get<{ requests: UserRequest[] }>('/requests/my?limit=5')
            setRequests(reqRes.data.requests || [])
        } catch (err) {
            console.error('Failed to load dashboard:', err)
        } finally {
            setLoading(false)
        }
    }

    if (loading) {
        return (
            <div className="loading-container">
                <div className="loading-spinner" />
            </div>
        )
    }

    return (
        <div className="dashboard animate-fadeIn">
            <header className="page-header">
                <h1>Welcome back, {user?.full_name}</h1>
                <p>
                    {isAdmin
                        ? 'Here\'s an overview of your system'
                        : 'Access your documents and manage requests'}
                </p>
            </header>

            {/* Admin Stats */}
            {isAdmin && stats && (
                <div className="stats-grid">
                    <StatCard
                        icon={<Users />}
                        label="Total Users"
                        value={stats.total_users}
                        subtext={`${stats.active_users ?? 0} active`}
                        color="blue"
                    />
                    <StatCard
                        icon={<FileText />}
                        label="Documents"
                        value={stats.total_documents}
                        color="purple"
                    />
                    <StatCard
                        icon={<AlertCircle />}
                        label="Pending Requests"
                        value={stats.pending_requests}
                        color="yellow"
                    />
                    <StatCard
                        icon={<Activity />}
                        label="Active Sessions"
                        value={stats.active_sessions}
                        subtext={`${stats.today_logins ?? 0} logins today`}
                        color="green"
                    />
                </div>
            )}

            <div className="dashboard-grid">
                {/* Recent Documents */}
                <div className="dashboard-section glass-card">
                    <div className="section-header">
                        <h2><FileText size={20} /> My Documents</h2>
                        <a href="/documents" className="view-all">View All →</a>
                    </div>

                    {documents.length > 0 ? (
                        <div className="document-list">
                            {documents.map(doc => (
                                <a key={doc.id} href={`/documents/${doc.id}`} className="document-item">
                                    <FileText size={18} />
                                    <div className="document-info">
                                        <span className="document-title">{doc.title}</span>
                                        <span className="document-date">
                                            {new Date(doc.created_at).toLocaleDateString()}
                                        </span>
                                    </div>
                                </a>
                            ))}
                        </div>
                    ) : (
                        <div className="empty-state">
                            <FileText size={40} />
                            <p>No documents assigned yet</p>
                        </div>
                    )}
                </div>

                {/* Recent Requests */}
                <div className="dashboard-section glass-card">
                    <div className="section-header">
                        <h2><Clock size={20} /> My Requests</h2>
                    </div>

                    {requests.length > 0 ? (
                        <div className="request-list">
                            {requests.map(req => (
                                <div key={req.id} className="request-item">
                                    <StatusIcon status={req.status} />
                                    <div className="request-info">
                                        <span className="request-reason">{req.reason || 'Document access request'}</span>
                                        <span className="request-date">
                                            {new Date(req.created_at).toLocaleDateString()}
                                        </span>
                                    </div>
                                    <span className={`badge badge-${getStatusColor(req.status)}`}>
                                        {req.status}
                                    </span>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="empty-state">
                            <Clock size={40} />
                            <p>No requests yet</p>
                        </div>
                    )}
                </div>
            </div>

            {/* Quick Actions */}
            <div className="quick-actions glass-card">
                <h2>Quick Actions</h2>
                <div className="action-buttons">
                    <a href="/documents" className="btn btn-primary">
                        <FileText size={18} />
                        Browse Documents
                    </a>
                    {isAdmin && (
                        <>
                            <a href="/users" className="btn btn-secondary">
                                <Users size={18} />
                                Manage Users
                            </a>
                            <a href="/audit" className="btn btn-secondary">
                                <Activity size={18} />
                                View Audit Logs
                            </a>
                        </>
                    )}
                </div>
            </div>
        </div>
    )
}

function StatCard({ icon, label, value, subtext, color }: {
    icon: React.ReactNode
    label: string
    value: number | undefined
    subtext?: string
    color: string
}) {
    return (
        <div className={`stat-card glass-card stat-${color}`}>
            <div className="stat-icon">{icon}</div>
            <div className="stat-content">
                <span className="stat-value">{(value ?? 0).toLocaleString()}</span>
                <span className="stat-label">{label}</span>
                {subtext && <span className="stat-subtext">{subtext}</span>}
            </div>
        </div>
    )
}

function StatusIcon({ status }: { status: string }) {
    switch (status) {
        case 'approved':
            return <CheckCircle size={18} className="status-approved" />
        case 'rejected':
            return <XCircle size={18} className="status-rejected" />
        default:
            return <Clock size={18} className="status-pending" />
    }
}

function getStatusColor(status: string): string {
    switch (status) {
        case 'approved': return 'success'
        case 'rejected': return 'error'
        default: return 'warning'
    }
}
