// Package parser parses output from TP-Link switch CLI commands.
package parser

import (
	"fmt"
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
