package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetMachineID() (string, error) {
	out, err := exec.Command("systemd-id128", "machine-id", "-u").Output()
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve machine-id: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}
