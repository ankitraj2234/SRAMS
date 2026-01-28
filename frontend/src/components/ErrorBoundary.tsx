import { Component, ErrorInfo, ReactNode } from 'react'

interface Props {
    children: ReactNode
}

interface State {
    hasError: boolean
    error: Error | null
    errorInfo: ErrorInfo | null
}

export default class ErrorBoundary extends Component<Props, State> {
    public state: State = {
        hasError: false,
        error: null,
        errorInfo: null
    }

    public static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error, errorInfo: null }
    }

    public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        console.error('ErrorBoundary caught an error:', error, errorInfo)
        this.setState({ errorInfo })
    }

    public render() {
        if (this.state.hasError) {
            return (
                <div style={{
                    padding: '40px',
                    textAlign: 'center',
                    background: 'var(--color-bg-primary, #1a1a2e)',
                    color: 'var(--color-text-primary, #fff)',
                    minHeight: '100vh',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center'
                }}>
                    <h1 style={{ color: '#ff6b6b', marginBottom: '20px' }}>Something went wrong</h1>
                    <p style={{ marginBottom: '20px', opacity: 0.8 }}>
                        An error occurred while rendering the page.
                    </p>
                    <details style={{
                        textAlign: 'left',
                        maxWidth: '600px',
                        background: 'rgba(0,0,0,0.3)',
                        padding: '20px',
                        borderRadius: '8px',
                        marginBottom: '20px'
                    }}>
                        <summary style={{ cursor: 'pointer', marginBottom: '10px' }}>Error Details</summary>
                        <pre style={{
                            whiteSpace: 'pre-wrap',
                            wordBreak: 'break-word',
                            fontSize: '12px',
                            color: '#ff6b6b'
                        }}>
                            {this.state.error?.toString()}
                            {'\n\n'}
                            {this.state.errorInfo?.componentStack}
                        </pre>
                    </details>
                    <button
                        onClick={() => window.location.reload()}
                        style={{
                            padding: '12px 24px',
                            background: 'var(--color-accent-primary, #6366f1)',
                            color: '#fff',
                            border: 'none',
                            borderRadius: '8px',
                            cursor: 'pointer',
                            fontSize: '16px'
                        }}
                    >
                        Reload Page
                    </button>
                </div>
            )
        }

        return this.props.children
    }
}
