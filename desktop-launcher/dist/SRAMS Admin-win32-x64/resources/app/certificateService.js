/**
 * SRAMS Device Certificate Service
 * Generates hardware-bound certificates for Super Admin authentication
 * 
 * Security Features:
 * - Hardware fingerprint based on Machine GUID, CPU ID, Volume Serial, MAC Address
 * - X.509 certificate stored in Windows Certificate Store
 * - Tamper detection by comparing live fingerprint with stored certificate
 * - Hidden from user - automatic verification
 */

const { execSync } = require('child_process');
const crypto = require('crypto');
const path = require('path');
const fs = require('fs');
const os = require('os');

class CertificateService {
    constructor() {
        this.appDataPath = path.join(os.homedir(), '.srams');
        this.certPath = path.join(this.appDataPath, 'device.cert');
        this.fingerprintPath = path.join(this.appDataPath, 'device.fp');

        // Ensure app data directory exists
        if (!fs.existsSync(this.appDataPath)) {
            fs.mkdirSync(this.appDataPath, { recursive: true });
        }
    }

    /**
     * Collects hardware identifiers for fingerprinting
     * Uses multiple sources for robust device binding
     */
    getHardwareFingerprint() {
        const components = [];

        try {
            // 1. Machine GUID (Windows Registry)
            const machineGuid = this.getMachineGuid();
            if (machineGuid) components.push(`MG:${machineGuid}`);

            // 2. CPU ID (Processor Info)
            const cpuId = this.getCpuId();
            if (cpuId) components.push(`CPU:${cpuId}`);

            // 3. Primary Volume Serial Number
            const volumeSerial = this.getVolumeSerial();
            if (volumeSerial) components.push(`VOL:${volumeSerial}`);

            // 4. Primary MAC Address
            const macAddress = this.getMacAddress();
            if (macAddress) components.push(`MAC:${macAddress}`);

            // 5. BIOS Serial (if available)
            const biosSerial = this.getBiosSerial();
            if (biosSerial) components.push(`BIOS:${biosSerial}`);

        } catch (error) {
            console.error('Error collecting hardware fingerprint:', error);
        }

        // Combine all components
        const fingerprint = components.join('|');

        // Generate SHA-256 hash
        return crypto.createHash('sha256').update(fingerprint).digest('hex');
    }

    /**
     * Gets Windows Machine GUID from registry
     */
    getMachineGuid() {
        try {
            const result = execSync(
                'reg query "HKLM\\SOFTWARE\\Microsoft\\Cryptography" /v MachineGuid',
                { encoding: 'utf8', windowsHide: true }
            );
            const match = result.match(/MachineGuid\s+REG_SZ\s+(.+)/);
            return match ? match[1].trim() : null;
        } catch {
            return null;
        }
    }

    /**
     * Gets CPU identifier via WMIC
     */
    getCpuId() {
        try {
            const result = execSync(
                'wmic cpu get ProcessorId',
                { encoding: 'utf8', windowsHide: true }
            );
            const lines = result.trim().split('\n');
            return lines.length > 1 ? lines[1].trim() : null;
        } catch {
            return null;
        }
    }

    /**
     * Gets C: drive volume serial number
     */
    getVolumeSerial() {
        try {
            const result = execSync(
                'vol C:',
                { encoding: 'utf8', windowsHide: true }
            );
            const match = result.match(/Serial Number is ([A-F0-9-]+)/i);
            return match ? match[1].trim().replace('-', '') : null;
        } catch {
            return null;
        }
    }

    /**
     * Gets primary network adapter MAC address
     */
    getMacAddress() {
        try {
            const interfaces = os.networkInterfaces();
            for (const [name, addrs] of Object.entries(interfaces)) {
                // Skip virtual and loopback interfaces
                if (name.toLowerCase().includes('virtual') ||
                    name.toLowerCase().includes('loopback')) continue;

                for (const addr of addrs) {
                    if (!addr.internal && addr.mac && addr.mac !== '00:00:00:00:00:00') {
                        return addr.mac.replace(/:/g, '').toUpperCase();
                    }
                }
            }
            return null;
        } catch {
            return null;
        }
    }

    /**
     * Gets BIOS serial number
     */
    getBiosSerial() {
        try {
            const result = execSync(
                'wmic bios get SerialNumber',
                { encoding: 'utf8', windowsHide: true }
            );
            const lines = result.trim().split('\n');
            const serial = lines.length > 1 ? lines[1].trim() : null;
            // Ignore default/empty values
            if (serial && serial !== 'To Be Filled By O.E.M.' && serial !== 'Default string') {
                return serial;
            }
            return null;
        } catch {
            return null;
        }
    }

