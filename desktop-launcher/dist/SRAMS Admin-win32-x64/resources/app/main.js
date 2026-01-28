const { app, BrowserWindow, ipcMain, shell } = require('electron');
const { exec } = require('child_process');
const path = require('path');
const fs = require('fs');
const http = require('http');
const os = require('os');
const certificateService = require('./certificateService');

// Configuration
const DEFAULT_BACKEND_PORT = 8080;

let mainWindow;
let backendUrl = 'http://127.0.0.1:8080';
// Frontend is now served by backend on same port
let frontendUrl = 'http://127.0.0.1:8080';
let desktopSessionToken = null;
let accessToken = null;
let refreshToken = null;
let isLoggedIn = false;
let currentUser = null;

// Get install path from various locations
function getInstallPath() {
    // When packaged, the app is in {installDir}/admin-launcher/resources/app/
    // So we need to go up to find the install directory
    const possiblePaths = [
        // Packaged app: go up from resources/app to admin-launcher, then to install root
        path.resolve(app.getAppPath(), '..', '..', '..'),
        // Installed location (Inno Setup default)
        'C:\\Program Files\\SRAMS Enterprise',
        // Alternative install location
        'C:\\Program Files\\SRAMS',
        // Development: desktop-launcher is sibling to backend
        path.join(__dirname, '..')
    ];

    console.log('[Main] Looking for install path...');
    for (const p of possiblePaths) {
        const backendDir = path.join(p, 'backend');
        const backendExe = path.join(backendDir, 'srams-server.exe');
        console.log(`[Main] Checking: ${p}`);
        console.log(`[Main]   backend dir exists: ${fs.existsSync(backendDir)}`);
        console.log(`[Main]   exe exists: ${fs.existsSync(backendExe)}`);
        if (fs.existsSync(backendDir)) {
            console.log(`[Main] Found install path: ${p}`);
            return p;
        }
    }
    console.log('[Main] No valid install path found, using default');
    return 'C:\\Program Files\\SRAMS Enterprise';
}

function createWindow() {
    mainWindow = new BrowserWindow({
        width: 500,
        height: 700,
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
    mainWindow.setMenu(null);

    // Handle window close - logout Super Admin but don't stop server
    mainWindow.on('close', async (e) => {
        if (isLoggedIn && desktopSessionToken) {
            e.preventDefault();

            // End desktop session (logout Super Admin from browser)
            await endDesktopSession();

            // Now close the window
            mainWindow.destroy();
        }
    });
}

// End desktop session
async function endDesktopSession() {
    if (desktopSessionToken) {
        try {
            await httpRequest(`${backendUrl}/api/system/desktop-session`, 'DELETE');
        } catch (e) {
            console.error('Failed to end desktop session:', e);
        }
        desktopSessionToken = null;
        isLoggedIn = false;
        currentUser = null;
    }
}

// HTTP request helper
function httpRequest(url, method = 'GET', body = null) {
    return new Promise((resolve, reject) => {
        const urlObj = new URL(url);
        const options = {
            hostname: urlObj.hostname,
            port: urlObj.port || 80,
            path: urlObj.pathname,
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            timeout: 10000
        };

        if (body) {
            options.headers['Content-Length'] = Buffer.byteLength(JSON.stringify(body));
        }

        const req = http.request(options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                try {
                    resolve({ status: res.statusCode, data: JSON.parse(data) });
                } catch {
                    resolve({ status: res.statusCode, data: data });
                }
            });
        });

        req.on('error', reject);
        req.on('timeout', () => {
            req.destroy();
            reject(new Error('Request timeout'));
        });

        if (body) {
            req.write(JSON.stringify(body));
        }
        req.end();
    });
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

