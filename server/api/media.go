package api

import (
	"io"
	"net/http"
	"os"

	"github.com/oioio-space/encre/server/media"
)

// maxAudioUploadBytes bounds a voice recording upload: generous for a few
// minutes of webm/opus at MediaRecorder's usual bitrate, and small enough
// that a client cannot make the handler buffer an unbounded amount of
// memory or disk before ffmpeg ever runs (ENCRE_04 §8's own instruction:
// "une taille maximale sur le corps").
const maxAudioUploadBytes = 20 << 20 // 20 MiB

// maxMultipartMemoryBytes bounds how much of an upload [http.Request.ParseMultipartForm]
// buffers in memory; anything past it spills to a temp file the standard
// library cleans up. It is deliberately far smaller than
// maxAudioUploadBytes — see the comment at its one call site.
const maxMultipartMemoryBytes = 1 << 20 // 1 MiB

// errMediaUnavailable is what the two upload handlers answer with when
// there is no way to store or transcode a recording at all: no
// [Server.mediaRoot] configured, or ffmpeg missing
// ([media.ErrFFmpegNotFound]) — the "ffmpeg peut être absent, le dire
// proprement" this ticket asks for, in the one shape a client needs to
// react to (retry later, do not treat this as the recording being bad).
const errMediaUnavailable = "enregistrement audio indisponible pour le moment"

// handleUploadItemAudio accepts a parent's voice recording for one word
// (ENCRE_04 §8: multipart webm/opus → ffmpeg → Ogg Vorbis, normalized,
// silence trimmed) and stores its path on the item.
func (s *Server) handleUploadItemAudio(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	item, ok := s.ownItem(w, r, list)
	if !ok {
		return
	}
	relPath, ok := s.transcodeUpload(w, r, list.ID, item.ID)
	if !ok {
		return
	}
	item.AudioPath = relPath
	if err := s.db.SaveItem(r.Context(), item); err != nil {
		s.log.Error("saving item audio path", "item", item.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		AudioPath string `json:"audioPath"`
	}{relPath})
}

// handleUploadSentenceAudio is [handleUploadItemAudio]'s twin for a
// sentence's own recording (ENCRE_04 §7, "idem pour sentences").
func (s *Server) handleUploadSentenceAudio(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.parentSession(w, r)
	if !ok {
		return
	}
	list, ok := s.ownList(w, r, sess)
	if !ok {
		return
	}
	sentence, ok := s.ownSentence(w, r, list)
	if !ok {
		return
	}
	relPath, ok := s.transcodeUpload(w, r, list.ID, sentence.ID)
	if !ok {
		return
	}
	sentence.AudioPath = relPath
	if err := s.db.SaveSentence(r.Context(), sentence); err != nil {
		s.log.Error("saving sentence audio path", "sentence", sentence.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		AudioPath string `json:"audioPath"`
	}{relPath})
}

// transcodeUpload reads r's multipart "audio" field, checks its content
// (never its filename or declared Content-Type) opens with the WebM
// signature, transcodes it through ffmpeg into listID/mediaID.ogg under
// [Server.mediaRoot], and returns the path stored relative to the media
// root. On any failure it writes the appropriate response and returns
// ok=false.
func (s *Server) transcodeUpload(w http.ResponseWriter, r *http.Request, listID, mediaID string) (relPath string, ok bool) {
	if !s.mediaRootSet {
		writeError(w, http.StatusServiceUnavailable, errMediaUnavailable)
		return "", false
	}

	// The request body as a whole is capped at maxAudioUploadBytes
	// (http.MaxBytesReader); maxMultipartMemoryBytes below is a second,
	// smaller bound on top of that — how much of it ParseMultipartForm may
	// hold in memory before spilling the rest to a temp file — so a
	// maxAudioUploadBytes-sized request never has to be held in RAM
	// twice over.
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioUploadBytes)
	// #nosec G120 -- maxMultipartMemoryBytes (1 MiB) is a small, fixed
	// bound, and http.MaxBytesReader above already caps the whole request
	// body at maxAudioUploadBytes regardless of what ParseMultipartForm
	// does with it; gosec's rule flags every ParseMultipartForm call in an
	// HTTP handler without weighing the argument's actual value.
	if err := r.ParseMultipartForm(maxMultipartMemoryBytes); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide ou trop volumineux")
		return "", false
	}
	file, _, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, `champ "audio" manquant`)
		return "", false
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "corps de requête invalide")
		return "", false
	}
	if !media.SniffWebm(data) {
		writeError(w, http.StatusBadRequest, "le fichier envoyé n'est pas un enregistrement webm valide")
		return "", false
	}

	inPath, cleanup, err := writeTempUpload(data)
	if err != nil {
		s.log.Error("writing temp upload", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return "", false
	}
	defer cleanup()

	outPath, err := s.mediaRoot.Path(listID, mediaID)
	if err != nil {
		s.log.Error("resolving media path", "list", listID, "media", mediaID, "error", err)
		writeError(w, http.StatusBadRequest, "identifiant invalide")
		return "", false
	}

	if err := media.Transcode(r.Context(), media.DefaultTranscodeOptions, inPath, outPath); err != nil {
		s.log.Error("transcoding upload", "list", listID, "media", mediaID, "error", err)
		writeError(w, http.StatusServiceUnavailable, errMediaUnavailable)
		return "", false
	}

	relPath, err = s.mediaRoot.RelPath(listID, mediaID)
	if err != nil {
		s.log.Error("resolving media rel path", "error", err)
		writeError(w, http.StatusInternalServerError, "erreur serveur")
		return "", false
	}
	return relPath, true
}

// writeTempUpload writes data to a fresh temporary file and returns its
// path and a cleanup func that removes it. ffmpeg (media.Transcode) is
// always given a real path, never a pipe or an in-memory reader — the
// simplest way to keep its own argument list free of anything but opaque
// paths.
func writeTempUpload(data []byte) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "encre-upload-*.webm")
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	cleanup = func() { _ = os.Remove(name) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return name, cleanup, nil
}
