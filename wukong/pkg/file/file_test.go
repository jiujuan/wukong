package file

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestReadWriteAndPathChecks(t *testing.T) {
	fs := afero.NewMemMapFs()
	op := New(fs)

	if err := op.EnsureDir("data/logs"); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	if err := op.WriteString("data/logs/app.txt", "hello\nworld"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if err := op.AppendFile("data/logs/app.txt", []byte("\nagain")); err != nil {
		t.Fatalf("AppendFile failed: %v", err)
	}

	text, err := op.ReadString("data/logs/app.txt")
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}
	if text != "hello\nworld\nagain" {
		t.Fatalf("unexpected content: %q", text)
	}

	if got, err := op.ReadHead("data/logs/app.txt", 5); err != nil || got != "hello" {
		t.Fatalf("ReadHead = %q, %v", got, err)
	}
	if got, err := op.ReadTail("data/logs/app.txt", 5); err != nil || got != "again" {
		t.Fatalf("ReadTail = %q, %v", got, err)
	}
	if got, err := op.ReadRange("data/logs/app.txt", 6, 5); err != nil || string(got) != "world" {
		t.Fatalf("ReadRange = %q, %v", string(got), err)
	}

	if got, err := op.ConfirmDir("data/logs"); err != nil || filepath.Clean(got) != filepath.Clean("data/logs") {
		t.Fatalf("ConfirmDir = %q, %v", got, err)
	}
	if got, err := op.ConfirmFile("data/logs/app.txt"); err != nil || filepath.Clean(got) != filepath.Clean("data/logs/app.txt") {
		t.Fatalf("ConfirmFile = %q, %v", got, err)
	}

	if ok, err := op.IsDir("data/logs"); err != nil || !ok {
		t.Fatalf("IsDir = %v, %v", ok, err)
	}
	if ok, err := op.IsFile("data/logs/app.txt"); err != nil || !ok {
		t.Fatalf("IsFile = %v, %v", ok, err)
	}
	if ok, err := op.Exists("data/logs/app.txt"); err != nil || !ok {
		t.Fatalf("Exists = %v, %v", ok, err)
	}
}

func TestFileAndDirPathUnderBase(t *testing.T) {
	fs := afero.NewMemMapFs()
	op := New(fs)

	if err := op.EnsureDir("workspace/reports"); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	if err := op.WriteString("workspace/reports/daily.md", "report"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	path, err := op.FilePath("workspace", "reports/daily.md")
	if err != nil {
		t.Fatalf("FilePath failed: %v", err)
	}
	if filepath.Clean(path) != filepath.Clean("workspace/reports/daily.md") {
		t.Fatalf("FilePath = %q", path)
	}

	dir, err := op.DirPath("workspace", "reports")
	if err != nil {
		t.Fatalf("DirPath failed: %v", err)
	}
	if filepath.Clean(dir) != filepath.Clean("workspace/reports") {
		t.Fatalf("DirPath = %q", dir)
	}

	if _, err := op.FilePath("workspace", "../escape.md"); err == nil {
		t.Fatal("expected escape file path to fail")
	}
}

func TestScanDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	op := New(fs)

	for _, path := range []string{
		"root/a.txt",
		"root/b.md",
		"root/nested/c.txt",
		"root/nested/inner/d.log",
	} {
		if err := op.WriteString(path, path); err != nil {
			t.Fatalf("WriteString(%s) failed: %v", path, err)
		}
	}

	entries, err := op.ScanDir("root", ScanOptions{Recursive: true, Exts: []string{"txt"}})
	if err != nil {
		t.Fatalf("ScanDir failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ScanDir len = %d, want 2", len(entries))
	}
	if filepath.Clean(entries[0].Path) != filepath.Clean("root/a.txt") || filepath.Clean(entries[1].Path) != filepath.Clean("root/nested/c.txt") {
		t.Fatalf("unexpected scan entries: %#v", entries)
	}

	dirs, err := op.ScanDir("root", ScanOptions{Recursive: true, DirsOnly: true})
	if err != nil {
		t.Fatalf("ScanDir dirs failed: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("expected directories in recursive scan")
	}
}

func TestReadLinesAndWrittenInDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	op := New(fs)

	path, err := op.WriteInDir("root/output", "note.txt", "one\ntwo\nthree")
	if err != nil {
		t.Fatalf("WriteInDir failed: %v", err)
	}
	if filepath.Clean(path) != filepath.Clean("root/output/note.txt") {
		t.Fatalf("WriteInDir path = %q", path)
	}

	lines, err := op.ReadLines(path, 1, 2)
	if err != nil {
		t.Fatalf("ReadLines failed: %v", err)
	}
	if len(lines) != 2 || lines[0] != "two" || lines[1] != "three" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestCreateDirAndFileByPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	op := New(fs)

	dir, err := op.CreateDir("workspace/output")
	if err != nil {
		t.Fatalf("CreateDir failed: %v", err)
	}
	if filepath.Clean(dir) != filepath.Clean("workspace/output") {
		t.Fatalf("CreateDir path = %q", dir)
	}
	if ok, err := op.IsDir(dir); err != nil || !ok {
		t.Fatalf("created dir check = %v, %v", ok, err)
	}

	filePath, err := op.CreateTextFile("workspace/output/result.txt", "done")
	if err != nil {
		t.Fatalf("CreateTextFile failed: %v", err)
	}
	if filepath.Clean(filePath) != filepath.Clean("workspace/output/result.txt") {
		t.Fatalf("CreateTextFile path = %q", filePath)
	}
	text, err := op.ReadString(filePath)
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}
	if text != "done" {
		t.Fatalf("created file content = %q", text)
	}

	emptyPath, err := op.CreateEmptyFile("workspace/output/empty.txt")
	if err != nil {
		t.Fatalf("CreateEmptyFile failed: %v", err)
	}
	if ok, err := op.IsFile(emptyPath); err != nil || !ok {
		t.Fatalf("created empty file check = %v, %v", ok, err)
	}
}

func TestCreateDirAndFileInDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	op := New(fs)

	dir, err := op.CreateDirIn("workspace", "reports/daily")
	if err != nil {
		t.Fatalf("CreateDirIn failed: %v", err)
	}
	if filepath.Clean(dir) != filepath.Clean("workspace/reports/daily") {
		t.Fatalf("CreateDirIn path = %q", dir)
	}

	filePath, err := op.CreateTextFileInDir(dir, "summary.md", "# summary")
	if err != nil {
		t.Fatalf("CreateTextFileInDir failed: %v", err)
	}
	if filepath.Clean(filePath) != filepath.Clean("workspace/reports/daily/summary.md") {
		t.Fatalf("CreateTextFileInDir path = %q", filePath)
	}

	if _, err := op.CreateTextFileInDir("workspace/reports", "../escape.md", "bad"); err == nil {
		t.Fatal("expected escaping file name to fail")
	}
	if _, err := op.CreateDirIn("workspace/reports", "../escape"); err == nil {
		t.Fatal("expected escaping dir name to fail")
	}
}
