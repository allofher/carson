package lookout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ANSI color codes for log levels.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorGray   = "\033[90m"
)

// Run streams daemon logs. It tries the SSE endpoint first, falling back
// to tailing the log file if the daemon isn't reachable.
func Run(logDir string, port int, numLines int) error {
	// Try SSE first.
	url := fmt.Sprintf("http://127.0.0.1:%d/logs", port)
	if err := streamSSE(url); err == nil {
		return nil // SSE ran until disconnect
	}

	// Fall back to file tail.
	fmt.Fprintf(os.Stderr, "daemon not reachable — tailing log file\n\n")
	return tailFile(logDir, numLines)
}

func streamSSE(url string) error {
	client := &http.Client{Timeout: 0} // no timeout for streaming
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			printLine(data)
		}
	}
	return scanner.Err()
}

func tailFile(logDir string, numLines int) error {
	logPath := filepath.Join(logDir, "carson.log")

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no log file found at %s — is the daemon running?", logPath)
		}
		return err
	}
	defer f.Close()

	if err := printTail(f, numLines); err != nil {
		return err
	}

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return err
		}
		printLine(line)
	}
}

func printTail(f *os.File, n int) error {
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	for _, line := range lines[start:] {
		printLine(line)
	}
	return nil
}

func printLine(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}

	var base map[string]any
	if err := json.Unmarshal([]byte(raw), &base); err != nil {
		fmt.Println(raw)
		return
	}

	ts := getString(base, "time")
	level := strings.ToUpper(getString(base, "level"))
	msg := getString(base, "msg")

	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		ts = t.Format("15:04:05")
	}

	delete(base, "time")
	delete(base, "level")
	delete(base, "msg")

	color := levelColor(level)
	extras := formatExtras(base)

	fmt.Printf("%s %s%-5s%s %s%s\n", ts, color, level, colorReset, msg, extras)
}

func levelColor(level string) string {
	switch level {
	case "ERROR":
		return colorRed
	case "WARN":
		return colorYellow
	case "INFO":
		return colorGreen
	case "DEBUG":
		return colorGray
	default:
		return colorReset
	}
}

func formatExtras(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return "  " + colorGray + strings.Join(parts, " ") + colorReset
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
