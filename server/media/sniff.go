package media

import "bytes"

// webmMagic is the EBML header every Matroska/WebM file starts with
// (the four bytes MediaRecorder's audio/webm output always opens with,
// container audio or video). net/http.DetectContentType classifies this
// signature as "video/webm" — it has no dedicated audio/webm entry — so
// this package checks the magic bytes directly rather than trusting that
// classification's label.
var webmMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}

// oggMagic is the four bytes an Ogg container (the format [Transcode]
// writes) always opens with.
var oggMagic = []byte("OggS")

// SniffWebm reports whether data opens with the EBML/WebM signature: the
// content check ENCRE_04 §8 asks for ("type MIME vérifié sur le contenu,
// pas sur l'extension") for an incoming voice recording, before it is ever
// handed to ffmpeg.
func SniffWebm(data []byte) bool { return bytes.HasPrefix(data, webmMagic) }

// SniffOgg reports whether data opens with the Ogg container signature.
func SniffOgg(data []byte) bool { return bytes.HasPrefix(data, oggMagic) }