// Query Windows service status
function getServiceStatus(serviceName) {
    return new Promise((resolve) => {
        exec(`sc query ${serviceName}`, (error, stdout) => {
            if (error) {
                resolve({ exists: false, running: false, status: 'NOT_FOUND' });
                return;
            }

            const running = stdout.includes('RUNNING');
            const stopped = stdout.includes('STOPPED');
            const starting = stdout.includes('START_PENDING');
            const stopping = stdout.includes('STOP_PENDING');

            let status = 'UNKNOWN';
            if (running) status = 'RUNNING';
            else if (stopped) status = 'STOPPED';
            else if (starting) status = 'STARTING';
            else if (stopping) status = 'STOPPING';

            resolve({ exists: true, running, status });
        });
    });
}

// Check server status with detailed service info
ipcMain.handle('check-server', async () => {
    try {
        // Check PostgreSQL service status
        const pgStatus = await getServiceStatus('srams-postgresql');

        // Check backend service status
        const backendStatus = await getServiceStatus('srams-backend');

        // Check if backend is actually responding
        let backendHealthy = false;
        try {
            const result = await httpRequest(`${backendUrl}/api/system/health`);
            backendHealthy = result.status === 200;
        } catch (e) {
            backendHealthy = false;
        }

        // Determine overall status
        const running = backendHealthy;

        return {
            running,
            postgresql: {
                installed: pgStatus.exists,
                status: pgStatus.status,
                running: pgStatus.running
            },
            backend: {
                installed: backendStatus.exists,
                status: backendStatus.status,
                running: backendStatus.running,
                healthy: backendHealthy
            },
            message: running
                ? 'Server is running'
                : !pgStatus.running
                    ? 'PostgreSQL is not running'
                    : !backendHealthy
                        ? 'Backend is not responding'
                        : 'Server is stopped'
        };
    } catch (e) {
        console.error('[Main] Error checking server:', e);
        return {
            running: false,
            message: 'Error checking server status: ' + e.message
        };
    }
});

// Check if a port is listening
function checkPort(port, timeout = 2000) {
    return new Promise((resolve) => {
        const net = require('net');
        const socket = new net.Socket();

        socket.setTimeout(timeout);
        socket.on('connect', () => {
            socket.destroy();
            resolve(true);
        });
        socket.on('timeout', () => {
            socket.destroy();
            resolve(false);
        });
        socket.on('error', () => {
            socket.destroy();
            resolve(false);
        });

        socket.connect(port, '127.0.0.1');
    });
}

// Wait for a port to become available
async function waitForPort(port, maxWaitMs = 30000) {
    const startTime = Date.now();
    while (Date.now() - startTime < maxWaitMs) {
        if (await checkPort(port)) {
            return true;
        }
        await new Promise(r => setTimeout(r, 500));
    }
    return false;
}

// Start PostgreSQL (simplified - installer handles initialization)
async function startPostgreSQL() {
    console.log('[Main] Checking PostgreSQL status...');

    // Check if already running
    if (await checkPort(5432)) {
        console.log('[Main] PostgreSQL is already running on port 5432');
        return { success: true, message: 'PostgreSQL already running' };
    }

    console.log('[Main] PostgreSQL not running, attempting to start...');

    // ProgramData paths (professional pattern)
    const programData = 'C:\\ProgramData\\SRAMS';
    const pgData = path.join(programData, 'postgres');
    const pgLog = path.join(programData, 'logs', 'postgres.log');

    const installPath = getInstallPath();
    const pgBin = path.join(installPath, 'pgsql', 'bin');
    const pgCtl = path.join(pgBin, 'pg_ctl.exe');
    const pgVersionFile = path.join(pgData, 'PG_VERSION');

    // Check if cluster exists
    if (!fs.existsSync(pgVersionFile)) {
        console.log('[Main] PostgreSQL cluster not found. Run Setup first.');
        return { success: false, error: 'PostgreSQL not initialized. Click "Run Setup" button.' };
    }

    // Try to start the srams-postgresql service
    return new Promise((resolve) => {
        exec('net start srams-postgresql', { timeout: 30000 }, async (error) => {
            if (error) {
                console.log('[Main] Service start failed, trying pg_ctl...');

                if (fs.existsSync(pgCtl)) {
                    exec(`"${pgCtl}" -D "${pgData}" -l "${pgLog}" start`, { timeout: 30000 }, async (err2) => {
                        if (err2) {
                            console.error('[Main] pg_ctl start failed:', err2.message);
                            resolve({ success: false, error: 'Failed to start PostgreSQL: ' + err2.message });
                        } else {
                            const ready = await waitForPort(5432, 30000);
                            if (ready) {
                                console.log('[Main] PostgreSQL started via pg_ctl');
                                resolve({ success: true, message: 'PostgreSQL started' });
                            } else {
                                resolve({ success: false, error: 'PostgreSQL started but port not responding' });
                            }
                        }
                    });
                } else {
                    resolve({ success: false, error: 'PostgreSQL binaries not found. Please reinstall.' });
                }
            } else {
                const ready = await waitForPort(5432, 30000);
                if (ready) {
                    console.log('[Main] PostgreSQL service started');
                    resolve({ success: true, message: 'PostgreSQL started' });
                } else {
                    resolve({ success: false, error: 'PostgreSQL service started but port not responding' });
                }
            }
        });
    });
}

