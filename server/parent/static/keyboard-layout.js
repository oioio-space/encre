// keyboard-layout.js enables the AZERTY keyboard-layout radio button on the
// réglages page once the browser proves the device is not a narrow portrait
// phone — ENCRE_07 §4.2: ten AZERTY columns on a 390px-wide portrait phone
// give ~36px (~6mm) keys, under every documented touch-target floor. The
// server always renders that radio "disabled" (see settings.go); this is
// the only thing allowed to turn it back on, and only when the media query
// below actually matches.
(function () {
  "use strict";

  // Landscape orientation, or wide enough to be a tablet even in portrait
  // (iPad mini portrait is 768px): either one gives AZERTY's ten columns
  // enough width per key.
  var allowed = window.matchMedia(
    "(orientation: landscape), (min-width: 768px)"
  );

  function apply() {
    document.querySelectorAll('input[data-portrait-restricted]').forEach(function (input) {
      input.disabled = !allowed.matches;
      var note = document.getElementById(input.getAttribute("aria-describedby"));
      if (note) {
        note.hidden = allowed.matches;
      }
    });
  }

  if (allowed.addEventListener) {
    allowed.addEventListener("change", apply);
  }
  document.addEventListener("DOMContentLoaded", apply);
})();
