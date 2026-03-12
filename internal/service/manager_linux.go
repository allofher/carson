//go:build linux

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const unitTmpl = `[Unit]
Description=Carson Agent Daemon

[Service]
Type=simple
ExecStart={{.Executable}} start --foreground
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

type unitData struct {
	Executable string
}

type systemdManager struct {
	unitPath   string
	configDir  string
	executable string
}

// NewManager creates a Linux systemd service manager.
func NewManager(configDir string) (Manager, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolving executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("resolving executable symlinks: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}

	return &systemdManager{
		unitPath:   filepath.Join(home, ".config", "systemd", "user", "carson.service"),
		configDir:  configDir,
		executable: exe,
	}, nil
}

func (m *systemdManager) Install() error {
	tmpl, err := template.New("unit").Parse(unitTmpl)
	if err != nil {
		return fmt.Errorf("parsing unit template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, unitData{
		Executable: m.executable,
	}); err != nil {
		return fmt.Errorf("rendering unit file: %w", err)
	}

	// Ensure systemd user directory exists.
	if err := os.MkdirAll(filepath.Dir(m.unitPath), 0755); err != nil {
		return fmt.Errorf("creating systemd user directory: %w", err)
	}

	if err := os.WriteFile(m.unitPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	// Reload systemd to pick up the new/changed unit file.
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}

	return nil
}

func (m *systemdManager) Uninstall() error {
	// Stop and disable if running.
	_ = exec.Command("systemctl", "--user", "stop", "carson").Run()
	_ = exec.Command("systemctl", "--user", "disable", "carson").Run()

	// Remove the unit file.
	if err := os.Remove(m.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	// Reload systemd.
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	// Clean up PID file.
	RemovePID(m.configDir)
	return nil
}

func (m *systemdManager) Start() error {
	if err := exec.Command("systemctl", "--user", "start", "carson").Run(); err != nil {
		return fmt.Errorf("systemctl start: %w", err)
	}
	return nil
}

func (m *systemdManager) Stop() error {
	if err := exec.Command("systemctl", "--user", "stop", "carson").Run(); err != nil {
		return fmt.Errorf("systemctl stop: %w", err)
	}
	return nil
}

func (m *systemdManager) Status() (Status, error) {
	pid, err := ReadPID(m.configDir)
	if err != nil {
		return Status{}, err
	}
	if pid == 0 {
		return Status{}, nil
	}
	return Status{
		Running: IsRunning(pid),
		PID:     pid,
	}, nil
}