// Run setup with elevated privileges (for Program Files)
async function runElevatedSetup(installPath) {
    const setupScript = path.join(installPath, 'scripts', 'setup-postgres-complete.ps1');

    if (fs.existsSync(setupScript)) {
        console.log('[Main] Running elevated setup script...');

        return new Promise((resolve) => {
            // Use PowerShell to run with elevation
            const cmd = `powershell -Command "Start-Process powershell -ArgumentList '-ExecutionPolicy Bypass -File \\"${setupScript}\\" -InstallPath \\"${installPath}\\"' -Verb RunAs -Wait"`;

            exec(cmd, { timeout: 300000 }, (error, stdout, stderr) => {
                if (error) {
                    console.error('[Main] Elevated setup failed:', error.message);
                    resolve({ success: false, error: error.message });
                    return;
                }
                console.log('[Main] Elevated setup completed');
                resolve({ success: true });
            });
        });
    } else {
        return { success: false, error: 'Setup script not found' };
    }
}

// Manual PostgreSQL initialization (fallback)
async function runManualInit(installPath) {
    const pgBin = path.join(installPath, 'pgsql', 'bin');
    const pgData = path.join(installPath, 'data', 'postgres');
    const initdbExe = path.join(pgBin, 'initdb.exe');
    const logsDir = path.join(installPath, 'logs');

    console.log('[Main] Running manual PostgreSQL initialization...');

    // Create directories
    try {
        if (!fs.existsSync(pgData)) {
            fs.mkdirSync(pgData, { recursive: true });
        }
        if (!fs.existsSync(logsDir)) {
            fs.mkdirSync(logsDir, { recursive: true });
        }
    } catch (e) {
        console.error('[Main] Directory creation failed:', e);
        return { success: false, error: 'Cannot create data directory. Run as Administrator.' };
    }

    // Run initdb
    return new Promise((resolve) => {
        exec(`"${initdbExe}" -D "${pgData}" -U postgres -E UTF8 --locale=C`,
            { timeout: 120000 },
            (error, stdout, stderr) => {
                if (error) {
                    console.error('[Main] initdb failed:', stderr || error.message);
                    resolve({ success: false, error: 'initdb failed: ' + (stderr || error.message) });
                } else {
                    console.log('[Main] PostgreSQL cluster initialized');

                    // Configure pg_hba.conf for trust authentication
                    const pgHbaConf = path.join(pgData, 'pg_hba.conf');
                    const pgHbaContent = `# PostgreSQL Authentication
host    all    all    127.0.0.1/32    trust
host    all    all    ::1/128         trust
`;
                    try {
                        fs.writeFileSync(pgHbaConf, pgHbaContent);
                    } catch (e) {
                        console.error('[Main] Could not write pg_hba.conf:', e);
                    }

                    resolve({ success: true });
                }
            });
    });
}

