package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var defaultExcludeDirs = map[string]bool{
	".venv":          true,
	"venv":           true,
	".env":           true,
	"__pycache__":    true,
	"node_modules":   true,
	".git":           true,
	".idea":          true,
	".vscode":        true,
	"dist":           true,
	"build":          true,
	".mypy_cache":    true,
	".pytest_cache":  true,
	".next":          true,
	"coverage":       true,
	".tox":           true,
}

var defaultExcludeExts = []string{
	".pyc", ".pyo",
}

var defaultExcludeFiles = map[string]bool{
	".DS_Store":  true,
	"Thumbs.db":  true,
}

func shouldExclude(name string, isDir bool) bool {
	if isDir {
		if defaultExcludeDirs[name] {
			return true
		}
		if strings.HasSuffix(name, ".egg-info") {
			return true
		}
		return false
	}

	if defaultExcludeFiles[name] {
		return true
	}
	for _, ext := range defaultExcludeExts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// CreateZip creates a zip archive from srcDir, excluding common junk files.
// Returns the path to a temporary zip file. Caller must remove it.
func CreateZip(srcDir string) (string, error) {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "tgs-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	w := zip.NewWriter(tmpFile)

	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()

		if d.IsDir() {
			if shouldExclude(name, true) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldExclude(name, false) {
			return nil
		}

		// Only include regular files
		if !d.Type().IsRegular() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		header, err := zip.FileInfoHeader(mustFileInfo(d))
		if err != nil {
			return fmt.Errorf("file header: %w", err)
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := w.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create zip entry: %w", err)
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		if _, err := io.Copy(writer, f); err != nil {
			return fmt.Errorf("write to zip: %w", err)
		}

		return nil
	})

	if closeErr := w.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("close zip: %w", closeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("close temp file: %w", closeErr)
	}

	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

func mustFileInfo(d os.DirEntry) os.FileInfo {
	info, err := d.Info()
	if err != nil {
		// Fallback: shouldn't happen since we just walked to this entry
		panic(fmt.Sprintf("file info: %v", err))
	}
	return info
}
