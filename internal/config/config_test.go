package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setHome overrides HOME (or USERPROFILE on Windows) to point at a temp dir,
// returning a cleanup function that restores the original value.
func setHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmp)
	} else {
		t.Setenv("HOME", tmp)
	}
	return tmp
}

func TestDir_CreatesDirectory(t *testing.T) {
	home := setHome(t)

	d, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(home, ".tgs")
	if d != expected+string(filepath.Separator) && d != expected {
		// Allow trailing slash or not
		if filepath.Clean(d) != filepath.Clean(expected) {
			t.Fatalf("expected dir %q (or with trailing sep), got %q", expected, d)
		}
	}

	info, err := os.Stat(filepath.Clean(d))
	if err != nil {
		t.Fatalf("dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0700 {
			t.Fatalf("expected 0700 permissions, got %o", info.Mode().Perm())
		}
	}
}

func TestDir_Idempotent(t *testing.T) {
	setHome(t)

	d1, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("Dir() should be idempotent, got %q and %q", d1, d2)
	}
}

func TestLoad_MissingConfig(t *testing.T) {
	setHome(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when config file doesn't exist")
	}
	if !contains(err.Error(), "not logged in") {
		t.Fatalf("expected error message containing 'not logged in', got %q", err.Error())
	}
}

func TestSaveAndLoad_BotMode(t *testing.T) {
	setHome(t)

	cfg := &Config{
		Mode:        "bot",
		BotToken:    "123456:ABC-DEF",
		OwnerChatID: 987654321,
	}

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Mode != "bot" {
		t.Fatalf("expected mode 'bot', got %q", loaded.Mode)
	}
	if loaded.BotToken != "123456:ABC-DEF" {
		t.Fatalf("expected token '123456:ABC-DEF', got %q", loaded.BotToken)
	}
	if loaded.OwnerChatID != 987654321 {
		t.Fatalf("expected owner chat id 987654321, got %d", loaded.OwnerChatID)
	}
	// User-mode fields should be zero values
	if loaded.AppID != 0 {
		t.Fatalf("expected AppID 0, got %d", loaded.AppID)
	}
	if loaded.AppHash != "" {
		t.Fatalf("expected empty AppHash, got %q", loaded.AppHash)
	}
	if loaded.Phone != "" {
		t.Fatalf("expected empty Phone, got %q", loaded.Phone)
	}
}

func TestSaveAndLoad_UserMode(t *testing.T) {
	setHome(t)

	cfg := &Config{
		Mode:    "user",
		AppID:   12345,
		AppHash: "abcdef0123456789",
		Phone:   "+15551234567",
	}

	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Mode != "user" {
		t.Fatalf("expected mode 'user', got %q", loaded.Mode)
	}
	if loaded.AppID != 12345 {
		t.Fatalf("expected AppID 12345, got %d", loaded.AppID)
	}
	if loaded.AppHash != "abcdef0123456789" {
		t.Fatalf("expected AppHash 'abcdef0123456789', got %q", loaded.AppHash)
	}
	if loaded.Phone != "+15551234567" {
		t.Fatalf("expected Phone '+15551234567', got %q", loaded.Phone)
	}
	// Bot-mode fields should be zero values
	if loaded.BotToken != "" {
		t.Fatalf("expected empty BotToken, got %q", loaded.BotToken)
	}
	if loaded.OwnerChatID != 0 {
		t.Fatalf("expected OwnerChatID 0, got %d", loaded.OwnerChatID)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test not applicable on Windows")
	}
	home := setHome(t)

	cfg := &Config{Mode: "bot", BotToken: "test"}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(home, ".tgs", "config.json")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestSave_JSONFieldOmission_BotMode(t *testing.T) {
	home := setHome(t)

	cfg := &Config{
		Mode:        "bot",
		BotToken:    "token123",
		OwnerChatID: 42,
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(home, ".tgs", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// User-only fields should be omitted in bot mode
	for _, field := range []string{"app_id", "appId", "AppID", "app_hash", "appHash", "AppHash", "phone", "Phone"} {
		if _, ok := raw[field]; ok {
			t.Fatalf("field %q should be omitted in bot mode JSON, but was present", field)
		}
	}
}

func TestSave_JSONFieldOmission_UserMode(t *testing.T) {
	home := setHome(t)

	cfg := &Config{
		Mode:    "user",
		AppID:   999,
		AppHash: "hash",
		Phone:   "+1234",
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(home, ".tgs", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// Bot-only fields should be omitted in user mode
	for _, field := range []string{"bot_token", "botToken", "BotToken", "owner_chat_id", "ownerChatId", "OwnerChatID"} {
		if _, ok := raw[field]; ok {
			t.Fatalf("field %q should be omitted in user mode JSON, but was present", field)
		}
	}
}

func TestSessionPath(t *testing.T) {
	home := setHome(t)

	cfg := &Config{Mode: "user"}
	sp := cfg.SessionPath()

	expected := filepath.Join(home, ".tgs", "user.session")
	if sp != expected {
		t.Fatalf("expected %q, got %q", expected, sp)
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	setHome(t)

	cfg1 := &Config{Mode: "bot", BotToken: "first"}
	if err := cfg1.Save(); err != nil {
		t.Fatal(err)
	}

	cfg2 := &Config{Mode: "bot", BotToken: "second"}
	if err := cfg2.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.BotToken != "second" {
		t.Fatalf("expected 'second', got %q", loaded.BotToken)
	}
}

func TestSave_ValidJSON(t *testing.T) {
	home := setHome(t)

	cfg := &Config{
		Mode:        "bot",
		BotToken:    "tok",
		OwnerChatID: 1,
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(home, ".tgs", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if !json.Valid(data) {
		t.Fatal("saved config is not valid JSON")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
