// Renderer process - UI logic
let isServerRunning = false;
let checkInterval = null;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    await checkServerStatus();
    await updateUI();

    // Check server status periodically
    checkInterval = setInterval(checkServerStatus, 5000);

    // Update connection info
    updateConnectionInfo();

    // Handle Enter key in login form
    document.getElementById('password').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') login();
    });
    document.getElementById('email').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') document.getElementById('password').focus();
    });
});

// Update connection info in footer
async function updateConnectionInfo() {
    try {
        const info = await window.electronAPI.getConnectionInfo();
        document.getElementById('connectionInfo').textContent =
            `Backend: ${info.backendUrl} | Frontend: ${info.frontendUrl}`;
    } catch (e) {
        document.getElementById('connectionInfo').textContent = 'Backend: localhost:8080';
    }
}

// Check server status
async function checkServerStatus() {
    const statusEl = document.getElementById('serverStatus');
    const dot = statusEl.querySelector('.status-dot');
    const text = statusEl.querySelector('.status-text');

    try {
        const result = await window.electronAPI.checkServer();

        if (result.running) {
            isServerRunning = true;
            dot.className = 'status-dot online';
            text.textContent = 'Server Running';
            document.getElementById('startServerBtn').disabled = true;

            // Stop button only enabled if logged in
            const status = await window.electronAPI.getStatus();
            document.getElementById('stopServerBtn').disabled = !status.isLoggedIn;
        } else {
            isServerRunning = false;
            dot.className = 'status-dot offline';

            // Show detailed status message
            let statusMessage = 'Server Offline';
            if (result.message) {
                statusMessage = result.message;
            } else if (result.postgresql && !result.postgresql.running) {
                statusMessage = 'PostgreSQL Not Running';
            } else if (result.backend && !result.backend.healthy) {
                statusMessage = 'Backend Not Responding';
            }

            text.textContent = statusMessage;
            document.getElementById('startServerBtn').disabled = false;
            document.getElementById('stopServerBtn').disabled = true;
        }
    } catch (e) {
        isServerRunning = false;
        dot.className = 'status-dot offline';
        text.textContent = 'Server Offline';
        document.getElementById('startServerBtn').disabled = false;
        document.getElementById('stopServerBtn').disabled = true;
    }
}

// Update UI based on login state
async function updateUI() {
    try {
        const status = await window.electronAPI.getStatus();

        if (status.isLoggedIn && status.user) {
            // Show logged in section
            document.getElementById('loginSection').style.display = 'none';
            document.getElementById('loggedInSection').style.display = 'block';

            // Update user info
            document.getElementById('userName').textContent = status.user.full_name || status.user.email;
            document.getElementById('userAvatar').textContent =
                (status.user.full_name || status.user.email).charAt(0).toUpperCase();

            // Enable stop button
            document.getElementById('stopServerBtn').disabled = !isServerRunning;
        } else {
            // Show login section
            document.getElementById('loginSection').style.display = 'block';
            document.getElementById('loggedInSection').style.display = 'none';

            // Disable stop button
            document.getElementById('stopServerBtn').disabled = true;
        }
    } catch (e) {
        console.error('Error updating UI:', e);
    }
}

// Start server
async function startServer() {
    const btn = document.getElementById('startServerBtn');
    const statusText = document.querySelector('.status-text');
    const statusDot = document.querySelector('.status-dot');

    btn.disabled = true;
    btn.classList.add('loading');
    statusDot.className = 'status-dot starting';

    // Show step-by-step progress
    statusText.textContent = 'Starting PostgreSQL...';

    try {
        const result = await window.electronAPI.startServer();

        if (result.success) {
            statusText.textContent = 'Server Started!';
            // Wait a moment then check status
            setTimeout(async () => {
                await checkServerStatus();
                btn.classList.remove('loading');
            }, 2000);
        } else {
            showError(result.error || 'Failed to start server');
            btn.disabled = false;
            btn.classList.remove('loading');
            statusDot.className = 'status-dot offline';
            statusText.textContent = result.error || 'Failed to Start';
        }
    } catch (e) {
        showError('Error starting server: ' + e.message);
        btn.disabled = false;
        btn.classList.remove('loading');
        statusDot.className = 'status-dot offline';
        statusText.textContent = 'Error: ' + e.message;
    }
}

