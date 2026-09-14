package api_test

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/oioio-space/encre/server/media"
	"github.com/oioio-space/encre/server/store"
)

// multipartAudioBody builds a multipart/form-data body with one "audio"
// file field carrying data, and returns it with its Content-Type.
func multipartAudioBody(t *testing.T, filename string, data []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("audio", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("writing multipart body: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}
	return body, w.FormDataContentType()
}

func postMultipart(t *testing.T, ts *testServer, path string, body *bytes.Buffer, contentType string) *http.Response {
	t.Helper()
	resp, err := ts.Client.Post(ts.URL+path, contentType, body)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestUploadItemAudioTranscodesAndStoresPath(t *testing.T) {
	if _, err := media.LookupFFmpeg(); errors.Is(err, media.ErrFFmpegNotFound) {
		t.Skip("ffmpeg not found on PATH; skipping the real transcode")
	}
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	webm, err := os.ReadFile("../media/testdata/sample.webm")
	if err != nil {
		t.Fatalf("reading sample.webm: %v", err)
	}
	body, ct := multipartAudioBody(t, "voice.webm", webm)

	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	type uploadResponse struct{ AudioPath string }
	got := decodeBody[uploadResponse](t, resp)
	if got.AudioPath == "" {
		t.Fatal("AudioPath is empty")
	}

	item, err := ts.DB.ItemByID(t.Context(), itemID)
	if err != nil {
		t.Fatalf("ItemByID() error = %v", err)
	}
	if item.AudioPath != got.AudioPath {
		t.Errorf("stored AudioPath = %q, want %q", item.AudioPath, got.AudioPath)
	}
	if _, err := os.Stat(root.Dir() + "/" + got.AudioPath); err != nil {
		t.Errorf("transcoded file not on disk at %s: %v", got.AudioPath, err)
	}
}

// TestUploadItemAudioMissingAudioFieldAnswers400 posts a multipart body with
// no "audio" field at all and checks the handler answers 400 rather than
// panicking on a nil file or a zero-value [multipart.FileHeader] — this
// catches a regression where r.FormFile's error is ignored.
func TestUploadItemAudioMissingAudioFieldAnswers400(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	// multipartAudioBody always names its field "audio"; build this one by
	// hand under a different field name so r.FormFile("audio") finds nothing.
	var raw bytes.Buffer
	mw := multipart.NewWriter(&raw)
	part, err := mw.CreateFormFile("not-audio", "voice.webm")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte{0x1A, 0x45, 0xDF, 0xA3}); err != nil {
		t.Fatalf("writing multipart body: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", &raw, mw.FormDataContentType())
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("upload with no \"audio\" field: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestUploadItemAudioMalformedMultipartBodyAnswers400 sends a request whose
// Content-Type claims multipart/form-data but whose body is not one, so
// r.ParseMultipartForm itself fails — distinct from a well-formed multipart
// body missing the "audio" field.
func TestUploadItemAudioMalformedMultipartBodyAnswers400(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio",
		bytes.NewBufferString("this is not a multipart body"), "multipart/form-data; boundary=nope")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("upload with a malformed multipart body: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestUploadItemAudioOversizedBodyAnswers400 sends an "audio" field past
// maxAudioUploadBytes and checks the handler answers 400 rather than
// buffering the whole thing — ENCRE_04 §8's "une taille maximale sur le
// corps" enforced by http.MaxBytesReader ahead of ParseMultipartForm.
func TestUploadItemAudioOversizedBodyAnswers400(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	oversized := make([]byte, 21<<20) // past maxAudioUploadBytes (20 MiB)
	copy(oversized, []byte{0x1A, 0x45, 0xDF, 0xA3})
	body, ct := multipartAudioBody(t, "voice.webm", oversized)

	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("upload past the size cap: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestUploadItemAudioFFmpegFailureAnswers503 sends a payload that opens
// with the WebM signature (so [media.SniffWebm] accepts it) but is not
// actually decodable audio, so ffmpeg itself fails once invoked — the
// handler must answer 503 (errMediaUnavailable), the same response it gives
// when ffmpeg is entirely missing, rather than a 500 that implies a server
// bug.
func TestUploadItemAudioFFmpegFailureAnswers503(t *testing.T) {
	if _, err := media.LookupFFmpeg(); errors.Is(err, media.ErrFFmpegNotFound) {
		t.Skip("ffmpeg not found on PATH; skipping the real transcode failure")
	}
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	// The WebM signature followed by garbage: SniffWebm passes on the first
	// four bytes alone, but ffmpeg has nothing decodable to transcode.
	junk := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, bytes.Repeat([]byte{0xFF}, 64)...)
	body, ct := multipartAudioBody(t, "voice.webm", junk)

	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("upload of WebM-signed garbage: status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// TestUploadItemAudioTempFileFailureAnswers500 forces os.CreateTemp (inside
// writeTempUpload) to fail by pointing TMPDIR at a path that does not
// exist, and checks the handler answers 500 rather than panicking on the
// nil path writeTempUpload would otherwise return.
func TestUploadItemAudioTempFileFailureAnswers500(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))

	body, ct := multipartAudioBody(t, "voice.webm", []byte{0x1A, 0x45, 0xDF, 0xA3})
	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("upload with an unwritable TMPDIR: status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestUploadItemAudioRejectsNonWebmContent(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	body, ct := multipartAudioBody(t, "not-audio.webm", []byte("this is not a webm file, just text"))
	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("upload of non-webm content (renamed .webm): status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUploadItemAudioWithNoMediaRootAnswers503(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginParent(t, ts, "child1-parent")

	body, ct := multipartAudioBody(t, "voice.webm", []byte{0x1A, 0x45, 0xDF, 0xA3})
	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("upload with no media root configured: status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestUploadItemAudioRequiresParentSession(t *testing.T) {
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	loginChild(t, ts, "Mia", "1379")

	body, ct := multipartAudioBody(t, "voice.webm", []byte{0x1A, 0x45, 0xDF, 0xA3})
	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/items/"+itemID+"/audio", body, ct)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("upload with a child session: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestUploadSentenceAudioTranscodesAndStoresPath(t *testing.T) {
	if _, err := media.LookupFFmpeg(); errors.Is(err, media.ErrFFmpegNotFound) {
		t.Skip("ffmpeg not found on PATH; skipping the real transcode")
	}
	ts := newTestServer(t)
	seedChild(t, ts.DB, "child1", "Mia", "1379")
	listID, itemID := seedSentenceItem(t, ts, "child1")
	sentence := &store.Sentence{ID: "s1", ItemID: itemID, Text: "Le chat dort.", TargetForm: "chat"}
	if err := ts.DB.SaveSentence(t.Context(), sentence); err != nil {
		t.Fatalf("SaveSentence() error = %v", err)
	}
	root, err := media.NewRoot(t.TempDir())
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	ts.APIServer.SetMediaRoot(root)
	loginParent(t, ts, "child1-parent")

	webm, err := os.ReadFile("../media/testdata/sample.webm")
	if err != nil {
		t.Fatalf("reading sample.webm: %v", err)
	}
	body, ct := multipartAudioBody(t, "voice.webm", webm)

	resp := postMultipart(t, ts, "/api/v1/lists/"+listID+"/sentences/s1/audio", body, ct)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	got, err := ts.DB.SentenceByID(t.Context(), "s1")
	if err != nil {
		t.Fatalf("SentenceByID() error = %v", err)
	}
	if got.AudioPath == "" {
		t.Error("AudioPath is empty after upload")
	}
}
