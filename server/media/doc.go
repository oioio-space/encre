// Package media transcodes and stores the audio ENCRE plays: the parent's
// own voice, recorded in the browser, and — where no recording exists yet
// — a synthesized fallback (brief/ENCRE_04 §8).
//
// This package is deliberately the most defensive one in server/: an audio
// upload plus a call out to ffmpeg is a textbook command-injection and
// path-traversal combination, and every function here is written against
// that threat model specifically, not just against a working ffmpeg
// install. In particular:
//
//   - [Transcode] runs ffmpeg through [os/exec.Command] with every argument
//     as its own slice element. There is no shell anywhere in this package,
//     so there is nothing for a filename or a flag value to inject into.
//   - [Root.Path] is the only way this package ever builds a file path, and
//     it only ever accepts IDs [ValidID] has already checked — never a
//     filename, extension or path fragment taken from a request body or a
//     multipart header. A client cannot make it write, read or delete
//     anything outside its own child's list.
//   - Every function that reads an upload bounds how much it reads
//     (server/api bounds the body), and [SniffWebm] checks the bytes actually sent
//     rather than trusting a Content-Type header or a filename extension.
//
// [Transcode] requires ffmpeg on PATH and returns [ErrFFmpegNotFound]
// cleanly if it is absent, rather than panicking or hanging; the caller is
// expected to say so plainly to the parent, not fail some other way.
// [SynthesizePiper] holds the same contract for Piper
// ([ErrPiperNotFound]) — Piper is not installed on every deployment target
// this project has been built and tested on, and this package never
// invents a binary to paper over that; it reports the gap.
package media
