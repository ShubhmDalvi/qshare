# QShare 🚀

QShare is a fast, secure, and minimal **command-line file transfer tool** for Windows (built with Go).  
It allows you to send and receive files over the local network with encryption, progress bars, and device discovery.

---

## ✨ Features
- 📡 Automatic device discovery (no IP typing needed)
- 📂 Send and receive single or multiple files
- 🔒 End-to-end encryption (AES-CTR)
- 📊 Progress bar with speed and ETA
- 💻 Works on LAN (Wi-Fi or Ethernet)
- 🛡️ Helper script to auto-add firewall rule

---

## ⚙️ Installation

### Option 1: Download Prebuilt Binary
1. Go to the [Releases](../../releases) section of this repository.
2. Download **`qshare.exe`** and **`AddQShareFirewallRule.bat`**.
3. Place them in the same folder.
4. (Optional) Run the `.bat` file once to allow QShare through Windows Firewall.

### Option 2: Build from Source
1. Install [Go](https://go.dev/dl/).
2. Clone this repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/qshare.git
   cd qshare
   ```
3. Build the binary:
   ```bash
   go build -o qshare.exe qshare.go
   ```

---

## 🚀 Usage

### Start QShare
Double-click `qshare.exe` → interactive menu will appear:

```
===================================
 QShare - Quick File Transfer 🚀
===================================
1. Send files/folders
2. Receive files
3. Exit
```

### Example: Send Files
```
1
Enter full paths (comma separated)
   Example: C:\Users\dell\file1.txt, D:\pics
```

### Example: Receive Files
```
2
📡 Ready to receive files...
📂 Files will be saved in: C:\Users\YOURNAME\Downloads\QShare
💻 Hostname: YOUR-PC-NAME
```

Type `exit` anytime to stop receiving.

---

## 🛠️ Firewall Rule
To avoid Windows Firewall blocking QShare, run the included script once:

```bat
AddQShareFirewallRule.bat
```

This adds a firewall rule for **all profiles (Private, Public, Domain)**.

---

## 🧑‍💻 Development Notes
- Language: **Go**
- Networking: UDP broadcast for discovery, TCP for file transfer
- Encryption: AES-CTR stream cipher
- Cross-platform potential (Linux/Mac), but optimized for Windows

---

## 📜 License
MIT License © 2025 YOUR NAME
