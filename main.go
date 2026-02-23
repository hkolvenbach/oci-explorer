package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
	"github.com/hkolvenbach/oci-explorer/docshandler"
	"github.com/hkolvenbach/oci-explorer/registry"
)

//go:embed web/*
var webFS embed.FS

//go:embed docs/*
var docsFS embed.FS

// Version is set at build time
var Version = "dev"

// Global verbose flag
var verbose bool

// APIResponse is the standard API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// logVerbose prints a message only if verbose mode is enabled
func logVerbose(format string, args ...interface{}) {
	if verbose {
		log.Printf("[VERBOSE] "+format, args...)
	}
}

// writeJSON writes a JSON response to the http.ResponseWriter
func writeJSON(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, APIResponse{Success: false, Error: msg})
}

// writeBadRequest writes a 400 Bad Request JSON error response.
func writeBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, msg)
}

// writeBytes writes bytes to the http.ResponseWriter
func writeBytes(w http.ResponseWriter, data []byte) {
	if _, err := w.Write(data); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func main() {
	// Parse command line flags
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&verbose, "v", false, "Enable verbose logging (shorthand)")
	port := flag.String("port", "", "HTTP server port (default: 8080, or PORT env var)")
	flag.Parse()

	// Determine port
	serverPort := *port
	if serverPort == "" {
		serverPort = os.Getenv("PORT")
	}
	if serverPort == "" {
		serverPort = "8080"
	}

	if verbose {
		log.Println("[VERBOSE] Verbose mode enabled")
		log.Printf("[VERBOSE] Version: %s", Version)
		log.Printf("[VERBOSE] Platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Set verbose mode in registry client
	registry.SetVerbose(verbose)

	// Create docs handler (embed.FS satisfies fs.FS)
	docsHandler := docshandler.New(docsFS, verbose)

	logVerbose("Initializing HTTP router...")
	r := mux.NewRouter()

	// API routes
	logVerbose("Registering API routes...")
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/inspect", handleInspect).Methods("GET", "OPTIONS")
	api.HandleFunc("/tags", handleListTags).Methods("GET", "OPTIONS")
	api.HandleFunc("/matching-tags", handleMatchingTags).Methods("GET", "OPTIONS")
	api.HandleFunc("/sbom", handleDownloadSBOM).Methods("GET", "OPTIONS")
	api.HandleFunc("/vex", handleFetchVEX).Methods("GET", "OPTIONS")
	api.HandleFunc("/health", handleHealth).Methods("GET")
	api.HandleFunc("/openapi.yaml", docsHandler.ServeOpenAPISpec).Methods("GET")
	logVerbose("  - GET /api/inspect")
	logVerbose("  - GET /api/tags")
	logVerbose("  - GET /api/matching-tags")
	logVerbose("  - GET /api/sbom")
	logVerbose("  - GET /api/vex")
	logVerbose("  - GET /api/health")
	logVerbose("  - GET /api/openapi.yaml")

	// Serve documentation files at /docs/
	logVerbose("Setting up documentation file server...")
	logVerbose("  - GET /docs/")
	logVerbose("  - GET /docs/{file}")
	r.PathPrefix("/docs/").HandlerFunc(docsHandler.ServeDocs)

	// Serve embedded web files with cache-busting headers for HTML
	logVerbose("Setting up embedded web file server...")
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	webFileServer := http.FileServer(http.FS(webContent))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Prevent browsers from caching stale HTML after binary upgrades
		w.Header().Set("Cache-Control", "no-cache")
		webFileServer.ServeHTTP(w, req)
	})

	// CORS middleware
	logVerbose("Applying CORS middleware...")
	handler := corsMiddleware(r)

	fmt.Println("┌─────────────────────────────────────────────────┐")
	fmt.Println("│           🐳 OCI Image Explorer                 │")
	fmt.Println("├─────────────────────────────────────────────────┤")
	fmt.Printf("│  URL:      http://localhost:%-20s│\n", serverPort)
	fmt.Printf("│  Platform: %-37s│\n", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	fmt.Printf("│  Version:  %-37s│\n", Version)
	if verbose {
		fmt.Println("│  Mode:     verbose                              │")
	}
	fmt.Println("│  Press Ctrl+C to stop                           │")
	fmt.Println("└─────────────────────────────────────────────────┘")

	logVerbose("Starting HTTP server on port %s...", serverPort)
	if err := http.ListenAndServe(":"+serverPort, handler); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	logVerbose("Health check requested from %s", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]string{
			"status":   "healthy",
			"platform": fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			"version":  Version,
		},
	})
}


