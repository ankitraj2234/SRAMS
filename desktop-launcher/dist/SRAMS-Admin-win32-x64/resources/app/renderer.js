// DOM Elements
const form = document.getElementById('login-form');
const emailInput = document.getElementById('email');
const passwordInput = document.getElementById('password');
const loginBtn = document.getElementById('login-btn');
const btnText = document.getElementById('btn-text');
const btnLoading = document.getElementById('btn-loading');
const errorMessage = document.getElementById('error-message');
const statusIndicator = document.getElementById('status-indicator');
const statusText = document.getElementById('status-text');

// Check backend status on load
async function checkBackendStatus() {
    try {
        const isOnline = await window.electronAPI.checkBackend();

        if (isOnline) {
            statusIndicator.className = 'indicator online';
            statusText.textContent = 'Server connected';
            loginBtn.disabled = false;
        } else {
            statusIndicator.className = 'indicator offline';
            statusText.textContent = 'Server offline - start SRAMS service';
            loginBtn.disabled = true;
        }
    } catch (error) {
        statusIndicator.className = 'indicator offline';
        statusText.textContent = 'Connection error';
        loginBtn.disabled = true;
    }
}

// Initial check
checkBackendStatus();

// Recheck every 5 seconds
setInterval(checkBackendStatus, 5000);

// Show error
function showError(message) {
    errorMessage.textContent = message;
    errorMessage.classList.add('show');
}

// Hide error
function hideError() {
    errorMessage.classList.remove('show');
}

// Set loading state
function setLoading(loading) {
    if (loading) {
        btnText.style.display = 'none';
        btnLoading.style.display = 'block';
        loginBtn.disabled = true;
        emailInput.disabled = true;
        passwordInput.disabled = true;
    } else {
        btnText.style.display = 'block';
        btnLoading.style.display = 'none';
        loginBtn.disabled = false;
        emailInput.disabled = false;
        passwordInput.disabled = false;
    }
}

// Handle form submission
form.addEventListener('submit', async (e) => {
    e.preventDefault();

    const email = emailInput.value.trim();
    const password = passwordInput.value;

    if (!email || !password) {
        showError('Please enter email and password');
        return;
    }

    hideError();
    setLoading(true);

    try {
        const result = await window.electronAPI.login(email, password);

        if (result.success) {
            // Successfully logged in as super_admin
            statusText.textContent = 'Opening dashboard...';
            await window.electronAPI.openDashboard(result.token);
        } else {
            showError(result.error);
            setLoading(false);
        }
    } catch (error) {
        showError('Login failed: ' + error.message);
        setLoading(false);
    }
});

// Auto-focus email field
emailInput.focus();
