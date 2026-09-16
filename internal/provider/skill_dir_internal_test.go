package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A symlink inside the skill directory must not be followed: it could point at
// a credentials file anywhere on the machine and upload it under a harmless
// name.
func TestReadSkillDirSkipsSymlinks(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "credentials")
	if err := os.WriteFile(secret, []byte("aws_access_key_id = AKIAIOSFODNN7EXAMPLE"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "example-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "notes.md")); err != nil {
		t.Skipf("symlinks not supported here: %v", err)
	}
	files, _, err := readSkillDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "notes.md") || strings.Contains(string(f.Data), "AKIA") {
			t.Fatalf("symlink target was uploaded as %s", f.Path)
		}
	}
	if len(files) != 1 {
		t.Fatalf("expected only SKILL.md, got %d files", len(files))
	}
}

// The file-count limit is enforced during the walk, not after everything has
// been read into memory.
func TestReadSkillDirEnforcesFileCountWhileWalking(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "example-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < skillMaxFiles+5; i++ {
		if err := os.WriteFile(filepath.Join(dir, "f"+string(rune('a'+i%26))+string(rune('a'+i/26))+".txt"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err := readSkillDir(dir)
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("expected a file-count error, got %v", err)
	}
}
