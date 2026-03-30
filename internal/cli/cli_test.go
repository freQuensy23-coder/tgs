package cli

import (
	"context"
	"strings"
	"testing"
)

func TestRun_NoArgs_ReturnsNil(t *testing.T) {
	err := Run(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected nil for no args, got: %v", err)
	}
}

func TestRun_TooManyArgs(t *testing.T) {
	err := Run(context.Background(), []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("expected error for too many arguments")
	}
	if !strings.Contains(err.Error(), "too many arguments") {
		t.Fatalf("expected 'too many arguments' in error, got %q", err.Error())
	}
}

func TestRun_NonexistentPath(t *testing.T) {
	err := Run(context.Background(), []string{"/nonexistent/path/to/file.txt"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got %q", err.Error())
	}
}

func TestRun_LoginWithoutMode(t *testing.T) {
	err := Run(context.Background(), []string{"login"})
	if err == nil {
		t.Fatal("expected error for login without mode")
	}
	// The error should mention the missing mode
	errMsg := err.Error()
	if errMsg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestRun_NonexistentCommand(t *testing.T) {
	err := Run(context.Background(), []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got %q", err.Error())
	}
}
