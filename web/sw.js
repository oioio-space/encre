// ENCRE's service worker (ENCRE_04 §2): caches the WASM client, wasm_exec.js
// and the loading page so a returning player gets an offline-capable first
// paint — the goutte shows while the browser's own HTTP cache (or, offline,
// this cache) serves the fetch, not a blank tab.
//
// __HASH__ is substituted by `mise run wasm:build` (see mise.toml's
// "wasm:build" task) with the content hash of that build's WASM binary, so
// CACHE_NAME changes on every release: [install] populates a fresh cache
// under the new name, and [activate] deletes every cache left over from an
// older one. This file itself is never cached (cmd/server serves it with
// Cache-Control: no-cache, see cmd/server/main.go's serveNoCache) so a
// returning player always fetches the current version of *this* script
// before it decides what else is stale.
const CACHE_NAME = "encre-__HASH__";

// PRECACHE is what [install] fetches eagerly. The two content-hashed
// /static files never change under this name (a new build gets a new hash,
// hence a new CACHE_NAME), so caching them here is safe to keep forever
// under this version; "/" is re-fetched on every navigation by [fetch]
// below and only falls back to the cached copy when offline, because it is
// the one entry point a stale copy could point at assets that no longer
// exist.
const PRECACHE = [
  "/",
  "/static/encre.__HASH__.wasm",
  "/static/wasm_exec.__HASH__.js",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(PRECACHE))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((names) => Promise.all(
        names.filter((name) => name !== CACHE_NAME).map((name) => caches.delete(name)),
      ))
      .then(() => self.clients.claim()),
  );
});

// fetch answers every navigation and asset request with the network first —
// so a player with a connection always gets the current build — and falls
// back to this version's cache only when the network is unavailable, which
// is the offline-during-a-run case ENCRE_04 §1 asks for.
self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") {
    return;
  }
  event.respondWith(
    fetch(event.request).catch(() => caches.match(event.request)),
  );
});
