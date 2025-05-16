// Package client provides an SSH interface to communicate with TP-Link switches.
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	promptRegex = regexp.MustCompile(`(?m)[\r\n]*(SG2210XMP-M2(-N\d+)?(\([^)]*\))?[>#])\s*$`)
	ansiEscape  = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

// Client provides an SSH session to interact with TP-Link switches.
type Client struct {
	Addr     string       // Address of the switch (host:port)
	User     string       // SSH username
	Password string       // SSH password
	conn     *ssh.Client  // Underlying SSH connection
	session  *ssh.Session // SSH session with PTY
	stdin    io.Writer    // Pipe to session stdin
	stdout   io.Reader    // Pipe from session stdout
	outBuf   *bytes.Buffer
}

// NewClient returns a new initialized Client instance.
func NewClient(addr, user, password string) *Client {
	return &Client{
		Addr:     addr,
		User:     user,
		Password: password,
		outBuf:   new(bytes.Buffer),
	}
}

// Connect establishes the SSH connection and interactive shell session.
func (c *Client) Connect(ctx context.Context) error {
	cfg := &ssh.ClientConfig{
		User:            c.User,
		Auth:            []ssh.AuthMethod{ssh.Password(c.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	conn, err := ssh.Dial("tcp", c.Addr, cfg)
	if err != nil {
		return fmt.Errorf("SSH dial failed: %w", err)
	}
	c.conn = conn

	sess, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session failed: %w", err)
	}
	c.session = sess

	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	c.stdin = stdin
	c.stdout = stdout

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm", 120, 40, modes); err != nil {
		return err
	}
	if err := sess.Shell(); err != nil {
		return err
	}

	return c.waitForPrompt(ctx)
}

// RunCommand sends a command to the switch and returns its output.
func (c *Client) RunCommand(ctx context.Context, cmd string) (string, error) {
	fmt.Fprint(c.stdin, cmd+"\r\n")
	if err := c.waitForPrompt(ctx); err != nil {
		return "", err
	}
	out := ansiEscape.ReplaceAllString(c.outBuf.String(), "")
	out = strings.ReplaceAll(out, "\r", "")
	c.outBuf.Reset()
	return out, nil
}

// Close terminates the SSH session and connection.
func (c *Client) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// waitForPrompt waits for the switch CLI prompt after sending a command.
func (c *Client) waitForPrompt(ctx context.Context) error {
	buf := make([]byte, 4096)
	tmp := make([]byte, 0)
	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for prompt")
		default:
			n, err := c.stdout.Read(buf)
			if err != nil && err != io.EOF {
				return err
			}
			if n > 0 {
				chunk := buf[:n]
				tmp = append(tmp, chunk...)
				c.outBuf.Write(chunk)
				cleaned := ansiEscape.ReplaceAll(tmp, []byte(""))
				if promptRegex.Match(cleaned) {
					return nil
				}
			}
		}
	}
}
