package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestSetup(t *testing.T) {
	dir := t.TempDir()
	logger, bc, err := Setup(dir, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Subscribe before logging.
	sub := bc.Subscribe()
	defer sub.Unsubscribe()

	logger.Info("hello", "key", "value")

	// Check that the log file was created and has content.
	data, err := os.ReadFile(filepath.Join(dir, "carson.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("log file is empty")
	}

	// Check that the subscriber received the message.
	select {
	case msg := <-sub.C:
		if len(msg) == 0 {
			t.Error("received empty message")
		}
	default:
		t.Error("subscriber did not receive message")
	}
}

func TestRotatingWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Small maxSize to trigger rotation quickly.
	w, err := NewRotatingWriter(path, 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Write enough to trigger rotation.
	msg := []byte("abcdefghij0123456789012345678901234567890123456789\n") // 51 bytes
	w.Write(msg)
	w.Write(msg) // should trigger rotation
	w.Write(msg) // new file

	// Rotated file should exist.
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("rotated file .1 should exist: %v", err)
	}
}

func TestBroadcaster(t *testing.T) {
	bc := NewBroadcaster()

	sub1 := bc.Subscribe()
	sub2 := bc.Subscribe()

	bc.Write([]byte("test message"))

	// Both subscribers should receive.
	msg1 := <-sub1.C
	msg2 := <-sub2.C
	if string(msg1) != "test message" || string(msg2) != "test message" {
		t.Errorf("unexpected messages: %q, %q", msg1, msg2)
	}

	// Unsubscribe one.
	sub1.Unsubscribe()
	bc.Write([]byte("second"))

	select {
	case msg := <-sub2.C:
		if string(msg) != "second" {
			t.Errorf("unexpected: %q", msg)
		}
	default:
		t.Error("sub2 should have received")
	}
}
