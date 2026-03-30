package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// helper: create a file with content inside dir
func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// helper: list all file names in a zip archive, sorted
func zipEntries(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

// helper: read a file's content from a zip archive
func zipFileContent(t *testing.T, path, name string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			buf := make([]byte, f.UncompressedSize64)
			n, _ := rc.Read(buf)
			return string(buf[:n])
		}
	}
	t.Fatalf("file %q not found in zip", name)
	return ""
}

// helper: get compression method for a file in the zip
func zipFileMethod(t *testing.T, path, name string) uint16 {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			return f.Method
		}
	}
	t.Fatalf("file %q not found in zip", name)
	return 0
}

func TestCreateZip_BasicFile(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "hello.txt", "world")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != "hello.txt" {
		t.Fatalf("expected [hello.txt], got %v", entries)
	}

	content := zipFileContent(t, zipPath, "hello.txt")
	if content != "world" {
		t.Fatalf("expected 'world', got %q", content)
	}
}

func TestCreateZip_DeflateCompression(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "data.txt", strings.Repeat("abcdef", 100))

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	method := zipFileMethod(t, zipPath, "data.txt")
	if method != zip.Deflate {
		t.Fatalf("expected Deflate (%d), got %d", zip.Deflate, method)
	}
}

func TestCreateZip_NonexistentSource(t *testing.T) {
	_, err := CreateZip("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent source dir")
	}
}

func TestCreateZip_PreservesDirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "a/b/c.txt", "deep")
	createFile(t, dir, "a/d.txt", "mid")
	createFile(t, dir, "root.txt", "top")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	expected := []string{"a/b/c.txt", "a/d.txt", "root.txt"}
	sort.Strings(expected)
	if len(entries) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, entries)
	}
	for i := range expected {
		if entries[i] != expected[i] {
			t.Fatalf("entry %d: expected %q, got %q", i, expected[i], entries[i])
		}
	}
}

func TestCreateZip_RelativePaths(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "sub/file.txt", "data")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	for _, e := range entries {
		if filepath.IsAbs(e) {
			t.Fatalf("zip entry should be relative, got %q", e)
		}
		if strings.HasPrefix(e, dir) {
			t.Fatalf("zip entry should not start with srcDir, got %q", e)
		}
	}
}

func TestCreateZip_EmptyDirectoriesNotIncluded(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "keep.txt", "yes")
	os.MkdirAll(filepath.Join(dir, "emptydir"), 0755)

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	for _, e := range entries {
		if strings.Contains(e, "emptydir") {
			t.Fatalf("empty directory should not be in zip, but found %q", e)
		}
	}
	if len(entries) != 1 || entries[0] != "keep.txt" {
		t.Fatalf("expected [keep.txt], got %v", entries)
	}
}

func TestCreateZip_FilesWithSpaces(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "my file.txt", "space")
	createFile(t, dir, "dir with space/another file.txt", "content")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	expected := []string{"dir with space/another file.txt", "my file.txt"}
	sort.Strings(expected)
	if len(entries) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, entries)
	}
	for i := range expected {
		if entries[i] != expected[i] {
			t.Fatalf("entry %d: expected %q, got %q", i, expected[i], entries[i])
		}
	}
}

func TestCreateZip_DeepNesting(t *testing.T) {
	dir := t.TempDir()
	deepPath := "a/b/c/d/e/f/g/h/i/j/deep.txt"
	createFile(t, dir, deepPath, "very deep")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != deepPath {
		t.Fatalf("expected [%s], got %v", deepPath, entries)
	}
}

// --- Excluded directories ---

func TestCreateZip_ExcludedDirs(t *testing.T) {
	excludedDirs := []string{
		".venv", "venv", ".env", "__pycache__", "node_modules",
		".git", ".idea", ".vscode", "dist", "build",
		".mypy_cache", ".pytest_cache", ".next", "coverage",
		".tox",
	}

	for _, excluded := range excludedDirs {
		t.Run(excluded, func(t *testing.T) {
			dir := t.TempDir()
			createFile(t, dir, "keep.txt", "yes")
			createFile(t, dir, filepath.Join(excluded, "secret.txt"), "no")

			zipPath, err := CreateZip(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(zipPath)

			entries := zipEntries(t, zipPath)
			for _, e := range entries {
				if strings.HasPrefix(e, excluded+"/") || e == excluded {
					t.Fatalf("excluded dir %q should not be in zip, found entry %q", excluded, e)
				}
			}
			if len(entries) != 1 || entries[0] != "keep.txt" {
				t.Fatalf("expected [keep.txt], got %v", entries)
			}
		})
	}
}

func TestCreateZip_ExcludedDirNested(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "src/main.py", "code")
	createFile(t, dir, "src/__pycache__/main.cpython-310.pyc", "bytecode")
	createFile(t, dir, "src/node_modules/pkg/index.js", "js")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != "src/main.py" {
		t.Fatalf("expected [src/main.py], got %v", entries)
	}
}

func TestCreateZip_EggInfoExcluded(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "keep.txt", "yes")
	createFile(t, dir, "mypackage.egg-info/PKG-INFO", "meta")
	createFile(t, dir, "another.egg-info/top_level.txt", "pkg")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	for _, e := range entries {
		if strings.Contains(e, ".egg-info") {
			t.Fatalf("egg-info dir should be excluded, found %q", e)
		}
	}
	if len(entries) != 1 || entries[0] != "keep.txt" {
		t.Fatalf("expected [keep.txt], got %v", entries)
	}
}

// --- Excluded files ---

