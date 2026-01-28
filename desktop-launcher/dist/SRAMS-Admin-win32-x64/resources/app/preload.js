const { contextBridge, ipcRenderer } = require('electron');

// Expose protected methods to the renderer process
contextBridge.exposeInMainWorld('electronAPI', {
    login: (email, password) => ipcRenderer.invoke('login', email, password),
    openDashboard: (token) => ipcRenderer.invoke('open-dashboard', token),
    checkBackend: () => ipcRenderer.invoke('check-backend')
});
