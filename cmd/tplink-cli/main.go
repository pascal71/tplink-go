// Command tplink-cli demonstrates usage of the TP-Link client and parser packages.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/pascal71/tplink-go/client"
	"github.com/pascal71/tplink-go/parser"
)

func main() {
	addr := os.Getenv("TPLINK_ADDR")
	user := os.Getenv("TPLINK_USER")
	pass := os.Getenv("TPLINK_PASS")
	if addr == "" || user == "" || pass == "" {
		log.Fatal("Please set TPLINK_ADDR, TPLINK_USER, and TPLINK_PASS environment variables")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c := client.NewClient(addr, user, pass)
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	defer c.Close()

	// Setup for command sequence
	commands := []string{"enable", "config", "no clipaging", "exit", "show power inline information interface"}
	var output string
	var err error

	for _, cmd := range commands {
		output, err = c.RunCommand(ctx, cmd)
		if err != nil {
			log.Fatalf("Command failed: %s: %v", cmd, err)
		}
	}

	ports, err := parser.ParsePoETable(output)
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ports); err != nil {
		log.Fatalf("Encoding JSON: %v", err)
	}
}
