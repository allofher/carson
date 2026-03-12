package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Manager handles OS-level service lifecycle.
type Manager interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (Status, error)
}

// Status describes the daemon's current state.
type Status struct {
	Running bool
	PID     int
}

// PIDPath returns the path to the PID file.
func PIDPath(configDir string) string {
	return filepath.Join(configDir, "carson.pid")
}

// WritePID writes the current process PID to the PID file.
func WritePID(configDir string) error {
	return os.WriteFile(PIDPath(configDir), []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads the PID from the PID file. Returns 0 if not found.
func ReadPID(configDir string) (int, error) {
	data, err := os.ReadFile(PIDPath(configDir))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}
	return pid, nil
}

// RemovePID deletes the PID file.
func RemovePID(configDir string) {
	os.Remove(PIDPath(configDir))
}

// IsRunning checks if a process with the given PID is alive.
func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Signal 0 checks if process exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// StopProcess sends SIGTERM to the given PID.
func StopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}
	return process.Signal(syscall.SIGTERM)
}