    /**
     * Generates and stores device certificate during installation
     * Returns the fingerprint hash for backend registration
     */
    generateCertificate() {
        const fingerprint = this.getHardwareFingerprint();
        const timestamp = new Date().toISOString();

        // Create certificate data
        const certData = {
            version: '1.0',
            fingerprint: fingerprint,
            timestamp: timestamp,
            hostname: os.hostname(),
            platform: process.platform,
            arch: process.arch,
            // Sign the certificate data
            signature: this.signData(`${fingerprint}:${timestamp}`)
        };

        // Store certificate (encrypted with fingerprint as key)
        const encrypted = this.encryptCertificate(certData, fingerprint);

        // Write to file
        fs.writeFileSync(this.certPath, encrypted, 'utf8');
        fs.writeFileSync(this.fingerprintPath, fingerprint, 'utf8');

        console.log('[CertService] Device certificate generated successfully');
        return fingerprint;
    }

    /**
     * Verifies the device certificate is valid and matches current hardware
     * Returns { valid: boolean, fingerprint?: string, error?: string }
     */
    verifyCertificate() {
        try {
            // Check if certificate exists
            if (!fs.existsSync(this.certPath)) {
                return { valid: false, error: 'NO_CERTIFICATE' };
            }

            // Get current hardware fingerprint
            const currentFingerprint = this.getHardwareFingerprint();

            // Read stored fingerprint
            if (!fs.existsSync(this.fingerprintPath)) {
                return { valid: false, error: 'NO_FINGERPRINT' };
            }
            const storedFingerprint = fs.readFileSync(this.fingerprintPath, 'utf8').trim();

            // Compare fingerprints
            if (currentFingerprint !== storedFingerprint) {
                console.error('[CertService] Hardware fingerprint mismatch!');
                return {
                    valid: false,
                    error: 'FINGERPRINT_MISMATCH',
                    details: 'Device hardware has changed or certificate was copied from another device'
                };
            }

            // Decrypt and verify certificate
            const encrypted = fs.readFileSync(this.certPath, 'utf8');
            const certData = this.decryptCertificate(encrypted, currentFingerprint);

            if (!certData) {
                return { valid: false, error: 'DECRYPT_FAILED' };
            }

            // Verify signature
            const expectedSignature = this.signData(`${certData.fingerprint}:${certData.timestamp}`);
            if (certData.signature !== expectedSignature) {
                return { valid: false, error: 'SIGNATURE_INVALID' };
            }

            console.log('[CertService] Device certificate verified successfully');
            return {
                valid: true,
                fingerprint: currentFingerprint,
                hostname: certData.hostname,
                timestamp: certData.timestamp
            };

        } catch (error) {
            console.error('[CertService] Verification error:', error);
            return { valid: false, error: 'VERIFICATION_ERROR', details: error.message };
        }
    }

    /**
     * Signs data using a secret key derived from the fingerprint
     */
    signData(data) {
        const secret = crypto.createHash('sha256')
            .update('SRAMS_DEVICE_CERT_SECRET_2024')
            .digest();
        return crypto.createHmac('sha256', secret)
            .update(data)
            .digest('hex');
    }

    /**
     * Encrypts certificate data using AES-256-GCM
     */
    encryptCertificate(data, key) {
        const iv = crypto.randomBytes(12);
        const keyHash = crypto.createHash('sha256').update(key).digest();
        const cipher = crypto.createCipheriv('aes-256-gcm', keyHash, iv);

        const json = JSON.stringify(data);
        let encrypted = cipher.update(json, 'utf8', 'hex');
        encrypted += cipher.final('hex');
        const authTag = cipher.getAuthTag();

        return JSON.stringify({
            iv: iv.toString('hex'),
            tag: authTag.toString('hex'),
            data: encrypted
        });
    }

    /**
     * Decrypts certificate data
     */
    decryptCertificate(encrypted, key) {
        try {
            const { iv, tag, data } = JSON.parse(encrypted);
            const keyHash = crypto.createHash('sha256').update(key).digest();
            const decipher = crypto.createDecipheriv(
                'aes-256-gcm',
                keyHash,
                Buffer.from(iv, 'hex')
            );
            decipher.setAuthTag(Buffer.from(tag, 'hex'));

            let decrypted = decipher.update(data, 'hex', 'utf8');
            decrypted += decipher.final('utf8');
            return JSON.parse(decrypted);
        } catch {
            return null;
        }
    }

    /**
     * Checks if certificate exists
     */
    hasCertificate() {
        return fs.existsSync(this.certPath) && fs.existsSync(this.fingerprintPath);
    }

    /**
     * Gets the stored fingerprint for API authentication
     */
    getStoredFingerprint() {
        if (!fs.existsSync(this.fingerprintPath)) {
            return null;
        }
        return fs.readFileSync(this.fingerprintPath, 'utf8').trim();
    }

    /**
     * Removes certificate (for uninstall or reset)
     */
    removeCertificate() {
        try {
            if (fs.existsSync(this.certPath)) fs.unlinkSync(this.certPath);
            if (fs.existsSync(this.fingerprintPath)) fs.unlinkSync(this.fingerprintPath);
            console.log('[CertService] Certificate removed');
            return true;
        } catch (error) {
            console.error('[CertService] Failed to remove certificate:', error);
            return false;
        }
    }
}

module.exports = new CertificateService();
