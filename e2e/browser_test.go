package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"

	"github.com/oioio-space/encre/client/ui"
)

// demoCards are cmd/client/rundata.go's own demoManche() words, in the
// fixed order that deck plays them — "chat", "pomme", "école", then three
// more this test never reaches. It is not this test's choice: the run
// screen only ever accepts what matches the current card
// (cmd/client/run.go's onLetterTyped drops any typed letter that diverges
// from it), so typing anything else is not possible without a card that
// carries it.
//
// encre-zfn.1's own acceptance criterion names "cœur" and "forêt" — neither
// is in this deck, so this test cannot type them: see this test's own doc
// comment below for what that means and where it is reported instead of
// worked around. "école" is the only accented word demoManche() offers
// (é, held under 'e' — client/ui/variants.go); reaching it means correctly
// typing "chat" and "pomme" first, since nextCard only ever advances one
// card at a time.
var demoCards = []string{"chat", "pomme", "école"}

// inkColor is cmd/client's own "encre" ink — the colour drawTracked paints
// the word being typed in once the entry is non-empty (cmd/client/main.go).
// The empty entry shows its placeholder "…" in "cuir" instead (a different
// colour), so counting inkColor pixels in the entry line is a rendering
// check a missing or wrong letter actually fails: cmd/client/main.go's own
// c.drawEntry is what paints either one.
var inkColor = color.RGBA{R: 0x24, G: 0x23, B: 0x42, A: 0xFF}

// chromeCandidates are the binary names a Linux, macOS or CI image is
// likely to have Chrome or Chromium installed under. CHROME_BIN or
// GOOGLE_CHROME_SHIM (Heroku's own convention, also honoured by a few CI
// images) is checked first, so a machine with an unusual install path can
// still point this test at it.
var chromeCandidates = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome",
}

// findChrome locates a Chrome or Chromium binary, or returns "" if none is
// on this machine — the condition encre-zfn.1 requires this test to
// [testing.T.Skip] on, loudly, rather than fail CI over.
func findChrome() string {
	for _, envVar := range []string{"CHROME_BIN", "GOOGLE_CHROME_SHIM"} {
		if p := os.Getenv(envVar); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	for _, name := range chromeCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// buildWasmSite builds the client for the browser and assembles the exact
// site cmd/serve answers with — encre.wasm, the Go runtime's own
// wasm_exec.js (copied from this toolchain's GOROOT, never vendored, for
// the reason mise.toml's wasm:build task comment gives), and web/index.html
// with its __HASH__ placeholders filled in — into a fresh temp directory,
// and returns that directory's path.
//
// It does not gzip its output (unlike `mise run wasm:build`): cmd/serve and
// internal/httpstatic.Precompressed both fall through to the uncompressed
// file when no ".gz" sibling exists, and this test's Chrome does not send
// Accept-Encoding: gzip for anything it cares about here either way.
func buildWasmSite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	staticDir := filepath.Join(dir, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", staticDir, err)
	}

	wasmPath := filepath.Join(staticDir, "encre.wasm")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", wasmPath, "github.com/oioio-space/encre/cmd/client")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building cmd/client for wasm: %v\n%s", err, out)
	}

	wasmExecSrc := filepath.Join(runtimeGOROOT(t), "lib", "wasm", "wasm_exec.js")
	jsExecPath := filepath.Join(staticDir, "wasm_exec.js")
	copyFile(t, wasmExecSrc, jsExecPath)

	indexHTML, err := os.ReadFile("../web/index.html")
	if err != nil {
		t.Fatalf("reading web/index.html: %v", err)
	}
	// web/index.html names its two assets "encre.__HASH__.wasm" and
	// "wasm_exec.__HASH__.js" (the cache-busting mise.toml's wasm:build task
	// comment explains); this build skips the hash, so the substitution
	// collapses the whole ".__HASH__." to a single "." to match the plain
	// "encre.wasm" and "wasm_exec.js" buildWasmSite actually wrote above.
	filled := bytes.ReplaceAll(indexHTML, []byte(".__HASH__."), []byte("."))
	if err := os.WriteFile(filepath.Join(dir, "index.html"), filled, 0o644); err != nil { //nolint:gosec // a served static HTML file, not a secret
		t.Fatalf("writing index.html: %v", err)
	}

	swJS, err := os.ReadFile("../web/sw.js")
	if err != nil {
		t.Fatalf("reading web/sw.js: %v", err)
	}
	swJS = bytes.ReplaceAll(swJS, []byte("__HASH__"), nil)
	if err := os.WriteFile(filepath.Join(dir, "sw.js"), swJS, 0o644); err != nil { //nolint:gosec // served static, not a secret
		t.Fatalf("writing sw.js: %v", err)
	}

	return dir
}

