const { app, BrowserWindow, ipcMain, shell } = require('electron');
const path = require('path');
const fs = require('fs');
const http = require('http');
const os = require('os');

// Configuration
const DEFAULT_BACKEND_PORT = 8080;
const DEFAULT_FRONTEND_PORT = 3000;

let mainWindow;
let backendUrl = 'http://localhost:8080';
let frontendUrl = 'http://localhost:3000';

// Detect the best IP to use
function detectBackendUrl() {
    // First, try to read from config file (set by installer)
    const configPaths = [
        path.join(process.resourcesPath || '', '..', 'config', '.machine_ip'),
        path.join(__dirname, '..', 'config', '.machine_ip'),
        'C:\\Program Files\\SRAMS\\config\\.machine_ip'
    ];

    for (const configPath of configPaths) {
        try {
            if (fs.existsSync(configPath)) {
                const ip = fs.readFileSync(configPath, 'utf8').trim();
                if (ip && ip !== '127.0.0.1') {
                    console.log(`Found machine IP from config: ${ip}`);
                    return `http://${ip}:${DEFAULT_BACKEND_PORT}`;
                }
            }
        } catch (e) {
            // Continue to next path
        }
    }

    // Fallback: detect local network IP
    try {
        const interfaces = os.networkInterfaces();
        for (const name of Object.keys(interfaces)) {
            for (const iface of interfaces[name]) {
                if (iface.family === 'IPv4' && !iface.internal) {
                    console.log(`Detected network IP: ${iface.address}`);
                    return `http://${iface.address}:${DEFAULT_BACKEND_PORT}`;
                }
            }
        }
    } catch (e) {
        console.error('Failed to detect network interfaces:', e);
    }

    // Final fallback
    return 'http://localhost:8080';
}

function detectFrontendUrl() {
    // In production, frontend is served from the same machine
    const configPaths = [
        path.join(process.resourcesPath || '', '..', 'config', '.machine_ip'),
        path.join(__dirname, '..', 'config', '.machine_ip'),
        'C:\\Program Files\\SRAMS\\config\\.machine_ip'
    ];

    for (const configPath of configPaths) {
        try {
            if (fs.existsSync(configPath)) {
                const ip = fs.readFileSync(configPath, 'utf8').trim();
                if (ip && ip !== '127.0.0.1') {
                    return `http://${ip}:${DEFAULT_FRONTEND_PORT}`;
                }
            }
        } catch (e) {
            // Continue
        }
    }

    return 'http://localhost:3000';
}

function createWindow() {
    // Initialize URLs
    backendUrl = detectBackendUrl();
    frontendUrl = detectFrontendUrl();
    console.log(`Backend URL: ${backendUrl}`);
    console.log(`Frontend URL: ${frontendUrl}`);

    mainWindow = new BrowserWindow({
        width: 450,
        height: 580,
        resizable: false,
        frame: true,
        autoHideMenuBar: true,
        icon: path.join(__dirname, 'icon.ico'),
        webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            preload: path.join(__dirname, 'preload.js')
        }
    });

    mainWindow.loadFile('index.html');

    // Disable menu
    mainWindow.setMenu(null);
}

app.whenReady().then(createWindow);

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit();
    }
});

app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
        createWindow();
    }
});

// Handle login request from renderer
ipcMain.handle('login', async (event, email, password) => {
    return new Promise((resolve) => {
        const postData = JSON.stringify({
            email: email,
            password: password
        });

        const url = new URL(`${backendUrl}/api/v1/auth/login`);

        const options = {
            hostname: url.hostname,
            port: url.port || 8080,
            path: url.pathname,
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Content-Length': Buffer.byteLength(postData)
            }
        };

        const req = http.request(options, (res) => {
            let data = '';

            res.on('data', (chunk) => {
                data += chunk;
            });

            res.on('end', () => {
                try {
                    const response = JSON.parse(data);

                    if (res.statusCode === 200 && response.access_token) {
                        // Check if user is super_admin
                        if (response.user && response.user.role === 'super_admin') {
                            resolve({
                                success: true,
                                token: response.access_token,
                                user: response.user,
                                frontendUrl: frontendUrl
                            });
                        } else {
                            resolve({
                                success: false,
                                error: 'Access denied. This launcher is only for Super Admin users.\n\nRegular users should access via browser.'
                            });
                        }
                    } else {
                        resolve({
                            success: false,
                            error: response.error || 'Invalid credentials'
                        });
                    }
                } catch (e) {
                    resolve({ success: false, error: 'Server error: Invalid response' });
                }
            });
        });

        req.on('error', (error) => {
            console.error('Login request error:', error);
            resolve({
                success: false,
                error: 'Cannot connect to SRAMS server.\n\nPlease ensure:\n• SRAMS service is running\n• Firewall allows port 8080'
            });
        });

        req.setTimeout(15000, () => {
            req.destroy();
            resolve({ success: false, error: 'Connection timeout. Server may be starting up.' });
        });

        req.write(postData);
        req.end();
    });
});

// Handle opening browser
ipcMain.handle('open-dashboard', async (event, token) => {
    // Open browser to frontend URL with token
    const dashboardUrl = `${frontendUrl}?token=${encodeURIComponent(token)}`;
    shell.openExternal(dashboardUrl);

    // Close the launcher after a short delay
    setTimeout(() => {
        app.quit();
    }, 1500);

    return true;
});

// Check if backend is running
ipcMain.handle('check-backend', async () => {
    return new Promise((resolve) => {
        const url = new URL(`${backendUrl}/api/v1/health`);

        const req = http.get({
            hostname: url.hostname,
            port: url.port || 8080,
            path: url.pathname,
            timeout: 5000
        }, (res) => {
            resolve(res.statusCode === 200);
        });

        req.on('error', () => {
            resolve(false);
        });

        req.on('timeout', () => {
            req.destroy();
            resolve(false);
        });
    });
});

// Get connection info for display
ipcMain.handle('get-connection-info', async () => {
    return {
        backendUrl: backendUrl,
        frontendUrl: frontendUrl
    };
});
