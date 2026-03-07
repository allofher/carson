package brain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// requiredDirs are the subdirectories created on initialization.
var requiredDirs = []string{
	".brain",
	".meta",
	"static",
	"daily-summary",
}

// Init ensures the brain folder and its required subdirectories exist.
// It creates any missing directories but never overwrites existing content.
func Init(brainDir string) error {
	for _, dir := range requiredDirs {
		path := filepath.Join(brainDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return nil
}

// IsStaticPath reports whether the given absolute path falls under the
// static/ subdirectory of the brain folder. This is the single permission
// boundary: writes to static/ are blocked by tool handlers.
func IsStaticPath(brainDir, targetPath string) (bool, error) {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false, fmt.Errorf("resolving target path: %w", err)
	}
	absBrain, err := filepath.Abs(brainDir)
	if err != nil {
		return false, fmt.Errorf("resolving brain dir: %w", err)
	}

	staticDir := filepath.Join(absBrain, "static") + string(filepath.Separator)
	staticDirExact := filepath.Join(absBrain, "static")

	return absTarget == staticDirExact || strings.HasPrefix(absTarget, staticDir), nil
}

// IsInsideBrain reports whether the given absolute path falls within the
// brain folder. Used to sandbox file operations.
func IsInsideBrain(brainDir, targetPath string) (bool, error) {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false, fmt.Errorf("resolving target path: %w", err)
	}
	absBrain, err := filepath.Abs(brainDir)
	if err != nil {
		return false, fmt.Errorf("resolving brain dir: %w", err)
	}

	brainPrefix := absBrain + string(filepath.Separator)
	return absTarget == absBrain || strings.HasPrefix(absTarget, brainPrefix), nil
}

// ValidateWritePath checks that a target path is inside the brain folder
// and not under static/. Returns nil if the write is allowed, or an error
// describing why it was rejected.
func ValidateWritePath(brainDir, targetPath string) error {
	inside, err := IsInsideBrain(brainDir, targetPath)
	if err != nil {
		return err
	}
	if !inside {
		return fmt.Errorf("path %q is outside the brain folder", targetPath)
	}

	isStatic, err := IsStaticPath(brainDir, targetPath)
	if err != nil {
		return err
	}
	if isStatic {
		return fmt.Errorf("path %q is under static/ and is read-only", targetPath)
	}

	return nil
}