func handleInspect(w http.ResponseWriter, r *http.Request) {
	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		logVerbose("Inspect request rejected: missing image parameter")
		writeBadRequest(w, "image parameter is required")
		return
	}

	log.Printf("Inspecting image: %s", imageRef)
	logVerbose("Request from: %s", r.RemoteAddr)

	client := registry.NewClient()
	imageInfo, err := client.InspectImage(imageRef)
	if err != nil {
		log.Printf("Error inspecting image: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logVerbose("Successfully fetched image info for %s", imageRef)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{Success: true, Data: imageInfo})
}

func handleDownloadSBOM(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repository")
	digest := r.URL.Query().Get("digest")

	if repo == "" || digest == "" {
		logVerbose("SBOM download request rejected: missing parameters")
		writeBadRequest(w, "repository and digest parameters are required")
		return
	}

	log.Printf("Downloading SBOM from %s@%s", repo, digest)
	logVerbose("Request from: %s", r.RemoteAddr)

	client := registry.NewClient()
	sbomData, contentType, err := client.FetchSBOMContent(repo, digest)
	if err != nil {
		log.Printf("Error fetching SBOM: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Set appropriate content type and disposition for download
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"sbom-%s.json\"", digest[7:19]))
	w.Write(sbomData)
}

func handleFetchVEX(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repository")
	digest := r.URL.Query().Get("digest")

	if repo == "" || digest == "" {
		logVerbose("VEX request rejected: missing parameters")
		writeBadRequest(w, "repository and digest parameters are required")
		return
	}

	log.Printf("Fetching VEX from %s@%s", repo, digest)
	logVerbose("Request from: %s", r.RemoteAddr)

	client := registry.NewClient()
	vexDoc, err := client.FetchVEXContent(repo, digest)
	if err != nil {
		log.Printf("Error fetching VEX: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{Success: true, Data: vexDoc})
}

func handleListTags(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repository")
	if repo == "" {
		logVerbose("Tags request rejected: missing repository parameter")
		writeBadRequest(w, "repository parameter is required")
		return
	}

	logVerbose("Listing tags for repository: %s", repo)

	ref, err := name.NewRepository(repo)
	if err != nil {
		logVerbose("Invalid repository reference: %v", err)
		writeBadRequest(w, fmt.Sprintf("invalid repository: %v", err))
		return
	}

	logVerbose("Fetching tags from registry...")
	tags, err := remote.List(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		logVerbose("Failed to list tags: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logVerbose("Found %d tags for %s", len(tags), repo)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{Success: true, Data: tags})
}

// handleMatchingTags returns tags that share the same digest as the queried image.
//
// Test examples:
//   Docker Hub (returns matching tags):
//     curl 'http://localhost:8080/api/matching-tags?image=alpine:latest'
//   GCR / Artifact Registry (single-request lookup):
//     curl 'http://localhost:8080/api/matching-tags?image=gcr.io/google-containers/pause:3.2'
//   GHCR (unsupported — returns note):
//     curl 'http://localhost:8080/api/matching-tags?image=ghcr.io/hkolvenbach/oci-explorer:0.2.2'
func handleMatchingTags(w http.ResponseWriter, r *http.Request) {
	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		logVerbose("Matching tags request rejected: missing image parameter")
		writeBadRequest(w, "image parameter is required")
		return
	}

	log.Printf("Looking up matching tags for: %s", imageRef)
	logVerbose("Request from: %s", r.RemoteAddr)

	client := registry.NewClient()
	result, err := client.GetMatchingTags(imageRef)
	if err != nil {
		log.Printf("Error looking up matching tags: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logVerbose("Found %d matching tags for %s", len(result.Tags), imageRef)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{Success: true, Data: result})
}

