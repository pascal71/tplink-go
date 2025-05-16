package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	promptRegex = regexp.MustCompile(`(?m)[\r\n]*(SG2210XMP-M2(-N\d+)?(\([^)]+\))?[>#])\s*$`)
	ansiEscape  = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

type PoEPort struct {
	PowerWatts float64 `json:"power_watts"`
	CurrentMA  int     `json:"current_ma"`
	VoltageV   float64 `json:"voltage_v"`
	PDClass    string  `json:"pd_class"`
	Status     string  `json:"status"`
}

func waitForPrompt(stdin io.Writer, stdout io.Reader, password string, fullOutput *bytes.Buffer) {
	ctx := context.Background()
	buffer := make([]byte, 4096)
	tmp := make([]byte, 0)
	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-timeout:
			slog.WarnContext(ctx, "Prompt wait timed out")
			partial := ansiEscape.ReplaceAllString(string(tmp), "")
			partial = strings.ReplaceAll(partial, "\r", "")
			slog.WarnContext(ctx, "Partial data received before timeout", "output", strings.TrimSpace(partial))
			return
		default:
			n, err := stdout.Read(buffer)
			if err != nil && err != io.EOF {
				slog.ErrorContext(ctx, "Read error", "error", err)
				return
			}
			if n > 0 {
				raw := buffer[:n]
				slog.DebugContext(ctx, "Received raw", "raw", string(raw))
				fullOutput.Write(raw)
				tmp = append(tmp, raw...)

				cleaned := ansiEscape.ReplaceAll(tmp, []byte(""))
				slog.DebugContext(ctx, "Cleaned so far", "text", string(cleaned))

				if bytes.Contains(cleaned, []byte("Password:")) {
					slog.InfoContext(ctx, "Enable password prompt detected, sending password")
					fmt.Fprintln(stdin, password)
					tmp = []byte{}
					continue
				}
				if promptRegex.Match(cleaned) {
					slog.DebugContext(ctx, "Prompt detected")
					return
				}
			}
		}
	}
}

func main() {
	switchIP := "10.8.62.221:22"
	username := "admin"
	password := "Niilas12"

	logFile, err := os.OpenFile("tplink_cli.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	ctx := context.Background()

	slog.InfoContext(ctx, "Connecting to switch", "host", switchIP)

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	conn, err := ssh.Dial("tcp", switchIP, config)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to connect", "error", err)
		return
	}
	defer conn.Close()
	slog.InfoContext(ctx, "SSH connection established")

	session, err := conn.NewSession()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create SSH session", "error", err)
		return
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get stdin", "error", err)
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get stdout", "error", err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", 120, 40, modes); err != nil {
		slog.ErrorContext(ctx, "PTY request failed", "error", err)
		return
	}

	if err := session.Shell(); err != nil {
		slog.ErrorContext(ctx, "Failed to start shell", "error", err)
		return
	}
	slog.DebugContext(ctx, "Shell started")

	var fullOutput bytes.Buffer
	waitForPrompt(stdin, stdout, password, &fullOutput)

	send := func(cmd string) {
		slog.Info("Sending command", "command", cmd)
		fmt.Fprint(stdin, cmd+"\r\n")
		waitForPrompt(stdin, stdout, password, &fullOutput)
	}

	send("enable")
	send("config")
	send("no clipaging")
	send("exit")
	send("show power inline information interface")
	send("exit")
	send("exit")

	cleaned := ansiEscape.ReplaceAllString(fullOutput.String(), "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	slog.InfoContext(ctx, "SSH session complete")

	fmt.Println("=== TP-Link Switch Output ===")
	fmt.Println(strings.TrimSpace(cleaned))

	lines := strings.Split(cleaned, "\n")
	poeTable := make(map[string]PoEPort)

	for _, line := range lines {
		if strings.HasPrefix(line, "Tw") {
			fields := strings.Fields(line)

			if len(fields) < 6 {
				continue
			}

			iface := fields[0]
			var power float64
			var current int
			var voltage float64

			fmt.Sscanf(fields[1], "%f", &power)
			fmt.Sscanf(fields[2], "%d", &current)
			fmt.Sscanf(fields[3], "%f", &voltage)

			pdClassFields := fields[4 : len(fields)-1]
			status := fields[len(fields)-1]
			pdClass := strings.Join(pdClassFields, " ")

			poeTable[iface] = PoEPort{
				PowerWatts: power,
				CurrentMA:  current,
				VoltageV:   voltage,
				PDClass:    pdClass,
				Status:     status,
			}
		}
	}

	fmt.Println("\n=== Parsed JSON Output ===")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(poeTable); err != nil {
		log.Fatalf("Failed to encode JSON: %v", err)
	}
}