// Setup database, user, and run migrations
async function setupDatabase(psqlExe, migrationsDir) {
    console.log('[Main] Setting up database...');

    if (!fs.existsSync(psqlExe)) {
        console.error('[Main] psql not found');
        return;
    }

    // Create database
    await new Promise((resolve) => {
        exec(`"${psqlExe}" -h localhost -U postgres -c "CREATE DATABASE srams;"`,
            { timeout: 30000 }, (err) => {
                if (err) console.log('[Main] Database may already exist');
                resolve();
            });
    });

    // Create user
    await new Promise((resolve) => {
        exec(`"${psqlExe}" -h localhost -U postgres -c "CREATE USER srams_app WITH PASSWORD 'srams_secure_2024';"`,
            { timeout: 30000 }, (err) => {
                if (err) console.log('[Main] User may already exist');
                resolve();
            });
    });

    // Grant privileges
    await new Promise((resolve) => {
        exec(`"${psqlExe}" -h localhost -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE srams TO srams_app;"`,
            { timeout: 30000 }, () => resolve());
    });

    await new Promise((resolve) => {
        exec(`"${psqlExe}" -h localhost -U postgres -d srams -c "GRANT ALL ON SCHEMA public TO srams_app;"`,
            { timeout: 30000 }, () => resolve());
    });

    // Enable extensions
    await new Promise((resolve) => {
        exec(`"${psqlExe}" -h localhost -U postgres -d srams -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;"`,
            { timeout: 30000 }, () => resolve());
    });

    // Run migrations
    if (fs.existsSync(migrationsDir)) {
        const files = fs.readdirSync(migrationsDir).filter(f => f.endsWith('.sql')).sort();
        for (const file of files) {
            const filePath = path.join(migrationsDir, file);
            console.log(`[Main] Running migration: ${file}`);
            await new Promise((resolve) => {
                exec(`"${psqlExe}" -h localhost -U postgres -d srams -f "${filePath}"`,
                    { timeout: 60000 }, () => resolve());
            });
        }
    }

    console.log('[Main] Database setup complete');
}

// Start Backend
async function startBackend() {
    console.log('[Main] Starting backend...');

    // Try Windows service first
    return new Promise((resolve) => {
        exec('net start srams-backend', { timeout: 30000 }, async (error) => {
            if (!error) {
                console.log('[Main] Backend service started');
                resolve({ success: true, message: 'Backend service started' });
                return;
            }

            console.log('[Main] Service start failed, trying direct exe...');

            // Fallback: Start exe directly
            const installPath = getInstallPath();
            const exePath = path.join(installPath, 'backend', 'srams-server.exe');
            const configDir = path.join(installPath, 'config');

            if (!fs.existsSync(exePath)) {
                resolve({ success: false, error: 'Backend executable not found' });
                return;
            }

            const workDir = fs.existsSync(configDir) ? configDir : path.join(installPath, 'backend');
            console.log(`[Main] Starting backend from: ${exePath}`);
            console.log(`[Main] Working directory: ${workDir}`);

            // Use spawn instead of exec for better control
            const { spawn } = require('child_process');
            const child = spawn(exePath, [], {
                cwd: workDir,
                detached: true,
                stdio: 'ignore'
            });
            child.unref();

            console.log('[Main] Backend process spawned');
            resolve({ success: true, message: 'Backend started' });
        });
    });
}

// Start server (PostgreSQL-first sequence)
ipcMain.handle('start-server', async () => {
    console.log('[Main] ========== Starting Server ==========');

    // Step 1: Ensure PostgreSQL is running
    console.log('[Main] Step 1: Starting PostgreSQL...');
    const pgResult = await startPostgreSQL();
    if (!pgResult.success) {
        console.error('[Main] PostgreSQL start failed:', pgResult.error);
        return { success: false, error: 'PostgreSQL: ' + pgResult.error };
    }

    // Step 2: Start backend
    console.log('[Main] Step 2: Starting Backend...');
    const backendResult = await startBackend();
    if (!backendResult.success) {
        console.error('[Main] Backend start failed:', backendResult.error);
        return { success: false, error: 'Backend: ' + backendResult.error };
    }

    // Step 3: Wait for backend health check
    console.log('[Main] Step 3: Waiting for backend health...');
    const maxWait = 30000;
    const startTime = Date.now();

    while (Date.now() - startTime < maxWait) {
        try {
            const health = await httpRequest(`${backendUrl}/api/system/health`);
            if (health.status === 200) {
                console.log('[Main] Backend is healthy!');
                return { success: true, message: 'Server started successfully' };
            }
        } catch (e) {
            // Still waiting
        }
        await new Promise(r => setTimeout(r, 1000));
    }

    return { success: false, error: 'Backend started but health check failed after 30 seconds' };
});

