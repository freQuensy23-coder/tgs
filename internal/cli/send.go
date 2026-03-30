package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/freQuensy23-coder/tgs/internal/archive"
	"github.com/freQuensy23-coder/tgs/internal/config"
	"github.com/freQuensy23-coder/tgs/internal/sender"
)

func cmdSend(ctx context.Context, targetName string, path string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path %q: %w", path, err)
	}

	filePath := path
	displayName := filepath.Base(path)

	if fi.IsDir() {
		fmt.Fprintf(os.Stderr, "Zipping %s...\n", path)
		zipPath, err := archive.CreateZip(path)
		if err != nil {
			return fmt.Errorf("zip: %w", err)
		}
		defer os.Remove(zipPath)

		namedZip := filepath.Join(filepath.Dir(zipPath), filepath.Base(path)+".zip")
		if err := os.Rename(zipPath, namedZip); err != nil {
			namedZip = zipPath
		} else {
			defer os.Remove(namedZip)
		}

		filePath = namedZip
		displayName = filepath.Base(namedZip)
	}

	s, err := sender.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer s.Close()

	target := sender.Target{Name: targetName}

	fmt.Fprintf(os.Stderr, "Sending %s...\n", displayName)
	if err := s.SendFile(ctx, target, filePath); err != nil {
		return err
	}

	dest := "Saved Messages"
	if targetName != "" {
		dest = targetName
	}
	fmt.Fprintf(os.Stderr, "Sent %s to %s\n", displayName, dest)
	return nil
}
