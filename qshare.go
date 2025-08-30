package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BROADCAST_PORT = 50000
	TRANSFER_PORT  = 50001
	BUFFER_SIZE    = 8192
	SEPARATOR      = "<SEPARATOR>"
)

var devices = make(map[string]string)

// --- Utility ---
func getDeviceName() string {
	name, _ := os.Hostname()
	return name
}

// --- AES Encryption ---
func deriveKey(passphrase string) []byte {
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:]
}

func encryptStream(key []byte, w io.Writer) (cipher.StreamWriter, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return cipher.StreamWriter{}, err
	}
	iv := make([]byte, aes.BlockSize)
	return cipher.StreamWriter{S: cipher.NewCTR(block, iv), W: w}, nil
}

func decryptStream(key []byte, r io.Reader) (cipher.StreamReader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return cipher.StreamReader{}, err
	}
	iv := make([]byte, aes.BlockSize)
	return cipher.StreamReader{S: cipher.NewCTR(block, iv), R: r}, nil
}

// --- Discovery broadcaster ---
func broadcaster() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", BROADCAST_PORT))
	if err != nil {
		color.Red("❌ Broadcaster resolve error: %v", err)
		return
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		color.Red("❌ Broadcaster dial error: %v", err)
		return
	}
	defer conn.Close()

	name := getDeviceName()
	for {
		_, err := conn.Write([]byte(name))
		if err != nil {
			color.Red("❌ Broadcast write error: %v", err)
		}
		time.Sleep(2 * time.Second)
	}
}

// --- Discovery listener ---
func listener() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", BROADCAST_PORT))
	if err != nil {
		color.Red("❌ Listener resolve error: %v", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		color.Red("❌ Listener bind error: %v", err)
		return
	}
	defer conn.Close()

	for {
		buf := make([]byte, 1024)
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			color.Red("❌ Listener read error: %v", err)
			continue
		}
		devices[strings.TrimSpace(string(buf[:n]))] = raddr.IP.String()
	}
}

// --- File send with RAW multiple files ---
func sendFiles(paths []string, target string) {
	ip := devices[target]
	conn, err := net.Dial("tcp", ip+fmt.Sprintf(":%d", TRANSFER_PORT))
	if err != nil {
		color.Red("❌ Could not connect: %v", err)
		return
	}
	defer conn.Close()

	color.Cyan("🔗 Connected to %s", ip)

	key := deriveKey("qshare")
	encWriter, _ := encryptStream(key, conn)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			color.Red("❌ Could not access: %s", path)
			continue
		}
		filename := filepath.Base(path)
		filesize := info.Size()

		// send metadata
		meta := fmt.Sprintf("%s%s%d\n", filename, SEPARATOR, filesize)
		conn.Write([]byte(meta))

		// open file and stream
		file, _ := os.Open(path)
		defer file.Close()

		color.Blue("📤 Sending: %s (%.2f MB)", filename, float64(filesize)/(1024*1024))
		start := time.Now()
		bar := progressbar.NewOptions64(
			filesize,
			progressbar.OptionSetDescription("Uploading"),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(30),
			progressbar.OptionSetPredictTime(true),
		)

		var sent int64 = 0
		buf := make([]byte, BUFFER_SIZE)
		for {
			n, err := file.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				color.Red("❌ Error reading: %v", err)
				break
			}
			encWriter.Write(buf[:n])
			sent += int64(n)
			bar.Set64(sent)

			elapsed := time.Since(start).Seconds()
			speed := float64(sent) / 1024.0 / 1024.0 / elapsed
			eta := (float64(filesize-sent) / 1024.0 / 1024.0) / speed
			fmt.Printf("\r⚡ Speed: %.2f MB/s | ETA: %.1fs", speed, eta)
		}
		color.Green("\n✅ Sent: %s", filename)
	}
}