// Run setup manually (when installer failed)
async function runSetup() {
    const btn = document.getElementById('runSetupBtn');
    const statusText = document.querySelector('.status-text');
    const statusDot = document.querySelector('.status-dot');

    if (!confirm('This will run the PostgreSQL setup with Administrator privileges.\n\nContinue?')) {
        return;
    }

    btn.disabled = true;
    btn.classList.add('loading');
    statusDot.className = 'status-dot starting';
    statusText.textContent = 'Running Setup...';

    try {
        const result = await window.electronAPI.runSetup();

        if (result.success) {
            statusText.textContent = 'Setup Complete!';
            showSuccess('Setup completed successfully. Click Start Server to continue.');
            await checkServerStatus();
        } else {
            showError(result.error || 'Setup failed');
            statusText.textContent = result.error || 'Setup Failed';
        }
    } catch (e) {
        showError('Error running setup: ' + e.message);
        statusText.textContent = 'Setup Error';
    } finally {
        btn.disabled = false;
        btn.classList.remove('loading');
    }
}

// Stop server
async function stopServer() {
    const btn = document.getElementById('stopServerBtn');

    if (!confirm('Are you sure you want to stop the server?\n\nThis will disconnect all users.')) {
        return;
    }

    btn.disabled = true;
    btn.classList.add('loading');

    try {
        const result = await window.electronAPI.stopServer();

        if (result.success) {
            setTimeout(async () => {
                await checkServerStatus();
                btn.classList.remove('loading');
            }, 2000);
        } else {
            showError(result.error || 'Failed to stop server');
            btn.disabled = false;
            btn.classList.remove('loading');
        }
    } catch (e) {
        showError('Error stopping server: ' + e.message);
        btn.disabled = false;
        btn.classList.remove('loading');
    }
}

// Login
async function login() {
    const email = document.getElementById('email').value.trim();
    const password = document.getElementById('password').value;
    const btn = document.getElementById('loginBtn');
    const errorEl = document.getElementById('loginError');

    // Validation
    if (!email || !password) {
        showError('Please enter email and password');
        return;
    }

    // Check if server is running
    if (!isServerRunning) {
        showError('Server is not running. Please start the server first.');
        return;
    }

    btn.disabled = true;
    btn.classList.add('loading');
    hideError();

    try {
        const result = await window.electronAPI.login(email, password);

        if (result.success) {
            // Store tokens for browser
            localStorage.setItem('access_token', result.token);
            localStorage.setItem('desktop_session', result.desktopSession);

            await updateUI();

            // Clear form
            document.getElementById('email').value = '';
            document.getElementById('password').value = '';
        } else {
            showError(result.error || 'Login failed');
        }
    } catch (e) {
        showError('Connection error: ' + e.message);
    } finally {
        btn.disabled = false;
        btn.classList.remove('loading');
    }
}

// Logout
async function logout() {
    try {
        await window.electronAPI.logout();
        localStorage.removeItem('access_token');
        localStorage.removeItem('desktop_session');
        await updateUI();
    } catch (e) {
        showError('Logout error: ' + e.message);
    }
}

// Open dashboard in browser
async function openDashboard() {
    try {
        const result = await window.electronAPI.openDashboard();
        if (!result.success) {
            showError(result.error || 'Failed to open dashboard');
        }
    } catch (e) {
        showError('Error opening dashboard: ' + e.message);
    }
}

// Show error message
function showError(message) {
    const errorEl = document.getElementById('loginError');
    errorEl.textContent = message;
    errorEl.classList.add('show');
}

// Hide error message
function hideError() {
    const errorEl = document.getElementById('loginError');
    errorEl.classList.remove('show');
}

// Show success message
function showSuccess(message) {
    const errorEl = document.getElementById('loginError');
    errorEl.textContent = message;
    errorEl.style.color = '#4CAF50';
    errorEl.classList.add('show');
    setTimeout(() => {
        errorEl.style.color = '';
        hideError();
    }, 5000);
}
