import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { useTheme } from '../hooks/useTheme'
import { useRealtime } from '../hooks/useRealtime'
import {
    LayoutDashboard,
    FileText,
    Users,
    ClipboardList,
    Settings,
    LogOut,
    Moon,
    Sun,
    Shield,
    Menu,
    X
} from 'lucide-react'
import { useState, useCallback } from 'react'
import './Layout.css'

export default function Layout() {
    const { user, logout } = useAuth()
    const { theme, toggleTheme } = useTheme()
    const navigate = useNavigate()
    const [sidebarOpen, setSidebarOpen] = useState(false)

    // Handle real-time config updates
    const handleConfigUpdate = useCallback((key: string, value: unknown) => {
        console.log(`[SSE] Config updated: ${key} =`, value)
        // Dispatch custom event for components to listen
        window.dispatchEvent(new CustomEvent('srams:config-update', {
            detail: { key, value }
        }))
    }, [])

    // Handle forced logout
    const handleForceLogout = useCallback((reason: string) => {
        console.log(`[SSE] Force logout:`, reason)
        // Alert is shown by useRealtime hook
    }, [])

    // Connect to SSE for real-time updates
    useRealtime({
        onConfigUpdate: handleConfigUpdate,
        onForceLogout: handleForceLogout,
        onConnected: (clientId) => console.log(`[SSE] Connected: ${clientId}`),
        onError: () => console.warn('[SSE] Connection error, will retry...')
    })

    const handleLogout = async () => {
        await logout()
        navigate('/login')
    }

    const isAdmin = user?.role === 'admin' || user?.role === 'super_admin'
    const isSuperAdmin = user?.role === 'super_admin'

    const navItems = [
        { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
        { to: '/documents', icon: FileText, label: 'Documents' },
        ...(isAdmin ? [
            { to: '/users', icon: Users, label: 'Users' },
            { to: '/audit', icon: ClipboardList, label: 'Audit Logs' },
        ] : []),
        { to: '/settings', icon: Settings, label: 'Settings' },
    ]

    return (
        <div className="layout">
            {/* Mobile Header */}
            <header className="mobile-header">
                <button className="menu-btn" onClick={() => setSidebarOpen(true)}>
                    <Menu size={24} />
                </button>
                <div className="logo">
                    <Shield size={24} />
                    <span>SRAMS</span>
                </div>
                <button className="theme-btn" onClick={toggleTheme}>
                    {theme === 'twilight' ? <Sun size={20} /> : <Moon size={20} />}
                </button>
            </header>

            {/* Overlay */}
            {sidebarOpen && <div className="sidebar-overlay" onClick={() => setSidebarOpen(false)} />}

            {/* Sidebar */}
            <aside className={`sidebar ${sidebarOpen ? 'open' : ''}`}>
                <div className="sidebar-header">
                    <div className="logo">
                        <Shield size={28} />
                        <span>SRAMS</span>
                    </div>
                    <button className="close-btn" onClick={() => setSidebarOpen(false)}>
                        <X size={24} />
                    </button>
                </div>

                <nav className="sidebar-nav">
                    {navItems.map(item => (
                        <NavLink
                            key={item.to}
                            to={item.to}
                            className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}
                            onClick={() => setSidebarOpen(false)}
                        >
                            <item.icon size={20} />
                            <span>{item.label}</span>
                        </NavLink>
                    ))}
                </nav>

                <div className="sidebar-footer">
                    <div className="user-info">
                        <div className="user-avatar">
                            {(user?.full_name || '').charAt(0).toUpperCase()}
                        </div>
                        <div className="user-details">
                            <span className="user-name">{user?.full_name || 'User'}</span>
                            <span className="user-role">
                                {isSuperAdmin ? 'Super Admin' : isAdmin ? 'Admin' : 'User'}
                            </span>
                        </div>
                    </div>

                    <div className="sidebar-actions">
                        <button className="action-btn" onClick={toggleTheme} title="Toggle theme">
                            {theme === 'twilight' ? <Sun size={18} /> : <Moon size={18} />}
                        </button>
                        <button className="action-btn logout" onClick={handleLogout} title="Logout">
                            <LogOut size={18} />
                        </button>
                    </div>
                </div>
            </aside>

            {/* Main Content */}
            <main className="main-content">
                <Outlet />
            </main>
        </div>
    )
}
