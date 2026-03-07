package brain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	tmp := t.TempDir()

	if err := Init(tmp); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, dir := range requiredDirs {
		path := filepath.Join(tmp, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

func TestInit_Idempotent(t *testing.T) {
	tmp := t.TempDir()

	// Create a file inside .brain/ to verify Init doesn't clobber it.
	brainDir := filepath.Join(tmp, ".brain")
	os.MkdirAll(brainDir, 0755)
	sentinel := filepath.Join(brainDir, "index.json")
	os.WriteFile(sentinel, []byte(`{}`), 0644)

	if err := Init(tmp); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	data, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("sentinel file missing after Init: %v", err)
	}
	if string(data) != "{}" {
		t.Error("sentinel file was modified by Init")
	}
}

func TestIsStaticPath(t *testing.T) {
	brain := t.TempDir()

	tests := []struct {
		name   string
		path   string
		expect bool
	}{
		{"static dir itself", filepath.Join(brain, "static"), true},
		{"file in static", filepath.Join(brain, "static", "photo.jpg"), true},
		{"nested in static", filepath.Join(brain, "static", "sub", "file.txt"), true},
		{"brain root", brain, false},
		{"file in root", filepath.Join(brain, "notes.md"), false},
		{"dotbrain dir", filepath.Join(brain, ".brain", "index.json"), false},
		{"meta dir", filepath.Join(brain, ".meta", "file.json"), false},
		{"static-prefix trick", filepath.Join(brain, "static-notes", "file.txt"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsStaticPath(brain, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expect {
				t.Errorf("IsStaticPath(%q) = %v, want %v", tt.path, got, tt.expect)
			}
		})
	}
}

func TestIsInsideBrain(t *testing.T) {
	brain := t.TempDir()

	tests := []struct {
		name   string
		path   string
		expect bool
	}{
		{"brain root", brain, true},
		{"file in brain", filepath.Join(brain, "notes.md"), true},
		{"nested file", filepath.Join(brain, "sub", "deep", "file.txt"), true},
		{"outside brain", "/tmp/other", false},
		{"parent of brain", filepath.Dir(brain), false},
		{"brain-prefix trick", brain + "-extra/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsInsideBrain(brain, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expect {
				t.Errorf("IsInsideBrain(%q) = %v, want %v", tt.path, got, tt.expect)
			}
		})
	}
}

func TestValidateWritePath(t *testing.T) {
	brain := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"writable file", filepath.Join(brain, "notes.md"), false},
		{"writable nested", filepath.Join(brain, "drafts", "post.md"), false},
		{"dotbrain file", filepath.Join(brain, ".brain", "index.json"), false},
		{"meta file", filepath.Join(brain, ".meta", "file.json"), false},
		{"static file blocked", filepath.Join(brain, "static", "photo.jpg"), true},
		{"static nested blocked", filepath.Join(brain, "static", "sub", "f.txt"), true},
		{"outside brain blocked", "/tmp/other/file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWritePath(brain, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWritePath(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
