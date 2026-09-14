// Package assets carries the files embedded into the client binary.
//
// plume.ogg is still a placeholder — a synthesised paper scratch, not the
// real sound — pending the rest of the Design deliverables of ENCRE_02 §16.
// The three fonts below are not: ticket encre-amh chose them after
// downloading and testing every candidate against client/ui's
// MissingGlyphs, and client/assets/fonts holds the OFL license text each
// ships with.
package assets

import _ "embed"

// PlumeOgg is the scratch played when a letter is typed. Ogg Vorbis, the format
// ENCRE_04 §2 fixes for every sound in the game.
//
//go:embed plume.ogg
var PlumeOgg []byte

// FontGreffeTTF is Ark Pixel 10 px proportional latin, subsetted to the runes
// the game draws: Le Greffe of brief/ENCRE_02 §5 — the interface and the
// digits. SIL OFL 1.1, no Reserved Font Name; see
// client/assets/fonts/OFL-ArkPixel.txt. Its native grid is 10 logical
// pixels — [ui.LoadFace] must only ever be asked for a whole multiple of it
// (ENCRE_02 §15).
//
//go:embed fonts/ArkPixel10-Latin.ttf
var FontGreffeTTF []byte

// FontPlumeTTF is Ark Pixel 16 px proportional latin, subsetted the same way:
// La Plume of brief/ENCRE_02 §5 — titles and the word being copied. Same
// license as [FontGreffeTTF]. Its native grid is 16 logical pixels.
//
//go:embed fonts/ArkPixel16-Latin.ttf
var FontPlumeTTF []byte

// FontCursiveTTF is Marelle, subsetted and renamed EncreCursive: La Cursive
// of brief/ENCRE_02 §5 — the Enluminure and the Cahier screen, nothing else.
// SIL OFL 1.1 with the Reserved Font Name "Marelle", which is why this
// modified, subsetted file carries a different name instead; see
// client/assets/fonts/OFL-Marelle.txt. It is vectorial: no grid constraint
// applies to the size it is loaded at.
//
//go:embed fonts/EncreCursive.ttf
var FontCursiveTTF []byte

// JuiceJSON carries the timings of brief/ENCRE_02 §12 as the hand-editable
// juice.json of brief/ENCRE_04 §10, parsed with anim.LoadJuice. It exists
// alongside anim.DefaultJuice, and a test keeps the two equal: the Go defaults
// are what the game runs on with no file at all, this is what a designer edits
// to change them without a rebuild.
//
//go:embed juice.json
var JuiceJSON []byte
