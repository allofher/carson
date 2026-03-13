package watcher

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Category classifies the type of file change detected.
type Category int

const (
	CategoryIgnored  Category = iota // .brain/, .meta/, sync junk
	CategoryStatic                   // static/ subtree
	CategoryTodo                     // TODO.md
	CategoryTopOfMind                // topofmind.md
	CategoryMutable                  // everything else
)

func (c Category) String() string {
	switch c {
	case CategoryIgnored:
		return "ignored"
	case CategoryStatic:
		return "static"
	case CategoryTodo:
		return "todo"
	case CategoryTopOfMind:
		return "topofmind"
	case CategoryMutable:
		return "mutable"
	}
	return "unknown"
}

// FileEvent is a classified filesystem event.
type FileEvent struct {
	Path      string   `json:"path"`      // relative to brain dir
	Op        string   `json:"op"`        // "create", "modify", "delete", "rename"
	Category  Category `json:"category"`
	Timestamp time.Time `json:"timestamp"`
}

// ignoredDirs are directory prefixes to ignore entirely.
var ignoredDirs = []string{
	".brain",
	".meta",
	".stfolder",
	".dropbox.cache",
}

// ignoredPatterns are file name patterns to ignore (matched against basename).
var ignoredPatterns = []string{
	"~$*",
	".~lock.*",
	"*.swp",
	"*.swx",
	"*.tmp",
	"*.partial",
	"*.crdownload",
	".DS_Store",
	"Thumbs.db",
}

// ignoredPrefixes are filename prefixes to ignore.
var ignoredPrefixes = []string{
	".sync-conflict-",
}

// Classify converts an fsnotify event into a categorized FileEvent.
func Classify(brainDir string, ev fsnotify.Event) FileEvent {
	rel, err := filepath.Rel(brainDir, ev.Name)
	if err != nil {
		rel = ev.Name
	}
	// Normalize to forward slashes for consistency.
	rel = filepath.ToSlash(rel)

	fe := FileEvent{
		Path:      rel,
		Op:        opString(ev.Op),
		Timestamp: time.Now(),
	}

	fe.Category = classifyPath(rel)
	return fe
}

// IsIgnoredDir reports whether a directory path (relative to brain dir)
// should not be watched.
func IsIgnoredDir(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	for _, dir := range ignoredDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+"/") {
			return true
		}
	}
	return false
}

func classifyPath(rel string) Category {
	// Check ignored dirs.
	for _, dir := range ignoredDirs {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return CategoryIgnored
		}
	}

	// Check ignored file patterns against basename.
	base := filepath.Base(rel)
	for _, pattern := range ignoredPatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return CategoryIgnored
		}
	}
	for _, prefix := range ignoredPrefixes {
		if strings.HasPrefix(base, prefix) {
			return CategoryIgnored
		}
	}

	// Static subtree.
	if rel == "static" || strings.HasPrefix(rel, "static/") {
		return CategoryStatic
	}

	// Special files.
	if rel == "TODO.md" {
		return CategoryTodo
	}
	if rel == "topofmind.md" {
		return CategoryTopOfMind
	}

	return CategoryMutable
}

func opString(op fsnotify.Op) string {
	switch {
	case op.Has(fsnotify.Create):
		return "create"
	case op.Has(fsnotify.Write):
		return "modify"
	case op.Has(fsnotify.Remove):
		return "delete"
	case op.Has(fsnotify.Rename):
		return "rename"
	default:
		return "unknown"
	}
}
