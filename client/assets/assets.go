// Package assets carries the files embedded into the client binary.
//
// Everything here is a placeholder until the Design deliverables of ENCRE_02
// §16 arrive: plume.ogg is a synthesised paper scratch, not the real sound.
// It exists so the Étape 0 prototype can answer one question it must answer on
// a real phone — whether the browser's audio context actually unlocks on the
// first tap.
package assets

import _ "embed"

// PlumeOgg is the scratch played when a letter is typed. Ogg Vorbis, the format
// ENCRE_04 §2 fixes for every sound in the game.
//
//go:embed plume.ogg
var PlumeOgg []byte
