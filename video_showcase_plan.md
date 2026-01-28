# 🎬 SRAMS Project Showcase Video Plan

## 🎯 Video Objective
To demonstrate **SRAMS** not just as a document viewer, but as a **military-grade security platform**. The video should contrast the *ease of use* for normal users with the *strict security* for admins.

---

## ⏱️ Video Structure (Estimated Duration: 2:30)

### **Segment 1: The Problem & The Solution (0:00 - 0:30)**
*   **Visual:** Fast montage of "Access Denied" screens or generic insecure file folders.
*   **Voiceover/Text:** "Enterprise document leakage is usually an inside job. Standard storage isn't enough."
*   **Hero Shot:** The **SRAMS Desktop Launcher** opening up.
*   **Feature to Highlight:** **"Hardware-Gated Administration"**.
    *   *Action:* Try to access an Admin API endpoint (like `/api/v1/system/config`) via a standard Browser or Postman. Show it failing (`403 Forbidden`).
    *   *Action:* Access the *same* endpoint via the SRAMS Desktop Launcher. Show it succeeding.
    *   *Takeaway:* "Admins cannot be phished via the web. The keys simply don't exist in the browser."

### **Segment 2: Zero-Trust Document Viewing (0:30 - 1:10)**
*   **Visual:** Transition to the **Web Interface** (for standard users).
*   **Action:** User logs in and opens a sensitive PDF.
*   **Feature to Highlight:** **"Dynamic, Unavoidable Watermarking"**.
    *   *Demo:* Zoom in on the watermark. Point out the **User's Name**, **Current IP**, and **Precise Timestamp**.
    *   *Demo:* Show the **"Multiply" blend mode** in action—the watermark sits *behind* the text (thanks to your recent fix), ensuring readability while maintaining security.
    *   *Demo:* Try to **Right-Click**, **Select Text**, or **Print (Ctrl+P)**. Show that *nothing happens*. It is "Read-Only" in the truest sense.

### **Segment 3: The Admin Experience (1:10 - 1:50)**
*   **Visual:** Split screen or fast switch back to the **Desktop Launcher (Admin)**.
*   **Feature to Highlight:** **"Real-Time Security Control"**.
    *   *Action:* Go to **Settings > Company Logo**. Upload a new logo.
    *   *Action:* Adjust the **Opacity Slider** from 20% to 50%. Click Apply.
    *   *Action:* Switch immediately back to the User's view and refresh. *Boom*—the watermark has changed instantly for everyone.
*   **Feature to Highlight:** **"Forensic Audit Trails"**.
    *   *Action:* Go to the **Audit Logs** page.
    *   *Focus:* Highlight a specific log entry: `"User John Doe viewed Document X (Page 4)"`.
    *   *Takeaway:* "We don't just track who opened the file. We track what they read."

### **Segment 4: Future Roadmap (1:50 - 2:15)**
*   **Visual:** Sleek, dark-mode motion graphics or high-level architecture diagrams.
*   **Voiceover/Text:** "The foundation is built. The future is intelligent."
*   **Upcoming Feature 1:** **"AI-Driven Anomaly Detection (UBA)"**.
    *   *Concept:* "If John Doe usually reads 5 pages a day but suddenly scrolls through 500 pages in 2 minutes, the AI locks the account instantly."
*   **Upcoming Feature 2:** **"Blockchain Immutability"**.
    *   *Concept:* "Audit logs hashed to a private blockchain (Hyperledger), making forensic data legally admissible and tamper-proof."

### **Segment 5: Outro (2:15 - 2:30)**
*   **Visual:** SRAMS Logo pulsing.
*   **Text:** "Secure. Traceable. Absolute."
*   **Call to Action:** Link to your portfolio / GitHub.

---

## 💡 Pro Tips for the Recording
1.  **The "Split-Personality" Demo:** Since you are demonstrating both Admin and User roles, use **two different themes** or browser profiles. Keep the Admin Desktop App in **Dark Mode** and the User Web Interface in **Light Mode** to make it instantly clear to the viewer which role you are playing.
2.  **Zoom cuts:** When showing the watermark details (IP/Date), zoom in the video editor. Don't make the viewer squint.
3.  **Mouse Movement:** Move the mouse deliberately and smoothly. Jerky mouse movements look unprofessional.
