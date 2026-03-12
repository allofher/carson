package scheduler

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	cronHeader = "# >>> CARSON MANAGED — DO NOT EDIT <<<"
	cronFooter = "# <<< CARSON MANAGED >>>"
)

// CrontabManager reads and writes entries in the CARSON MANAGED block
// of the user's crontab.
type CrontabManager struct {
	carsonBin string // absolute path to the carson binary
}

// NewCrontabManager creates a manager that writes crontab entries pointing
// to the given carson binary path.
func NewCrontabManager(carsonBin string) *CrontabManager {
	return &CrontabManager{carsonBin: carsonBin}
}

// AddEvent adds a crontab entry for a one-shot scheduled event.
// The cron expression is derived from the bundle's FiresAt time.
func (m *CrontabManager) AddEvent(b *Bundle) error {
	cronExpr := timeToCron(b.FiresAt)
	comment := truncateComment(b.Prompt, 60)
	line := fmt.Sprintf("# [%s] %s\n%s %s run-scheduled --event %s",
		b.ID, comment, cronExpr, m.carsonBin, b.ID)
	return m.addLine(b.ID, line)
}

// AddRecurring adds a crontab entry for a recurring scheduled event.
func (m *CrontabManager) AddRecurring(b *Bundle) error {
	comment := truncateComment(b.Prompt, 60)
	line := fmt.Sprintf("# [%s] %s\n%s %s run-scheduled --event %s",
		b.ID, comment, b.Recurrence, m.carsonBin, b.ID)
	return m.addLine(b.ID, line)
}

// Remove removes the crontab entry for the given event ID.
func (m *CrontabManager) Remove(id string) error {
	current, err := m.readCrontab()
	if err != nil {
		return err
	}

	block, before, after := m.extractBlock(current)
	if block == "" {
		return nil // no managed block, nothing to remove
	}

	updated := m.removeEntry(block, id)
	return m.writeCrontab(before + updated + after)
}

// addLine inserts a line into the CARSON MANAGED block, replacing any
// existing entry with the same ID.
func (m *CrontabManager) addLine(id, line string) error {
	current, err := m.readCrontab()
	if err != nil {
		return err
	}

	block, before, after := m.extractBlock(current)
	if block == "" {
		// No managed block yet — create one.
		block = cronHeader + "\n" + cronFooter + "\n"
		if current != "" && !strings.HasSuffix(current, "\n") {
			current += "\n"
		}
		before = current
		after = ""
	}

	// Remove any existing entry for this ID first.
	block = m.removeEntry(block, id)

	// Insert new entry before the footer.
	block = strings.Replace(block, cronFooter, line+"\n"+cronFooter, 1)

	return m.writeCrontab(before + block + after)
}

// extractBlock splits the crontab into (managed_block, before, after).
// If no managed block exists, returns ("", "", "").
func (m *CrontabManager) extractBlock(crontab string) (block, before, after string) {
	headerIdx := strings.Index(crontab, cronHeader)
	if headerIdx == -1 {
		return "", "", ""
	}
	footerIdx := strings.Index(crontab, cronFooter)
	if footerIdx == -1 {
		return "", "", ""
	}
	footerEnd := footerIdx + len(cronFooter)
	if footerEnd < len(crontab) && crontab[footerEnd] == '\n' {
		footerEnd++
	}

	return crontab[headerIdx:footerEnd], crontab[:headerIdx], crontab[footerEnd:]
}

// removeEntry removes all lines belonging to the given event ID from the block.
// An entry consists of a comment line (# [id] ...) and the following command line.
func (m *CrontabManager) removeEntry(block, id string) string {
	marker := fmt.Sprintf("# [%s]", id)
	lines := strings.Split(block, "\n")
	var result []string
	skip := false
	for _, line := range lines {
		if strings.Contains(line, marker) {
			skip = true // skip this comment line
			continue
		}
		if skip {
			skip = false // skip the command line that follows
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func (m *CrontabManager) readCrontab() (string, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// "no crontab for user" is not an error for us.
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no crontab") {
				return "", nil
			}
		}
		return "", fmt.Errorf("reading crontab: %w", err)
	}
	return string(out), nil
}

func (m *CrontabManager) writeCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("writing crontab: %w: %s", err, string(out))
	}
	return nil
}

// timeToCron converts a time.Time to a cron expression for a one-shot event.
// Format: minute hour day month *
func timeToCron(t time.Time) string {
	return fmt.Sprintf("%d %d %d %d *", t.Minute(), t.Hour(), t.Day(), int(t.Month()))
}

func truncateComment(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