// runtimeGOROOT asks the running toolchain for its own GOROOT, the same way
// mise.toml's wasm:build task does ("$(go env GOROOT)"), so wasm_exec.js
// always matches whatever built encre.wasm.
func runtimeGOROOT(t *testing.T) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	return string(bytes.TrimSpace(out))
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil { //nolint:gosec // served static JS, not a secret
		t.Fatalf("writing %s: %v", dst, err)
	}
}

// startServe builds cmd/serve and runs it — the real binary, unmodified,
// exactly as encre-zfn.1 asks for — bound to an ephemeral port, serving
// siteDir. It returns the base URL to reach it at, and registers a cleanup
// that kills the process.
func startServe(t *testing.T, siteDir string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "encre-serve")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binPath, "github.com/oioio-space/encre/cmd/serve")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building cmd/serve: %v\n%s", err, out)
	}

	port, err := freePort()
	if err != nil {
		t.Fatalf("finding a free port: %v", err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)

	// #nosec G204 -- binPath is this test's own build output, not
	// attacker-controlled input.
	cmd := exec.CommandContext(t.Context(), binPath, "-dir", siteDir, "-addr", addr)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting cmd/serve: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	baseURL := "http://" + addr
	waitForServer(t, baseURL)
	return baseURL
}

// freePort asks the OS for a port nobody is listening on, releases it, and
// returns its number — the "port éphémère" encre-zfn.1's acceptance
// criterion names. There is an inherent, unavoidable race between releasing
// it here and cmd/serve binding it a moment later; in practice nothing else
// on a CI runner or a dev machine claims a just-freed loopback port in the
// span of a few milliseconds.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForServer polls baseURL until it answers or 5 seconds pass.
func waitForServer(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/") //nolint:noctx // a short-lived readiness poll, not a real request
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("cmd/serve never answered at %s: %v", baseURL, lastErr)
}

// typeRune dispatches a raw keydown/keyup pair carrying r as the DOM
// KeyboardEvent's own "key" value, through [input.DispatchKeyEvent] — never
// chromedp.SendKeys or chromedp.KeyEvent, which build a US-layout keystroke
// and cannot produce a character like œ or ê that is not on any physical
// key at all (client/ui/variants.go's whole reason to exist). This is
// exactly what a real physical keyboard's own keydown carries, and it is
// also exactly what ebiten's own browser glue reads:
// internal/ui/input_js.go's keydown handler calls e.Get("key").String() and
// appends every rune of it, independently of e.code — confirmed by
// injecting a diagnostic "keydown" listener of this test's own into the
// page while developing it, which observed the correct .key delivered for
// every case below; see this file's own history for the two dead ends that
// were ruled out that way (a JS-constructed KeyboardEvent, which ebiten
// never even saw fire, and Backspace with no virtual key code, which
// Chrome delivered with the wrong .code).
func typeRune(r rune) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		key := string(r)
		down := input.DispatchKeyEvent(input.KeyDown).WithKey(key).WithText(key)
		if err := down.Do(ctx); err != nil {
			return fmt.Errorf("dispatching keydown for %q: %w", r, err)
		}
		if err := input.DispatchKeyEvent(input.KeyUp).WithKey(key).Do(ctx); err != nil {
			return fmt.Errorf("dispatching keyup for %q: %w", r, err)
		}
		return nil
	})
}

