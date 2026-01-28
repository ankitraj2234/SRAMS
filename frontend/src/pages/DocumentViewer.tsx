import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { api } from '../services/api_client'
import * as pdfjsLib from 'pdfjs-dist'
import { ArrowLeft, ZoomIn, ZoomOut, ChevronLeft, ChevronRight, X } from 'lucide-react'
import './DocumentViewer.css'

// Set up PDF.js worker - use local bundled version for offline support
import pdfjsWorker from 'pdfjs-dist/build/pdf.worker.min.mjs?url'
pdfjsLib.GlobalWorkerOptions.workerSrc = pdfjsWorker

export default function DocumentViewer() {
    const { id } = useParams<{ id: string }>()
    const { user } = useAuth()
    const navigate = useNavigate()
    const canvasRef = useRef<HTMLCanvasElement>(null)
    const containerRef = useRef<HTMLDivElement>(null)
    const logoImageRef = useRef<HTMLImageElement | null>(null)

    const [pdf, setPdf] = useState<pdfjsLib.PDFDocumentProxy | null>(null)
    const [currentPage, setCurrentPage] = useState(1)
    const [totalPages, setTotalPages] = useState(0)
    const [scale, setScale] = useState(1.2)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [viewId, setViewId] = useState<string | null>(null)
    const [pagesViewed, setPagesViewed] = useState<number[]>([])
    const [logoLoaded, setLogoLoaded] = useState(false)
    const [watermarkOpacity, setWatermarkOpacity] = useState(20) // Default 20%
    const [settingsLoaded, setSettingsLoaded] = useState(false) // Track when settings are ready

    useEffect(() => {
        loadDocument()

        // Disable context menu
        const handleContextMenu = (e: Event) => e.preventDefault()
        document.addEventListener('contextmenu', handleContextMenu)

        // Keyboard navigation
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'ArrowLeft') {
                setCurrentPage(prev => Math.max(1, prev - 1))
            } else if (e.key === 'ArrowRight') {
                setCurrentPage(prev => Math.min(totalPages || 1, prev + 1))
            } else if (e.key === '-' || e.key === '_') {
                setScale(prev => Math.max(0.5, prev - 0.2))
            } else if (e.key === '+' || e.key === '=') {
                setScale(prev => Math.min(3, prev + 0.2))
            }
        }
        document.addEventListener('keydown', handleKeyDown)

        // Log page view
        api.logEvent('document_view', 'document', id)

        return () => {
            document.removeEventListener('contextmenu', handleContextMenu)
            document.removeEventListener('keydown', handleKeyDown)
            // End view session
            if (viewId) {
                api.post(`/documents/view/${viewId}/end`, { pages_viewed: pagesViewed })
            }
        }
    }, [id, totalPages])

    useEffect(() => {
        // Only render when PDF is loaded AND settings are ready
        if (pdf && settingsLoaded) {
            renderPage()
        }
    }, [pdf, currentPage, scale, settingsLoaded])

    const loadDocument = async () => {
        try {
            // Load company logo for watermark
            try {
                const logoHeaders: Record<string, string> = {
                    Authorization: `Bearer ${localStorage.getItem('access_token')}`,
                }
                const desktopSession = localStorage.getItem('desktop_session')
                if (desktopSession) {
                    logoHeaders['X-Desktop-Session'] = desktopSession
                }

                const logoResponse = await fetch('/api/system/logo', {
                    headers: logoHeaders
                })
                if (logoResponse.ok) {
                    const logoBlob = await logoResponse.blob()
                    const logoUrl = URL.createObjectURL(logoBlob)
                    const img = document.createElement('img') as HTMLImageElement
                    img.src = logoUrl
                    await new Promise((resolve, reject) => {
                        img.onload = resolve
                        img.onerror = reject
                    })
                    logoImageRef.current = img
                    setLogoLoaded(true)
                    console.log('Custom logo loaded successfully')
                }
            } catch (err) {
                // No logo or failed to load - fall back to text
                console.log('No custom logo found, using default text', err)
            }

            // Load watermark opacity setting from system config
            try {
                const configHeaders: Record<string, string> = {
                    Authorization: `Bearer ${localStorage.getItem('access_token')}`,
                }
                const desktopSession = localStorage.getItem('desktop_session')
                if (desktopSession) {
                    configHeaders['X-Desktop-Session'] = desktopSession
                }

                const configResponse = await fetch('/api/system/config', { headers: configHeaders })
                if (configResponse.ok) {
                    const configData = await configResponse.json()
                    const opacityValue = parseInt(configData.config?.watermark_opacity || '20', 10)
                    setWatermarkOpacity(opacityValue)
                    console.log('Watermark opacity loaded:', opacityValue)
                }
            } catch (err) {
                console.log('Failed to load opacity setting, using default', err)
            }

            // Mark settings as loaded so rendering can proceed
            setSettingsLoaded(true)

            const headers: Record<string, string> = {
                Authorization: `Bearer ${localStorage.getItem('access_token')}`,
                'X-Device-ID': localStorage.getItem('device_id') || '',
            }

            // Add desktop session for Super Admin access
            const desktopSession = localStorage.getItem('desktop_session')
            if (desktopSession) {
                headers['X-Desktop-Session'] = desktopSession
            }

            const response = await fetch(`/api/documents/id/${id}/view`, {
                headers,
                credentials: 'include',
            })

            if (!response.ok) {
                throw new Error('Failed to load document')
            }

            // Get view ID from header
            const viewIdHeader = response.headers.get('X-View-ID')
            if (viewIdHeader) {
                setViewId(viewIdHeader)
            }

            const blob = await response.blob()
            const arrayBuffer = await blob.arrayBuffer()

            const loadingTask = pdfjsLib.getDocument({ data: arrayBuffer })
            const pdfDoc = await loadingTask.promise

            setPdf(pdfDoc)
            setTotalPages(pdfDoc.numPages)
            setLoading(false)
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to load document')
            setLoading(false)
        }
    }

    // Track active render task to prevent "same canvas" errors
    const renderTaskRef = useRef<any>(null)

    const renderPage = async () => {
        if (!pdf || !canvasRef.current) return

        // Cancel previous render task if it exists
        if (renderTaskRef.current) {
            try {
                await renderTaskRef.current.cancel()
            } catch (ignore) {
                // Expected error on cancellation
            }
            renderTaskRef.current = null
        }

        const page = await pdf.getPage(currentPage)

        // Use device pixel ratio for sharp rendering on high-DPI screens
        const devicePixelRatio = window.devicePixelRatio || 1
        const viewport = page.getViewport({ scale: scale * devicePixelRatio })

        const canvas = canvasRef.current
        const context = canvas.getContext('2d')!

        // Set canvas dimensions for high-DPI rendering
        canvas.height = viewport.height
        canvas.width = viewport.width

        // Scale canvas back down using CSS for display
        canvas.style.width = `${viewport.width / devicePixelRatio}px`
        canvas.style.height = `${viewport.height / devicePixelRatio}px`

        // Reset canvas state completely before rendering
        context.save()
        context.setTransform(1, 0, 0, 1, 0, 0)
        context.globalCompositeOperation = 'source-over'
        context.globalAlpha = 1
        context.clearRect(0, 0, canvas.width, canvas.height)
        context.restore()

        // Render PDF page
        const renderContext = {
            canvasContext: context,
            viewport,
        }

        const renderTask = page.render(renderContext)
        renderTaskRef.current = renderTask

        try {
            await renderTask.promise
            renderTaskRef.current = null // Clear on success
        } catch (err: any) {
            if (err.name === 'RenderingCancelledException') {
                return // Ignore cancelled renders
            }
            console.error('Render error:', err)
        }

        // Draw watermark on top with transparency (no destination-over)
        drawWatermark(context, canvas.width, canvas.height, devicePixelRatio)

        // Record page view
        if (!pagesViewed.includes(currentPage)) {
            setPagesViewed(prev => [...prev, currentPage])
            if (viewId) {
                api.post(`/documents/view/${viewId}/page/${currentPage}`, {})
            }
        }
    }

    const drawWatermark = (ctx: CanvasRenderingContext2D, width: number, height: number, dpr: number = 1) => {
        // Get client IP from localStorage if available (set by backend)
        const clientIP = localStorage.getItem('client_ip') || 'Local'

        // Format current time
        const now = new Date()
        const timeStr = now.toLocaleDateString() + ' ' + now.toLocaleTimeString()

        ctx.save()

        // Use multiply blend mode so watermark appears BEHIND text
        // Multiply makes dark text show through, watermark visible in light areas
        ctx.globalCompositeOperation = 'multiply'
        ctx.globalAlpha = watermarkOpacity / 100 // Convert percentage to decimal
        ctx.textAlign = 'center'

        // Diagonal pattern
        const angle = -35 * Math.PI / 180
        ctx.rotate(angle)

        // Spacing for watermark grid
        const xSpacing = 450 * dpr
        const ySpacing = 300 * dpr

        // Logo dimensions (scaled for high-DPI)
        const logoWidth = 120 * dpr
        const logoHeight = 50 * dpr

        for (let x = -width; x < width * 2; x += xSpacing) {
            for (let y = -height; y < height * 2; y += ySpacing) {
                let yOffset = 0

                // Draw logo or text
                if (logoImageRef.current) {
                    // Draw uploaded company logo
                    ctx.drawImage(
                        logoImageRef.current,
                        x - logoWidth / 2, y - logoHeight / 2,
                        logoWidth, logoHeight
                    )
                    yOffset = logoHeight / 2 + 15 * dpr
                } else {
                    // Draw default SRAMS text logo
                    ctx.fillStyle = '#0066CC'
                    ctx.font = `bold ${28 * dpr}px Arial, sans-serif`
                    ctx.fillText('📄 SRAMS', x, y)
                    yOffset = 30 * dpr
                }

                // Draw username
                ctx.fillStyle = '#333333'
                ctx.font = `bold ${16 * dpr}px Arial, sans-serif`
                ctx.fillText(user?.full_name || 'User', x, y + yOffset)

                // Draw IP address
                ctx.fillStyle = '#666666'
                ctx.font = `${14 * dpr}px Arial, sans-serif`
                ctx.fillText(`IP: ${clientIP}`, x, y + yOffset + 20 * dpr)

                // Draw timestamp
                ctx.fillText(timeStr, x, y + yOffset + 40 * dpr)

                // Draw confidential notice
                ctx.fillStyle = '#CC0000'
                ctx.font = `bold ${12 * dpr}px Arial, sans-serif`
                ctx.fillText('🔒 CONFIDENTIAL - DO NOT COPY', x, y + yOffset + 65 * dpr)
            }
        }

        ctx.restore()
    }


    const goToPage = (page: number) => {
        if (page >= 1 && page <= totalPages) {
            setCurrentPage(page)
        }
    }

    const adjustZoom = (delta: number) => {
        setScale(prev => Math.max(0.5, Math.min(3, prev + delta)))
    }

    if (loading) {
        return (
            <div className="viewer-loading">
                <div className="loading-spinner" />
                <p>Loading document...</p>
            </div>
        )
    }

    if (error) {
        return (
            <div className="viewer-error">
                <p>{error}</p>
                <button className="btn btn-primary" onClick={() => navigate('/documents')}>
                    Back to Documents
                </button>
            </div>
        )
    }

    return (
        <div className="document-viewer" ref={containerRef}>
            {/* Toolbar */}
            <div className="viewer-toolbar glass">
                <button className="toolbar-btn" onClick={() => navigate('/documents')}>
                    <ArrowLeft size={20} />
                </button>

                <div className="toolbar-separator" />

                <div className="page-controls">
                    <button
                        className="toolbar-btn"
                        onClick={() => goToPage(currentPage - 1)}
                        disabled={currentPage <= 1}
                    >
                        <ChevronLeft size={20} />
                    </button>
                    <span className="page-indicator">
                        {currentPage} / {totalPages}
                    </span>
                    <button
                        className="toolbar-btn"
                        onClick={() => goToPage(currentPage + 1)}
                        disabled={currentPage >= totalPages}
                    >
                        <ChevronRight size={20} />
                    </button>
                </div>

                <div className="toolbar-separator" />

                <div className="zoom-controls">
                    <button className="toolbar-btn" onClick={() => adjustZoom(-0.2)}>
                        <ZoomOut size={20} />
                    </button>
                    <span className="zoom-indicator">{Math.round(scale * 100)}%</span>
                    <button className="toolbar-btn" onClick={() => adjustZoom(0.2)}>
                        <ZoomIn size={20} />
                    </button>
                </div>

                {/* Spacer to push Close button to the right */}
                <div style={{ flex: 1 }} />

                <button
                    className="toolbar-btn close-viewer-btn"
                    onClick={() => navigate('/documents')}
                    title="Close Viewer"
                >
                    <X size={20} />
                </button>
            </div>

            {/* Canvas Container */}
            <div className="canvas-container">
                <canvas ref={canvasRef} className="pdf-canvas" />
            </div>

            {/* Security Overlay - prevents screenshots to some degree */}
            <div className="security-overlay" />
        </div>
    )
}
