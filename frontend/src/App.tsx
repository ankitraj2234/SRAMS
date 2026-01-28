import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './hooks/useAuth'
import { ThemeProvider } from './hooks/useTheme'
import ErrorBoundary from './components/ErrorBoundary'
import Login from './pages/Login'
import ForceChangePassword from './pages/ForceChangePassword'
import ForceMfaEnrollment from './pages/ForceMfaEnrollment'
import Dashboard from './pages/Dashboard'
import Documents from './pages/Documents'
import DocumentViewer from './pages/DocumentViewer'
import Users from './pages/Users'
import AuditLogs from './pages/AuditLogs'
import Settings from './pages/Settings'
import Layout from './components/Layout'

function ProtectedRoute({ children, roles }: { children: React.ReactNode; roles?: string[] }) {
    const { user, loading } = useAuth()

    if (loading) {
        return (
            <div className="flex items-center justify-center" style={{ height: '100vh' }}>
                <div className="animate-spin" style={{ width: 40, height: 40, border: '3px solid var(--color-border)', borderTopColor: 'var(--color-accent-primary)', borderRadius: '50%' }} />
            </div>
        )
    }

    if (!user) {
        return <Navigate to="/login" replace />
    }

    if (roles && !roles.includes(user.role)) {
        return <Navigate to="/dashboard" replace />
    }

    return <>{children}</>
}

function App() {
    return (
        <ErrorBoundary>
            <AuthProvider>
                <ThemeProvider>
                    <BrowserRouter>
                        <Routes>
                            <Route path="/login" element={<Login />} />
                            <Route path="/force-change-password" element={<ForceChangePassword />} />
                            <Route path="/force-enroll-mfa" element={<ForceMfaEnrollment />} />
                            <Route
                                path="/"
                                element={
                                    <ProtectedRoute>
                                        <Layout />
                                    </ProtectedRoute>
                                }
                            >
                                <Route index element={<Navigate to="/dashboard" replace />} />
                                <Route path="dashboard" element={<Dashboard />} />
                                <Route path="documents" element={<Documents />} />
                                <Route path="documents/:id" element={<DocumentViewer />} />
                                <Route
                                    path="users"
                                    element={
                                        <ProtectedRoute roles={['admin', 'super_admin']}>
                                            <Users />
                                        </ProtectedRoute>
                                    }
                                />
                                <Route
                                    path="audit"
                                    element={
                                        <ProtectedRoute roles={['admin', 'super_admin']}>
                                            <AuditLogs />
                                        </ProtectedRoute>
                                    }
                                />
                                <Route path="settings" element={<Settings />} />
                            </Route>
                        </Routes>
                    </BrowserRouter>
                </ThemeProvider>
            </AuthProvider>
        </ErrorBoundary>
    )
}

export default App
