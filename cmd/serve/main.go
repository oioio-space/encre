// Command serve publishes the built WASM client over HTTP for the Étape 0
// device test of brief/ENCRE_05 ticket T00.
//
// It is the http.FileServer that ticket asks for and nothing more: the real
// static serving belongs to cmd/server behind Caddy (ENCRE_04 §1). It binds all
// interfaces on purpose, because the whole point is to open the page on a phone
// and a tablet that are not this machine.
package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	dir := flag.String("dir", "dist/web", "directory to serve")
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	// WebAssembly.instantiateStreaming refuses anything that is not
	// application/wasm, and Go's built-in table has been wrong about .wasm on
	// some systems; setting it is cheaper than debugging it on a phone.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		log.Fatalf("registering the wasm MIME type: %v", err)
	}

	if _, err := os.Stat(*dir); err != nil {
		log.Fatalf("nothing to serve: %v (run `mise run wasm:build` first)", err)
	}

	for _, host := range addresses() {
		fmt.Printf("  http://%s%s\n", host, *addr)
	}
	// An explicit server rather than http.ListenAndServe: a handler with no
	// read timeout holds a connection open for as long as a client cares to
	// dawdle, and this one is meant to be reachable from every phone on the
	// local network. The write timeout is generous because the other end is
	// pulling ~18 MB of WebAssembly over whatever link it has.
	srv := &http.Server{
		Addr:              *addr,
		Handler:           http.FileServer(http.Dir(*dir)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
	}
	log.Fatal(srv.ListenAndServe())
}

// addresses lists the host's routable addresses so the URL can be typed into a
// phone without looking it up.
func addresses() []string {
	hosts := []string{"localhost"}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return hosts
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			hosts = append(hosts, ipnet.IP.String())
		}
	}
	return hosts
}
