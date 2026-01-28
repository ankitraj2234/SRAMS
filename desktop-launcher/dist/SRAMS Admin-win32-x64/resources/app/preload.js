const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
    // Server control
    checkServer: () => ipcRenderer.invoke('check-server'),
    startServer: () => ipcRenderer.invoke('start-server'),
    stopServer: () => ipcRenderer.invoke('stop-server'),
    runSetup: () => ipcRenderer.invoke('run-setup'),

    // Authentication
    login: (email, password) => ipcRenderer.invoke('login', email, password),
    logout: () => ipcRenderer.invoke('logout'),

    // Dashboard
    openDashboard: () => ipcRenderer.invoke('open-dashboard'),

    // Status
    getStatus: () => ipcRenderer.invoke('get-status'),
    getConnectionInfo: () => ipcRenderer.invoke('get-connection-info')
});