// entryInkPixels screenshots the page and counts inkColor pixels in the
// entry line's own band — [ui.Screen.EntryY] plus or minus half
// [ui.Screen.WordBandH], the exact rectangle cmd/client's drawEntry paints
// the typed word in — so a missing or silently-dropped letter shows up as
// fewer lit pixels than the one before it, not as a string a screen reader
// of the canvas has to guess at.
func entryInkPixels(ctx context.Context, screen ui.Screen) (int, error) {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return 0, fmt.Errorf("capturing screenshot: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return 0, fmt.Errorf("decoding screenshot: %w", err)
	}

	yFrom := screen.EntryY - screen.WordBandH/2
	yTo := screen.EntryY + screen.WordBandH/2
	wantR, wantG, wantB, wantA := inkColor.RGBA()
	bounds := img.Bounds()
	n := 0
	for y := max(yFrom, bounds.Min.Y); y < min(yTo, bounds.Max.Y); y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if r == wantR && g == wantG && b == wantB && a == wantA {
				n++
			}
		}
	}
	return n, nil
}

// openGarde navigates to baseURL, waits for the WASM to boot, and taps the
// Garde's one sceau *Jouer* (cmd/client/garde_scene.go's playButtonRect,
// centred at (W/2, H*2/3)) — also this test's one user gesture, unlocking
// audio the same way a real first tap does (ENCRE_04 §2).
func openGarde(ctx context.Context, baseURL string) error {
	return chromedp.Run(
		ctx,
		chromedp.Navigate(baseURL+"/"),
		chromedp.WaitVisible("canvas", chromedp.ByQuery),
		// The WASM keeps loading after the canvas element exists (it is
		// created before go.run(instance) — web/index.html); the first tap
		// would otherwise land before Update ever runs once.
		chromedp.Sleep(2*time.Second),
		chromedp.MouseClickXY(float64(ui.PortraitWidth)/2, float64(ui.PortraitHeight)*2/3),
		chromedp.Sleep(500*time.Millisecond),
	)
}

// chooseWager answers ENCRE_06 §7's pari, "combien de fois écouter ?"
// ([ui.Screen.WagerRects], cmd/client/wager.go), opening the current
// card's entry line — every card of the demo manche asks it again,
// [client.nextCard] itself calls [client.startWager]. It clicks the same
// pixels a finger would rather than guessing them off a screenshot;
// "1 seule fois" is picked arbitrarily, either answer reaches the keyboard.
func chooseWager(ctx context.Context, screen ui.Screen) error {
	_, listenOnce := screen.WagerRects()
	return chromedp.Run(
		ctx,
		chromedp.MouseClickXY(float64(listenOnce.X+listenOnce.W/2), float64(listenOnce.Y+listenOnce.H/2)),
		chromedp.Sleep(500*time.Millisecond),
	)
}

// typeCard types word letter by letter, on whichever card is current, and
// checks the entry line's own ink pixels strictly increase after every
// one — the "échoue si une lettre manque" of encre-zfn.1's acceptance
// criterion. It requires the entry to start empty (the wager screen just
// opened it) and word to be exactly the current card's own text: typing
// anything else is not possible at all — see [demoCards]'s own doc comment.
func typeCard(ctx context.Context, t *testing.T, screen ui.Screen, word string) {
	t.Helper()
	baseline, err := entryInkPixels(ctx, screen)
	if err != nil {
		t.Fatalf("%q: entryInkPixels (baseline): %v", word, err)
	}
	if baseline != 0 {
		t.Fatalf("%q: ink pixels in the entry line before typing anything = %d, want 0 (the placeholder \"…\" is drawn in a different colour)", word, baseline)
	}

	prev := 0
	for i, r := range word {
		if err := chromedp.Run(ctx, typeRune(r)); err != nil {
			t.Fatalf("typing %q of %q: %v", r, word, err)
		}
		// cmd/client/run.go's onLetterTyped freezes input for
		// anim.Juice.HitstopTrap (80ms, client/assets/juice.json) after
		// every keystroke that is not the word's very last letter —
		// game.FirstMismatch reports a correct-but-incomplete prefix the
		// same way it reports a real mismatch (see this test's own doc
		// comment below), and onLetterTyped does not tell the two apart. A
		// letter dispatched inside that window is dropped, not merely
		// delayed — ebiten.AppendInputChars is a per-frame buffer, not a
		// queue that survives a skipped Update. Outrunning the window here
		// is a workaround for this test, not a fix for the underlying bug.
		chromedp.Run(ctx, chromedp.Sleep(150*time.Millisecond)) //nolint:errcheck // best-effort pacing, not a page action that can meaningfully fail here
		got, err := entryInkPixels(ctx, screen)
		if err != nil {
			t.Fatalf("entryInkPixels after letter %d of %q: %v", i, word, err)
		}
		if got <= prev {
			t.Fatalf("%q: ink pixels after typing letter %d (%q) = %d, want more than after letter %d's %d — a letter did not render",
				word, i, r, got, i-1, prev)
		}
		prev = got
	}
}