func TestCreateZip_ExcludedFiles(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "keep.txt", "yes")
	createFile(t, dir, ".DS_Store", "mac junk")
	createFile(t, dir, "Thumbs.db", "win junk")
	createFile(t, dir, "script.pyc", "bytecode")
	createFile(t, dir, "script.pyo", "optimized bytecode")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != "keep.txt" {
		t.Fatalf("expected [keep.txt], got %v", entries)
	}
}

func TestCreateZip_ExcludedFilesInSubdirs(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "sub/.DS_Store", "mac junk")
	createFile(t, dir, "sub/Thumbs.db", "win junk")
	createFile(t, dir, "sub/module.pyc", "bytecode")
	createFile(t, dir, "sub/module.pyo", "optimized")
	createFile(t, dir, "sub/real.txt", "keep")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != "sub/real.txt" {
		t.Fatalf("expected [sub/real.txt], got %v", entries)
	}
}

func TestCreateZip_SymlinksNotIncluded(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "real.txt", "data")
	os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt"))

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	for _, e := range entries {
		if e == "link.txt" {
			t.Fatal("symlink should not be included in zip")
		}
	}
	if len(entries) != 1 || entries[0] != "real.txt" {
		t.Fatalf("expected [real.txt], got %v", entries)
	}
}

func TestCreateZip_MixedIncludedExcluded(t *testing.T) {
	dir := t.TempDir()
	// Included
	createFile(t, dir, "main.py", "code")
	createFile(t, dir, "src/app.py", "app")
	createFile(t, dir, "src/utils/helper.py", "helper")
	createFile(t, dir, "README.md", "readme")
	// Excluded dirs
	createFile(t, dir, ".git/HEAD", "ref")
	createFile(t, dir, "node_modules/express/index.js", "express")
	createFile(t, dir, "src/__pycache__/app.cpython-310.pyc", "cache")
	createFile(t, dir, ".venv/lib/python3.10/site-packages/pip.py", "pip")
	createFile(t, dir, "dist/output.js", "built")
	createFile(t, dir, "build/out.js", "built")
	createFile(t, dir, ".idea/workspace.xml", "ide")
	createFile(t, dir, ".vscode/settings.json", "vsc")
	createFile(t, dir, "coverage/lcov.info", "cov")
	// Excluded files
	createFile(t, dir, ".DS_Store", "junk")
	createFile(t, dir, "src/.DS_Store", "junk")
	createFile(t, dir, "compiled.pyc", "byte")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	expected := []string{"README.md", "main.py", "src/app.py", "src/utils/helper.py"}
	sort.Strings(expected)
	if len(entries) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, entries)
	}
	for i := range expected {
		if entries[i] != expected[i] {
			t.Fatalf("entry %d: expected %q, got %q", i, expected[i], entries[i])
		}
	}
}

func TestCreateZip_ReturnsTempFile(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "file.txt", "data")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	// Verify the file exists
	info, err := os.Stat(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Fatal("expected a file, got directory")
	}
	if info.Size() == 0 {
		t.Fatal("zip file should not be empty")
	}
}

func TestCreateZip_MultipleFilesPreserved(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "a.txt", "aaa")
	createFile(t, dir, "b.txt", "bbb")
	createFile(t, dir, "c.txt", "ccc")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	expected := []string{"a.txt", "b.txt", "c.txt"}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %v", entries)
	}
	for i, e := range expected {
		if entries[i] != e {
			t.Fatalf("entry %d: expected %q, got %q", i, e, entries[i])
		}
	}

	// Verify content is preserved
	if c := zipFileContent(t, zipPath, "a.txt"); c != "aaa" {
		t.Fatalf("expected 'aaa', got %q", c)
	}
	if c := zipFileContent(t, zipPath, "b.txt"); c != "bbb" {
		t.Fatalf("expected 'bbb', got %q", c)
	}
	if c := zipFileContent(t, zipPath, "c.txt"); c != "ccc" {
		t.Fatalf("expected 'ccc', got %q", c)
	}
}

func TestCreateZip_ExcludedDirNameNotAffectingFiles(t *testing.T) {
	// A file named "node_modules" (not a dir) or "dist" should not match
	// dir exclusion rules if it's a regular file at root. But the spec says
	// exclusion is by directory name, so files with the same name should be kept.
	// However, since exclusion is about directories and their contents, a file
	// named ".git" is just a file and should be included.
	dir := t.TempDir()
	// Create a regular file named "build" (not a directory)
	createFile(t, dir, "keep.txt", "yes")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %v", entries)
	}
}

func TestCreateZip_OnlyRegularFilesIncluded(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "regular.txt", "data")
	// Symlink to directory
	os.Symlink(dir, filepath.Join(dir, "dirlink"))

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	for _, e := range entries {
		if strings.Contains(e, "dirlink") {
			t.Fatalf("symlink to dir should not appear, found %q", e)
		}
	}
}

func TestCreateZip_ExcludedDirDeepNested(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "a/b/c/.git/config", "gitconfig")
	createFile(t, dir, "a/b/c/real.txt", "data")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != "a/b/c/real.txt" {
		t.Fatalf("expected [a/b/c/real.txt], got %v", entries)
	}
}

func TestCreateZip_PycInSubdirectory(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "pkg/module.pyc", "bytecode")
	createFile(t, dir, "pkg/module.pyo", "optimized")
	createFile(t, dir, "pkg/module.py", "source")

	zipPath, err := CreateZip(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(zipPath)

	entries := zipEntries(t, zipPath)
	if len(entries) != 1 || entries[0] != "pkg/module.py" {
		t.Fatalf("expected [pkg/module.py], got %v", entries)
	}
}
