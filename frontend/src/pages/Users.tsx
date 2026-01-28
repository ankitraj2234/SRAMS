import { useEffect, useState } from 'react'
import { api } from '../services/api_client'
import { useAuth } from '../hooks/useAuth'
import { Users as UsersIcon, Plus, Edit, Trash2, X, Search, Upload, Download, FileSpreadsheet } from 'lucide-react'
import ImportUsersModal from '../components/ImportUsersModal'
import './Users.css'

interface User {
    id: string
    email: string
    full_name: string
    mobile: string
    role: string
    is_active: boolean
    created_at: string
    last_login?: string
}

export default function Users() {
    const { user: currentUser } = useAuth()
    const [users, setUsers] = useState<User[]>([])
    const [loading, setLoading] = useState(true)
    const [search, setSearch] = useState('')
    const [roleFilter, setRoleFilter] = useState('')
    const [showModal, setShowModal] = useState(false)
    const [showImportModal, setShowImportModal] = useState(false)
    const [editingUser, setEditingUser] = useState<User | null>(null)

    const isSuperAdmin = currentUser?.role === 'super_admin'
    const isAdmin = currentUser?.role === 'admin'

    useEffect(() => {
        loadUsers()
    }, [roleFilter])

    const loadUsers = async () => {
        try {
            const query = roleFilter ? `?role=${roleFilter}` : ''
            const response = await api.get<{ users: User[] }>(`/users${query}`)
            setUsers(response.data.users || [])
        } catch (err) {
            console.error('Failed to load users:', err)
        } finally {
            setLoading(false)
        }
    }

    const handleDelete = async (user: User) => {
        if (!confirm(`Are you sure you want to delete ${user.full_name}?`)) return

        try {
            await api.delete(`/users/${user.id}`)
            loadUsers()
        } catch (err) {
            alert(err instanceof Error ? err.message : 'Failed to delete user')
        }
    }

    const handleExport = async () => {
        try {
            const blob = await api.downloadBlob('/users/bulk/export')
            const url = window.URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = 'srams_users_export.xlsx'
            a.click()
            window.URL.revokeObjectURL(url)
        } catch (err) {
            alert(err instanceof Error ? err.message : 'Failed to export users')
        }
    }

    const handleDownloadTemplate = async () => {
        try {
            const blob = await api.downloadBlob('/users/bulk/template')
            const url = window.URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = 'srams_user_import_template.xlsx'
            a.click()
            window.URL.revokeObjectURL(url)
        } catch (err) {
            alert(err instanceof Error ? err.message : 'Failed to download template')
        }
    }

    // Filter users based on role visibility rules:
    // - Admins should NOT see super_admin users
    // - Super Admins can see everyone
    const visibleUsers = users.filter(user => {
        // If current user is not super_admin, hide super_admin users from list
        if (!isSuperAdmin && user.role === 'super_admin') {
            return false
        }
        return true
    })

    const filteredUsers = visibleUsers.filter(user =>
        user.full_name.toLowerCase().includes(search.toLowerCase()) ||
        user.email.toLowerCase().includes(search.toLowerCase())
    )

    // Check if current user can perform actions on a target user
    const canEditUser = (targetUser: User): boolean => {
        // Users cannot edit themselves
        if (targetUser.id === currentUser?.id) return false
        // Non-super_admin cannot edit super_admin
        if (!isSuperAdmin && targetUser.role === 'super_admin') return false
        return true
    }

    const canDeleteUser = (targetUser: User): boolean => {
        // Cannot delete yourself
        if (targetUser.id === currentUser?.id) return false
        // Cannot delete super_admin (only super_admin role management)
        if (targetUser.role === 'super_admin') return false
        // Admins cannot delete other admins
        if (isAdmin && !isSuperAdmin && targetUser.role === 'admin') return false
        return true
    }

    if (loading) {
        return (
            <div className="loading-container">
                <div className="loading-spinner" />
            </div>
        )
    }

    return (
        <div className="users-page animate-fadeIn">
            <header className="page-header">
                <div>
                    <h1>Users</h1>
                    <p>Manage system users and their roles</p>
                </div>

                <div className="header-actions">
                    <div className="bulk-actions">
                        <button className="btn btn-outline" onClick={() => setShowImportModal(true)}>
                            <Upload size={16} />
                            Import
                        </button>
                        <button className="btn btn-outline" onClick={handleExport}>
                            <Download size={16} />
                            Export
                        </button>
                        <button className="btn btn-outline" onClick={handleDownloadTemplate}>
                            <FileSpreadsheet size={16} />
                            Template
                        </button>
                    </div>
                    <button className="btn btn-primary" onClick={() => { setEditingUser(null); setShowModal(true) }}>
                        <Plus size={18} />
                        Add User
                    </button>
                </div>
            </header>

            <div className="filters glass-card">
                <div className="search-input">
                    <Search size={20} />
                    <input
                        type="text"
                        placeholder="Search users..."
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                    />
                </div>

                <select
                    className="input"
                    value={roleFilter}
                    onChange={(e) => setRoleFilter(e.target.value)}
                >
                    <option value="">All Roles</option>
                    <option value="user">Users</option>
                    <option value="admin">Admins</option>
                    {isSuperAdmin && <option value="super_admin">Super Admins</option>}
                </select>
            </div>

            <div className="table-container glass-card">
                <table className="table">
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Email</th>
                            <th>Role</th>
                            <th>Status</th>
                            <th>Last Login</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {filteredUsers.map(user => (
                            <tr key={user.id}>
                                <td>
                                    <div className="user-cell">
                                        <div className="user-avatar">
                                            {user.full_name.charAt(0).toUpperCase()}
                                        </div>
                                        <span>{user.full_name}</span>
                                    </div>
                                </td>
                                <td>{user.email}</td>
                                <td>
                                    <span className={`badge badge-${getRoleBadge(user.role)}`}>
                                        {formatRole(user.role)}
                                    </span>
                                </td>
                                <td>
                                    <span className={`badge badge-${user.is_active ? 'success' : 'error'}`}>
                                        {user.is_active ? 'Active' : 'Inactive'}
                                    </span>
                                </td>
                                <td>
                                    {user.last_login
                                        ? new Date(user.last_login).toLocaleDateString()
                                        : 'Never'}
                                </td>
                                <td>
                                    <div className="action-buttons">
                                        {canEditUser(user) && (
                                            <button
                                                className="action-btn"
                                                onClick={() => { setEditingUser(user); setShowModal(true) }}
                                            >
                                                <Edit size={16} />
                                            </button>
                                        )}
                                        {canDeleteUser(user) && (
                                            <button
                                                className="action-btn danger"
                                                onClick={() => handleDelete(user)}
                                            >
                                                <Trash2 size={16} />
                                            </button>
                                        )}
                                    </div>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>

                {filteredUsers.length === 0 && (
                    <div className="empty-state">
                        <UsersIcon size={48} />
                        <p>No users found</p>
                    </div>
                )}
            </div>

            {showModal && (
                <UserModal
                    user={editingUser}
                    isSuperAdmin={isSuperAdmin}
                    onClose={() => setShowModal(false)}
                    onSuccess={() => { setShowModal(false); loadUsers() }}
                />
            )}

            {showImportModal && (
                <ImportUsersModal
                    onClose={() => setShowImportModal(false)}
                    onSuccess={() => { setShowImportModal(false); loadUsers() }}
                />
            )}
        </div>
    )
}

function UserModal({ user, isSuperAdmin, onClose, onSuccess }: {
    user: User | null
    isSuperAdmin: boolean
    onClose: () => void
    onSuccess: () => void
}) {
    const [formData, setFormData] = useState({
        email: user?.email || '',
        password: '',
        full_name: user?.full_name || '',
        mobile: user?.mobile || '',
        role: user?.role || 'user',
        is_active: user?.is_active ?? true,
        must_change_password: true,
        must_enroll_mfa: false,
    })
    const [submitting, setSubmitting] = useState(false)
    const [error, setError] = useState('')

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setSubmitting(true)
        setError('')

        try {
            if (user) {
                await api.put(`/users/${user.id}`, {
                    full_name: formData.full_name,
                    mobile: formData.mobile,
                    role: isSuperAdmin ? formData.role : undefined,
                    is_active: formData.is_active,
                })
            } else {
                await api.post('/users', formData)
            }
            onSuccess()
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to save user')
        } finally {
            setSubmitting(false)
        }
    }

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal glass-card" onClick={e => e.stopPropagation()}>
                <div className="modal-header">
                    <h2>{user ? 'Edit User' : 'Add User'}</h2>
                    <button className="close-btn" onClick={onClose}><X size={24} /></button>
                </div>

                <form onSubmit={handleSubmit} className="modal-body">
                    {error && <div className="error-message">{error}</div>}

                    {!user && (
                        <>
                            <div className="input-group">
                                <label className="input-label">Email</label>
                                <input
                                    type="email"
                                    className="input"
                                    value={formData.email}
                                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                                    required
                                />
                            </div>

                            <div className="input-group">
                                <label className="input-label">Password</label>
                                <input
                                    type="password"
                                    className="input"
                                    value={formData.password}
                                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                                    required
                                    minLength={8}
                                />
                            </div>

                            <div className="input-group">
                                <label className="checkbox-label">
                                    <input
                                        type="checkbox"
                                        checked={formData.must_change_password}
                                        onChange={(e) => setFormData({ ...formData, must_change_password: e.target.checked })}
                                    />
                                    <span>Force password change on first login</span>
                                </label>
                            </div>

                            <div className="input-group">
                                <label className="checkbox-label">
                                    <input
                                        type="checkbox"
                                        checked={formData.must_enroll_mfa}
                                        onChange={(e) => setFormData({ ...formData, must_enroll_mfa: e.target.checked })}
                                    />
                                    <span>Require MFA enrollment (Google Authenticator)</span>
                                </label>
                            </div>
                        </>
                    )}

                    <div className="input-group">
                        <label className="input-label">Full Name</label>
                        <input
                            type="text"
                            className="input"
                            value={formData.full_name}
                            onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                            required
                        />
                    </div>

                    <div className="input-group">
                        <label className="input-label">Mobile</label>
                        <input
                            type="tel"
                            className="input"
                            value={formData.mobile}
                            onChange={(e) => setFormData({ ...formData, mobile: e.target.value })}
                        />
                    </div>

                    <div className="input-group">
                        <label className="input-label">Role</label>
                        <select
                            className="input"
                            value={formData.role}
                            onChange={(e) => setFormData({ ...formData, role: e.target.value })}
                            disabled={!isSuperAdmin || user?.role === 'super_admin'}
                        >
                            <option value="user">User</option>
                            <option value="admin">Admin</option>
                        </select>
                    </div>

                    {user && (
                        <div className="input-group">
                            <label className="checkbox-label">
                                <input
                                    type="checkbox"
                                    checked={formData.is_active}
                                    onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
                                />
                                <span>Active</span>
                            </label>
                        </div>
                    )}

                    <div className="modal-footer">
                        <button type="button" className="btn btn-secondary" onClick={onClose}>Cancel</button>
                        <button type="submit" className="btn btn-primary" disabled={submitting}>
                            {submitting ? 'Saving...' : 'Save'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}

function formatRole(role: string): string {
    return role.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ')
}

function getRoleBadge(role: string): string {
    switch (role) {
        case 'super_admin': return 'error'
        case 'admin': return 'warning'
        default: return 'info'
    }
}
