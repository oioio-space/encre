package media_test

import (
	"errors"
	"os"
	"path/filepath"
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
