// Package parser parses output from TP-Link switch CLI commands.
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PoEPort describes power-over-ethernet details for a switch port.
type PoEPort struct {
	PowerWatts float64 `json:"power_watts"`
	CurrentMA  int     `json:"current_ma"`
	VoltageV   float64 `json:"voltage_v"`
	PDClass    string  `json:"pd_class"`
	Status     string  `json:"status"`
}

// ParsePoETable extracts a map of PoEPort entries from switch output.
func ParsePoETable(output string) (map[string]PoEPort, error) {
	lines := strings.Split(output, "\n")
	ports := make(map[string]PoEPort)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Tw") {
			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}
			iface := fields[0]

			power, err1 := strconv.ParseFloat(fields[1], 64)
			current, err2 := strconv.Atoi(fields[2])
			voltage, err3 := strconv.ParseFloat(fields[3], 64)
			if err1 != nil || err2 != nil || err3 != nil {
				return nil, fmt.Errorf("parse error on line: %q", line)
			}

			pdClass := strings.Join(fields[4:len(fields)-1], " ")
			status := fields[len(fields)-1]

			ports[iface] = PoEPort{
				PowerWatts: power,
				CurrentMA:  current,
				VoltageV:   voltage,
				PDClass:    pdClass,
				Status:     status,
			}
		}
	}

	return ports, nil
}

// InterfaceCounters represents a set of counters for a single port.
type InterfaceCounters map[string]uint64

// InterfaceStats holds counters per port.
type InterfaceStats map[string]InterfaceCounters

// ParseInterfaceCounters parses the "show interface counters" output into structured data.
func ParseInterfaceCounters(output string) (InterfaceStats, error) {
	lines := strings.Split(output, "\n")
	stats := make(InterfaceStats)
	var currentPort string

	keyValRegex := regexp.MustCompile(`^([\w\- /]+):\s+([\d,]+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Port:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentPort = strings.TrimSpace(parts[1])
				stats[currentPort] = make(InterfaceCounters)
			}
			continue
		}
		if currentPort == "" {
			continue
		}

		if matches := keyValRegex.FindStringSubmatch(line); len(matches) == 3 {
			key := strings.TrimSpace(matches[1])
			valStr := strings.ReplaceAll(matches[2], ",", "")
			val, err := strconv.ParseUint(valStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number for key %q: %v", key, err)
			}
			stats[currentPort][key] = val
		}
	}

	return stats, nil
}