// --- Handle incoming connection (RAW multiple files) ---
func handleConnection(conn net.Conn, saveDir string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	key := deriveKey("qshare")
	decReader, _ := decryptStream(key, conn)

	for {
		// read metadata
		meta, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				color.Green("✅ All files received.")
			}
			return
		}
		parts := strings.Split(strings.TrimSpace(meta), SEPARATOR)
		if len(parts) < 2 {
			continue
		}
		filename := parts[0]
		var filesize int64
		fmt.Sscan(parts[1], &filesize)

		color.Blue("📥 Receiving: %s (%.2f MB)", filename, float64(filesize)/(1024*1024))

		savePath := filepath.Join(saveDir, filename)
		out, _ := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY, 0644)
		defer out.Close()

		start := time.Now()
		bar := progressbar.NewOptions64(
			filesize,
			progressbar.OptionSetDescription("Downloading"),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(30),
			progressbar.OptionSetPredictTime(true),
		)

		var written int64 = 0
		buf := make([]byte, BUFFER_SIZE)
		for written < filesize {
			n, err := decReader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				color.Red("❌ Error reading stream: %v", err)
				break
			}
			remaining := filesize - written
			if int64(n) > remaining {
				n = int(remaining)
			}
			out.Write(buf[:n])
			written += int64(n)
			bar.Set64(written)

			elapsed := time.Since(start).Seconds()
			speed := float64(written) / 1024.0 / 1024.0 / elapsed
			eta := (float64(filesize-written) / 1024.0 / 1024.0) / speed
			fmt.Printf("\r⚡ Speed: %.2f MB/s | ETA: %.1fs", speed, eta)
		}
		color.Green("\n✅ Saved: %s", savePath)
	}
}

// --- File receive loop ---
func receiveFile(saveDir string, stopChan chan bool) {
	if saveDir == "" {
		home, _ := os.UserHomeDir()
		saveDir = filepath.Join(home, "Downloads", "QShare")
	}
	os.MkdirAll(saveDir, os.ModePerm)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", TRANSFER_PORT))
	if err != nil {
		color.Red("❌ Could not start TCP listener: %v", err)
		return
	}
	defer ln.Close()

	connChan := make(chan net.Conn)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				color.Red("❌ Accept error: %v", err)
				return
			}
			connChan <- conn
		}
	}()

	for {
		select {
		case <-stopChan:
			color.Green("👋 Stopping receiver, returning to menu...")
			return
		case conn := <-connChan:
			handleConnection(conn, saveDir)
		}
	}
}

// --- Interactive Menu ---
func interactiveMenu() {
	reader := bufio.NewReader(os.Stdin)
	for {
		color.Cyan("===================================")
		color.Cyan(" QShare - Quick File Transfer 🚀")
		color.Cyan("===================================")
		color.Yellow("1.") ; fmt.Println(" Send files/folders")
		color.Yellow("2.") ; fmt.Println(" Receive files")
		color.Yellow("3.") ; fmt.Println(" Exit")
		color.Cyan("Select option: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			color.Cyan("📤 Send Mode")
			color.Yellow("Enter full paths (comma separated)")
			color.Yellow("   Example: C:\\Users\\dell\\file1.txt, D:\\pics")
			color.Cyan("Or type 'back' to return: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "" || strings.ToLower(line) == "back" {
				color.Yellow("↩️ Returning to menu...")
				continue
			}

			paths := strings.Split(line, ",")
			for i := range paths {
				paths[i] = strings.TrimSpace(paths[i])
			}

			time.Sleep(3 * time.Second)
			if len(devices) == 0 {
				color.Red("❌ No devices found.")
				continue
			}

			var target string
			if len(devices) == 1 {
				for name := range devices {
					target = name
				}
			} else {
				color.Cyan("🔍 Available devices:")
				i := 1
				names := []string{}
				for name := range devices {
					color.Yellow("%d.", i) ; fmt.Printf(" %s\n", name)
					names = append(names, name)
					i++
				}
				color.Cyan("Select target: ")
				var num int
				fmt.Scanln(&num)
				target = names[num-1]
			}
			sendFiles(paths, target)

		case "2":
			color.Cyan("📥 Receive Mode")
			stopChan := make(chan bool)
			home, _ := os.UserHomeDir()
			defaultSave := filepath.Join(home, "Downloads", "QShare")
			hostname, _ := os.Hostname()
			color.Green("💻 Hostname: %s", hostname)

			color.Blue("📡 Ready to receive files...")
			color.Cyan("📂 Files will be saved in: %s", defaultSave)
			color.Yellow("👉 Type 'exit' and press ENTER to stop receiving and return to menu")
			

			go receiveFile("", stopChan)

			for {
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input == "exit" {
					stopChan <- true
					break
				}
			}

		case "3":
			color.Green("👋 Exiting QShare. Goodbye!")
			return
		default:
			color.Red("❌ Invalid choice, try again.")
		}
	}
}

// --- Main ---
func main() {
	go broadcaster()
	go listener()

	interactiveMenu()
}