// Run setup manually (for when installer failed)
ipcMain.handle('run-setup', async () => {
    console.log('[Main] Running manual setup...');

    const installPath = getInstallPath();
    const setupResult = await runElevatedSetup(installPath);

    if (!setupResult.success) {
        return setupResult;
    }

    // Now start PostgreSQL and set up database
    const pgResult = await startPostgreSQL();
    if (!pgResult.success) {
        return { success: false, error: 'Setup complete but PostgreSQL failed to start: ' + pgResult.error };
    }

    return { success: true, message: 'Setup completed successfully. You can now start the server.' };
});

// Stop server (requires login)
ipcMain.handle('stop-server', async () => {
    if (!isLoggedIn) {
        return { success: false, error: 'You must be logged in to stop the server' };
    }

    return new Promise((resolve) => {
        // Try using PowerShell to stop the service (works better than net stop)
        const stopCmd = 'powershell -Command "Stop-Service -Name srams-backend -Force -ErrorAction SilentlyContinue"';

        exec(stopCmd, { timeout: 30000 }, (error) => {
            if (error) {
                // Fallback: Try net stop
                exec('net stop srams-backend', { timeout: 30000 }, (err2) => {
                    if (err2) {
                        // Last resort: taskkill
                        exec('taskkill /F /IM srams-server.exe', (err3) => {
                            if (err3) {
                                resolve({ success: false, error: 'Failed to stop server. Try running as Administrator.' });
                            } else {
                                resolve({ success: true, message: 'Server stopped' });
                            }
                        });
                    } else {
                        resolve({ success: true, message: 'Server stopped' });
                    }
                });
            } else {
                // Wait a moment for the service to fully stop
                setTimeout(() => resolve({ success: true, message: 'Server stopped' }), 1000);
            }
        });
    });
});

