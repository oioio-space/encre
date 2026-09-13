// Package encre — décrire ici en une phrase ce que fait la bibliothèque,
// l'approche en une phrase, et comment l'utiliser (voir la checklist GoDoc du
// pre-commit : ce paragraphe est rendu tel quel sur pkg.go.dev).
package encre

// Greet returns a friendly greeting for name. It is the starter's placeholder
// export — replace it with the project's real API (and keep its test pattern).
func Greet(name string) string {
	if name == "" {
		name = "world"
	}
	return "Hello, " + name + "!"
}
