/// <reference types="vite/client" />

// PDF.js worker URL import
declare module 'pdfjs-dist/build/pdf.worker.min.mjs?url' {
    const workerUrl: string
    export default workerUrl
}
