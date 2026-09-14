package media_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/oioio-space/encre/server/media"
)

func TestValidIDAcceptsBase64URLIdentifiers(t *testing.T) {
	for _, id := range []string{"abc123", "AbC-_09", "a"} {
		if !media.ValidID(id) {
			t.Errorf("ValidID(%q) = false, want true", id)
		}
	}
}

func TestValidIDRejectsPathTraversalAndSeparators(t *testing.T) {
	for _, id := range []string{"", "..", "../etc/passwd", "a/b", "a\x00b", "a b"} {
		if media.ValidID(id) {
			t.Errorf("ValidID(%q) = true, want false", id)
		}
	}
}

func TestValidIDRejectsOverlyLongIDs(t *testing.T) {
	long := make([]byte, 200)
	for i := range long {
		long[i] = 'a'
	}
	if media.ValidID(string(long)) {
		t.Error("ValidID() on a 200-rune id = true, want false")
	}
}

func TestRootPathStaysUnderRootAndRejectsBadIDs(t *testing.T) {
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}

	path, err := root.Path("list1", "item1")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	want := filepath.Join(root.Dir(), "list1", "item1.ogg")
	if path != want {
		t.Errorf("Path() = %q, want %q", path, want)
	}

	for _, bad := range []struct{ listID, itemID string }{
		{"../../etc", "passwd"},
		{"list1", "../../../etc/passwd"},
		{"", "item1"},
	} {
		if _, err := root.Path(bad.listID, bad.itemID); !errors.Is(err, media.ErrInvalidID) {
			t.Errorf("Path(%q, %q) error = %v, want ErrInvalidID", bad.listID, bad.itemID, err)
		}
	}
}

func TestRootRemoveListDeletesEveryRecording(t *testing.T) {
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	path, err := root.Path("list1", "item1")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("ogg"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := root.RemoveList("list1"); err != nil {
		t.Fatalf("RemoveList() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Stat() after RemoveList() error = %v, want IsNotExist", err)
	}
}

func TestRootRemoveListOnMissingListIsNotAnError(t *testing.T) {
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	if err := root.RemoveList("never-existed"); err != nil {
		t.Errorf("RemoveList() on a list with no directory: error = %v, want nil", err)
	}
}

// TestNewRootFailsWhenDirCannotBeCreated checks that [media.NewRoot]
// reports os.MkdirAll's error rather than silently returning a Root rooted
// somewhere that does not exist — this proves NewRoot("<file>/sub") returns
// an error instead of a Root nothing can write through.
func TestNewRootFailsWhenDirCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := media.NewRoot(filepath.Join(blocker, "sub")); err == nil {
		t.Error("NewRoot() under a path whose parent is a plain file: want error, got nil")
	}
}

// TestRootPathFailsWhenListDirCannotBeCreated checks that [Root.Path]
// reports os.MkdirAll's error when the list directory it needs to create
// is blocked by a same-named plain file, rather than returning a path that
// nothing can actually write through.
func TestRootPathFailsWhenListDirCannotBeCreated(t *testing.T) {
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	blocker := filepath.Join(root.Dir(), "list1")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := root.Path("list1", "item1"); err == nil {
		t.Error("Path() whose list directory is blocked by a plain file: want error, got nil")
	}
}

// TestRootRelPathMatchesPathAndRejectsBadIDs checks that [Root.RelPath]
// returns the same listID/itemID.ogg segment [Root.Path] writes to
// (relative rather than absolute), and rejects the same bad IDs [Root.Path]
// does — the two must never drift apart, since callers store RelPath's
// result and later resolve it against Path's own root.
func TestRootRelPathMatchesPathAndRejectsBadIDs(t *testing.T) {
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}

	rel, err := root.RelPath("list1", "item1")
	if err != nil {
		t.Fatalf("RelPath() error = %v", err)
	}
	want := filepath.Join("list1", "item1.ogg")
	if rel != want {
		t.Errorf("RelPath() = %q, want %q", rel, want)
	}

	abs, err := root.Path("list1", "item1")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if filepath.Join(root.Dir(), rel) != abs {
		t.Errorf("RelPath() joined onto root = %q, want Path()'s result %q", filepath.Join(root.Dir(), rel), abs)
	}

	for _, bad := range []struct{ listID, itemID string }{
		{"../../etc", "passwd"},
		{"list1", "../../../etc/passwd"},
		{"", "item1"},
	} {
		if _, err := root.RelPath(bad.listID, bad.itemID); !errors.Is(err, media.ErrInvalidID) {
			t.Errorf("RelPath(%q, %q) error = %v, want ErrInvalidID", bad.listID, bad.itemID, err)
		}
	}
}

// TestRootRemoveListReportsPermissionErrors checks that [Root.RemoveList]
// surfaces a failure to actually delete the list directory (here, a parent
// directory made read-only after the list was written) rather than
// reporting success while the recordings are still on disk.
func TestRootRemoveListReportsPermissionErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not block directory removal the same way on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}

	dir := t.TempDir()
	root, err := media.NewRoot(dir)
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	path, err := root.Path("list1", "item1")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("ogg"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := os.Chmod(root.Dir(), 0o500); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root.Dir(), 0o700) })

	if err := root.RemoveList("list1"); err == nil {
		t.Error("RemoveList() on a directory it lacks permission to unlink: want error, got nil")
	}
}

func TestSniffWebmAndOgg(t *testing.T) {
	webm, err := os.ReadFile("testdata/sample.webm")
	if err != nil {
		t.Fatalf("reading testdata/sample.webm: %v", err)
	}
	notAudio, err := os.ReadFile("testdata/not_audio.txt")
	if err != nil {
		t.Fatalf("reading testdata/not_audio.txt: %v", err)
	}

	if !media.SniffWebm(webm) {
		t.Error("SniffWebm() on a real webm file = false, want true")
	}
	if media.SniffWebm(notAudio) {
		t.Error("SniffWebm() on a text file = true, want false")
	}
	if media.SniffOgg(webm) {
		t.Error("SniffOgg() on a webm file = true, want false")
	}
}
