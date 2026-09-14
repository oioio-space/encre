package api_test

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
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