// Login
ipcMain.handle('login', async (event, email, password) => {
    try {
        // Verify or generate device certificate
        console.log('[Main] Checking device certificate...');
        let certResult = certificateService.verifyCertificate();

        // If no certificate exists, generate one (first run after installation)
        if (!certResult.valid && certResult.error === 'NO_CERTIFICATE') {
            console.log('[Main] No certificate found, generating new device certificate...');
            try {
                const fingerprint = certificateService.generateCertificate();
                console.log('[Main] Device certificate generated:', fingerprint);
                certResult = { valid: true, fingerprint: fingerprint };
            } catch (genError) {
                console.error('[Main] Failed to generate certificate:', genError);
                return { success: false, error: 'Failed to register device. Please run as Administrator.' };
            }
        }

        if (!certResult.valid) {
            console.error('[Main] Certificate verification failed:', certResult.error);
            let errorMessage = 'Device verification failed.';
            if (certResult.error === 'FINGERPRINT_MISMATCH') {
                errorMessage = 'Security Alert: Device hardware mismatch detected. Login denied.';
            } else {
                errorMessage = `Certificate error: ${certResult.error}`;
            }
            return { success: false, error: errorMessage };
        }
        console.log('[Main] Device authenticated:', certResult.fingerprint);

        // Create desktop session (required for Super Admin login)
        console.log('[Main] Creating desktop session...');
        console.log('[Main] Calling:', `${backendUrl}/api/system/desktop-session`);

        let sessionResult;
        try {
            sessionResult = await httpRequest(`${backendUrl}/api/system/desktop-session`, 'POST');
            console.log('[Main] Desktop session response status:', sessionResult.status);
            console.log('[Main] Desktop session response data:', JSON.stringify(sessionResult.data));
        } catch (sessionError) {
            console.error('[Main] Desktop session request failed:', sessionError);
            return {
                success: false,
                error: 'Cannot connect to backend server. Make sure it is running.'
            };
        }

        if (sessionResult.status !== 200 || !sessionResult.data.desktop_session) {
            console.error('[Main] Desktop session creation failed:', sessionResult.status, sessionResult.data);
            return {
                success: false,
                error: `Failed to create desktop session (status: ${sessionResult.status})`
            };
        }

        // Store the session token
        desktopSessionToken = sessionResult.data.desktop_session;

        // Login with credentials
        const loginResult = await httpRequest(`${backendUrl}/api/auth/login`, 'POST', {
            email: email,
            password: password,
            device_fingerprint: certResult.fingerprint
        });

        if (loginResult.status !== 200 || !loginResult.data.access_token) {
            // Login failed - clean up the desktop session
            await httpRequest(`${backendUrl}/api/system/desktop-session`, 'DELETE');
            desktopSessionToken = null;
            return {
                success: false,
                error: loginResult.data.error || 'Invalid credentials'
            };
        }

        // Check if user is super_admin
        if (loginResult.data.user.role !== 'super_admin') {
            // Not super admin - clean up the desktop session
            await httpRequest(`${backendUrl}/api/system/desktop-session`, 'DELETE');
            desktopSessionToken = null;
            return {
                success: false,
                error: 'Access denied. This launcher is only for Super Admin users.\n\nRegular users should access via browser at http://localhost:8080'
            };
        }

        // Store session info
        accessToken = loginResult.data.access_token;
        refreshToken = loginResult.data.refresh_token;
        currentUser = loginResult.data.user;
        isLoggedIn = true;

        return {
            success: true,
            user: currentUser,
            token: loginResult.data.access_token,
            desktopSession: desktopSessionToken,
            frontendUrl: frontendUrl
        };
    } catch (e) {
        // Clean up on error
        if (desktopSessionToken) {
            await httpRequest(`${backendUrl}/api/v1/desktop/session`, 'DELETE').catch(() => { });
            desktopSessionToken = null;
        }
        return {
            success: false,
            error: 'Cannot connect to server.\n\nPlease ensure:\n• Server is running\n• Port 8080 is accessible'
        };
    }
});

// Logout
ipcMain.handle('logout', async () => {
    await endDesktopSession();
    return { success: true };
});

// Open browser with Super Admin dashboard
ipcMain.handle('open-dashboard', async () => {
    if (!isLoggedIn || !desktopSessionToken || !accessToken) {
        return { success: false, error: 'Please login first' };
    }

    // Open browser with BOTH access_token AND desktop_session for auto-login
    const url = `${frontendUrl}/login?auto=1&token=${encodeURIComponent(accessToken)}&desktop_session=${encodeURIComponent(desktopSessionToken)}&refresh=${encodeURIComponent(refreshToken || '')}`;

    shell.openExternal(url);
    return { success: true };
});

// Get current status
ipcMain.handle('get-status', async () => {
    return {
        isLoggedIn: isLoggedIn,
        user: currentUser,
        hasDesktopSession: !!desktopSessionToken
    };
});

// Get connection info
ipcMain.handle('get-connection-info', async () => {
    return {
        backendUrl: backendUrl,
        frontendUrl: frontendUrl
    };
});

// Certificate Management
ipcMain.handle('check-certificate', async () => {
    return certificateService.verifyCertificate();
});

ipcMain.handle('generate-certificate', async () => {
    // Only allow generation if not exists (or maybe we force regeneration during install?)
    // For now, let's allow it but log it
    try {
        const fingerprint = certificateService.generateCertificate();
        return { success: true, fingerprint };
    } catch (e) {
        return { success: false, error: e.message };
    }
});