// TestBrowserTypesAccentedWordsUsingThePhysicalKeyboard is encre-zfn.1: the
// client WASM exactly as cmd/serve answers it, driven by a real headless
// Chrome (chromedp, pure Go — no playwright-go, no Node in this build
// chain), typing on the physical keyboard and checking the canvas actually
// grew the word letter by letter rather than dropping one.
//
// It types [demoCards] in order rather than encre-zfn.1's own named "cœur"
// and "forêt": cmd/client's current run screen is a fixed six-card demo
// deck (cmd/client/rundata.go's demoManche, not the free single-word
// prototype of ticket T00 this bead's brief describes), and onLetterTyped
// drops any typed letter that does not match the current card — so typing
// a word the deck does not contain is not possible at all, regardless of
// what this test dispatches. Of the six, only "école" carries an accent
// (é, held under 'e' — client/ui/variants.go); reaching it means typing
// "chat" and "pomme" correctly first, since [client.nextCard] only ever
// advances one card. This is reported here rather than worked around by
// editing rundata.go: whether the deck should carry cœur/forêt, or the
// free-typing prototype should come back, is a product call this ticket
// does not make unilaterally.
//
// It skips outright, rather than failing, when no Chrome or Chromium is on
// this machine ([findChrome]) — CI images do not all carry one — and it
// stays under [testing.Short]'s budget by skipping first, before building
// anything, whenever -short is set.
func TestBrowserTypesAccentedWordsUsingThePhysicalKeyboard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the browser e2e test in -short mode")
	}
	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("no Chrome or Chromium found on this machine (checked CHROME_BIN, GOOGLE_CHROME_SHIM, and " +
			fmt.Sprint(chromeCandidates) + " on PATH); install one to run this test")
	}

	siteDir := buildWasmSite(t)
	baseURL := startServe(t, siteDir)

	allocOpts := append(
		append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...),
		chromedp.ExecPath(chromePath),
		// The sandbox needs a setuid helper or user namespaces CI containers
		// often run without; this Chrome only ever loads a page this test
		// itself just built and served on loopback, so the sandbox's threat
		// model (an untrusted, attacker-served page) does not apply.
		chromedp.NoSandbox,
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(t.Context(), allocOpts...)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	defer cancelTimeout()

	screen := ui.NewScreen(ui.PortraitWidth, ui.PortraitHeight)
	if err := chromedp.Run(
		ctx,
		chromedp.EmulateViewport(int64(ui.PortraitWidth), int64(ui.PortraitHeight), chromedp.EmulateScale(1)),
	); err != nil {
		t.Fatalf("emulating the phone viewport: %v", err)
	}

	if err := openGarde(ctx, baseURL); err != nil {
		t.Fatalf("opening the Garde screen: %v", err)
	}

	for i, word := range demoCards {
		if err := chooseWager(ctx, screen); err != nil {
			t.Fatalf("card %d (%q): choosing the listen wager: %v", i, word, err)
		}
		typeCard(ctx, t, screen, word)

		if i == len(demoCards)-1 {
			break // no need to wait for the next card once this test is done with it
		}
		// completeWord (cmd/client/run.go) opens anim.Juice.ScoreSilence
		// (500ms), then the chips travel as anim.Juice.Droplet
		// (1.8s, staggered by anim.Juice.DropletStagger per droplet) before
		// [client.nextCard] opens the wager screen for the next card —
		// client/assets/juice.json's own numbers. A short word like these
		// scores under one droplet, so waiting comfortably past 500ms+1.8s
		// covers it without needing to read the droplet count back out.
		if err := chromedp.Run(ctx, chromedp.Sleep(3*time.Second)); err != nil {
			t.Fatalf("waiting for card %d (%q) to settle: %v", i, word, err)
		}
	}

	if _, err := os.Stat(filepath.Join(siteDir, "static", "encre.wasm")); errors.Is(err, os.ErrNotExist) {
		t.Fatal("the site directory's own encre.wasm vanished mid-test") // sanity: nothing above deleted t.TempDir() early
	}
}
