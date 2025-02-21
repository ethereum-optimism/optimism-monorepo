package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
)

var (
	flagAppRoot    = flag.String("app-root", "/app", "Root directory for the application")
	flagConfigsDir = flag.String("configs-dir", "chainconfig/configs", "Directory for config files (relative to build-dir)")
	flagBuildDir   = flag.String("build-dir", "op-program", "Directory where the build command will be executed (relative to app-root)")
	flagBuildCmd   = flag.String("build-cmd", "just build", "Build command to execute")
	flagPort       = flag.Int("port", 8080, "Port to listen on")
)

func main() {
	flag.Parse()

	srv := createServer()

	// Set up routes
	http.HandleFunc("/", srv.handleUpload)
	http.Handle("/proofs/", http.StripPrefix("/proofs/", http.FileServer(srv.proofFS)))

	log.Printf("Starting server on :%d with:", srv.port)
	log.Printf("  app-root: %s", srv.appRoot)
	log.Printf("  configs-dir: %s", filepath.Join(srv.appRoot, srv.buildDir, srv.configsDir))
	log.Printf("  build-dir: %s", filepath.Join(srv.appRoot, srv.buildDir))
	log.Printf("  build-cmd: %s", srv.buildCmd)
	log.Printf("  proofs-dir: %s", filepath.Join(srv.appRoot, srv.buildDir, "bin"))

	if err := http.ListenAndServe(fmt.Sprintf(":%d", srv.port), nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
