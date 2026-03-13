package watcher

import (
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestClassify(t *testing.T) {
	brainDir := "/home/user/brain"

	tests := []struct {
		name     string
		path     string
		op       fsnotify.Op
		wantCat  Category
		wantOp   string
		wantPath string
	}{
		{
			name:     "normal file create",
			path:     filepath.Join(brainDir, "notes", "idea.md"),
			op:       fsnotify.Create,
			wantCat:  CategoryMutable,
			wantOp:   "create",
			wantPath: "notes/idea.md",
		},
		{
			name:     "TODO.md modify",
			path:     filepath.Join(brainDir, "TODO.md"),
			op:       fsnotify.Write,
			wantCat:  CategoryTodo,
			wantOp:   "modify",
			wantPath: "TODO.md",
		},
		{
			name:     "topofmind.md modify",
			path:     filepath.Join(brainDir, "topofmind.md"),
			op:       fsnotify.Write,
			wantCat:  CategoryTopOfMind,
			wantOp:   "modify",
			wantPath: "topofmind.md",
		},
		{
			name:     "static file create",
			path:     filepath.Join(brainDir, "static", "photos", "new.jpg"),
			op:       fsnotify.Create,
			wantCat:  CategoryStatic,
			wantOp:   "create",
			wantPath: "static/photos/new.jpg",
		},
		{
			name:     "static dir itself",
			path:     filepath.Join(brainDir, "static"),
			op:       fsnotify.Create,
			wantCat:  CategoryStatic,
			wantOp:   "create",
			wantPath: "static",
		},
		{
			name:     ".brain dir ignored",
			path:     filepath.Join(brainDir, ".brain", "state.json"),
			op:       fsnotify.Write,
			wantCat:  CategoryIgnored,
			wantOp:   "modify",
			wantPath: ".brain/state.json",
		},
		{
			name:     ".meta dir ignored",
			path:     filepath.Join(brainDir, ".meta", "sidecar.json"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: ".meta/sidecar.json",
		},
		{
			name:     ".DS_Store ignored",
			path:     filepath.Join(brainDir, ".DS_Store"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: ".DS_Store",
		},
		{
			name:     "swap file ignored",
			path:     filepath.Join(brainDir, "notes", ".idea.md.swp"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: "notes/.idea.md.swp",
		},
		{
			name:     "tmp file ignored",
			path:     filepath.Join(brainDir, "download.tmp"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: "download.tmp",
		},
		{
			name:     "Office lock file ignored",
			path:     filepath.Join(brainDir, "~$document.docx"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: "~$document.docx",
		},
		{
			name:     "sync conflict ignored",
			path:     filepath.Join(brainDir, ".sync-conflict-20260101-abc123.md"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: ".sync-conflict-20260101-abc123.md",
		},
		{
			name:     "partial download ignored",
			path:     filepath.Join(brainDir, "file.crdownload"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: "file.crdownload",
		},
		{
			name:     ".stfolder ignored",
			path:     filepath.Join(brainDir, ".stfolder", "marker"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: ".stfolder/marker",
		},
		{
			name:     "Thumbs.db ignored",
			path:     filepath.Join(brainDir, "Thumbs.db"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: "Thumbs.db",
		},
		{
			name:     "LibreOffice lock ignored",
			path:     filepath.Join(brainDir, ".~lock.document.odt#"),
			op:       fsnotify.Create,
			wantCat:  CategoryIgnored,
			wantOp:   "create",
			wantPath: ".~lock.document.odt#",
		},
		{
			name:     "delete op",
			path:     filepath.Join(brainDir, "old.md"),
			op:       fsnotify.Remove,
			wantCat:  CategoryMutable,
			wantOp:   "delete",
			wantPath: "old.md",
		},
		{
			name:     "rename op",
			path:     filepath.Join(brainDir, "renamed.md"),
			op:       fsnotify.Rename,
			wantCat:  CategoryMutable,
			wantOp:   "rename",
			wantPath: "renamed.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := fsnotify.Event{Name: tt.path, Op: tt.op}
			fe := Classify(brainDir, ev)

			if fe.Category != tt.wantCat {
				t.Errorf("category = %v, want %v", fe.Category, tt.wantCat)
			}
			if fe.Op != tt.wantOp {
				t.Errorf("op = %q, want %q", fe.Op, tt.wantOp)
			}
			if fe.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", fe.Path, tt.wantPath)
			}
		})
	}
}

func TestIsIgnoredDir(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".brain", true},
		{".brain/sub", true},
		{".meta", true},
		{".stfolder", true},
		{".dropbox.cache", true},
		{"static", false},
		{"notes", false},
		{"brain", false}, // not .brain
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsIgnoredDir(tt.path); got != tt.want {
				t.Errorf("IsIgnoredDir(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
