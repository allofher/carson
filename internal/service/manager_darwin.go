//go:build darwin

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const (
	launchdLabel = "com.carson.agent"
	plistTmpl    = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.Executable}}</string>
        <string>start</string>
        <string>--foreground</string>
    </array>
    <key>RunAtLoad</key>
    <false/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>/dev/null</string>
    <key>StandardErrorPath</key>
    <string>/dev/null</string>
</dict>
</plist>
`
)

type plistData struct {
	Label      string
	Executable string
}

type launchdManager struct {
	plistPath  string
	configDir  string
	executable string
}

// NewManager creates a macOS launchd service manager.
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

	return &launchdManager{
		plistPath:  filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"),
		configDir:  configDir,
		executable: exe,
	}, nil
}

func (m *launchdManager) Install() error {
	tmpl, err := template.New("plist").Parse(plistTmpl)
	if err != nil {
		return fmt.Errorf("parsing plist template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, plistData{
		Label:      launchdLabel,
		Executable: m.executable,
	}); err != nil {
		return fmt.Errorf("rendering plist: %w", err)
	}

	// Ensure LaunchAgents directory exists.
	if err := os.MkdirAll(filepath.Dir(m.plistPath), 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	if err := os.WriteFile(m.plistPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	return nil
}

func (m *launchdManager) Start() error {
	// Unload first in case the plist is already registered with launchd
	// (e.g. auto-loaded on login from ~/Library/LaunchAgents, or left
	// loaded from a previous run that crashed). This is idempotent —
	// if not loaded, launchctl unload fails silently.
	_ = exec.Command("launchctl", "unload", m.plistPath).Run()

	// Load the plist (registers with launchd).
	if err := exec.Command("launchctl", "load", m.plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}

	// Start the job.
	if err := exec.Command("launchctl", "start", launchdLabel).Run(); err != nil {
		return fmt.Errorf("launchctl start: %w", err)
	}

	return nil
}

func (m *launchdManager) Uninstall() error {
	// Stop and unload if running.
	_ = exec.Command("launchctl", "stop", launchdLabel).Run()
	_ = exec.Command("launchctl", "unload", m.plistPath).Run()

	// Remove the plist file.
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}

	// Clean up PID file.
	RemovePID(m.configDir)
	return nil
}

func (m *launchdManager) Stop() error {
	// Stop the job (ignore error — may not be running).
	_ = exec.Command("launchctl", "stop", launchdLabel).Run()

	// Unload the plist.
	if err := exec.Command("launchctl", "unload", m.plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl unload: %w", err)
	}

	return nil
}

func (m *launchdManager) Status() (Status, error) {
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
