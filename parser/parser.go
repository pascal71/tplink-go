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

// CPUUtilization holds CPU load for different intervals.
type CPUUtilization struct {
	FiveSeconds int
	OneMinute   int
	FiveMinutes int
}

// ParseCPUUtilization parses the "show cpu-utilization" output.
func ParseCPUUtilization(output string) (CPUUtilization, error) {
	var result CPUUtilization
	re := regexp.MustCompile(`(?m)\|\s+(\d+)%\s+(\d+)%\s+(\d+)%`) // matches the row with percentages
	matches := re.FindStringSubmatch(output)
	if len(matches) == 4 {
		var err error
		result.FiveSeconds, err = strconv.Atoi(matches[1])
		if err != nil {
			return result, err
		}
		result.OneMinute, err = strconv.Atoi(matches[2])
		if err != nil {
			return result, err
		}
		result.FiveMinutes, err = strconv.Atoi(matches[3])
		if err != nil {
			return result, err
		}
		return result, nil
	}
	return result, fmt.Errorf("unable to parse cpu utilization")
}

// MemoryUtilization holds the memory usage percentage.
type MemoryUtilization struct {
	Unit  int
	Usage int
}

// ParseMemoryUtilization parses the "show memory-utilization" output.
func ParseMemoryUtilization(output string) (MemoryUtilization, error) {
	re := regexp.MustCompile(`(?m)^(\d+)\s+\|\s+(\d+)%`)
	matches := re.FindStringSubmatch(output)
	if len(matches) == 3 {
		unit, err := strconv.Atoi(matches[1])
		if err != nil {
			return MemoryUtilization{}, err
		}
		usage, err := strconv.Atoi(matches[2])
		if err != nil {
			return MemoryUtilization{}, err
		}
		return MemoryUtilization{Unit: unit, Usage: usage}, nil
	}
	return MemoryUtilization{}, fmt.Errorf("unable to parse memory utilization")
}

// InterfaceStatus represents a single line in the "show interface status" output.
type InterfaceStatus struct {
	Port         string
	Status       string
	Speed        string
	Duplex       string
	FlowCtrl     string
	ActiveMedium string
	Description  string
}

// ParseInterfaceStatus parses the "show interface status" output.
func ParseInterfaceStatus(output string) ([]InterfaceStatus, error) {
	var results []InterfaceStatus
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Port") || strings.HasPrefix(line, "----") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			desc := strings.Join(fields[6:], " ")
			results = append(results, InterfaceStatus{
				Port:         fields[0],
				Status:       fields[1],
				Speed:        fields[2],
				Duplex:       fields[3],
				FlowCtrl:     fields[4],
				ActiveMedium: fields[5],
				Description:  desc,
			})
		}
	}
	return results, nil
}

// MACEntry represents a MAC address table entry.
type MACEntry struct {
	MAC   string
	VLAN  int
	Port  string
	Type  string
	Aging string
}

// ParseMACTable parses the "show mac address-table" output.
func ParseMACTable(output string) ([]MACEntry, error) {
	var results []MACEntry
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MAC") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Total") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 5 {
			vlan, err := strconv.Atoi(fields[1])
			if err != nil {
				continue
			}
			results = append(results, MACEntry{
				MAC:   fields[0],
				VLAN:  vlan,
				Port:  fields[2],
				Type:  fields[3],
				Aging: fields[4],
			})
		}
	}
	return results, nil
}

// InterfaceConfig represents configuration details per port.
type InterfaceConfig struct {
	Port        string
	State       string
	Speed       string
	Duplex      string
	FlowCtrl    string
	Description string
}

// ParseInterfaceConfig parses the "show interface configuration" output.
func ParseInterfaceConfig(output string) ([]InterfaceConfig, error) {
	var results []InterfaceConfig
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Port") || strings.HasPrefix(line, "----") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			desc := strings.Join(fields[5:], " ")
			results = append(results, InterfaceConfig{
				Port:        fields[0],
				State:       fields[1],
				Speed:       fields[2],
				Duplex:      fields[3],
				FlowCtrl:    fields[4],
				Description: desc,
			})
		}
	}
	return results, nil
}
