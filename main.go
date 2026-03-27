package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
	"github.com/hkolvenbach/oci-explorer/cache"
	"github.com/hkolvenbach/oci-explorer/docshandler"
	"github.com/hkolvenbach/oci-explorer/registry"
	"github.com/hkolvenbach/oci-explorer/scanner"
)

//go:embed web/dist/*
var webFS embed.FS

//go:embed docs/*
var docsFS embed.FS

// Version is set at build time
var Version = "dev"

// Global verbose flag
var verbose bool
var jsonLogs bool

// cacheStore is the global response cache (nil when caching is disabled).
var cacheStore *cache.Store

// Cache TTLs per endpoint type.
const (
	inspectCacheTTL = 7 * 24 * time.Hour  // 7 days -- immutable for a given digest
	scanCacheTTL    = 24 * time.Hour       // 24 hours -- Trivy DB updates daily
	sbomCacheTTL    = 30 * 24 * time.Hour  // 30 days -- content-addressed, immutable
	vexCacheTTL     = 30 * 24 * time.Hour  // 30 days -- content-addressed, immutable
)

// APIResponse is the standard API response format
type APIResponse struct {
	Success  bool        `json:"success"`
	Data     interface{} `json:"data,omitempty"`
	Error    string      `json:"error,omitempty"`
	CachedAt string      `json:"cachedAt,omitempty"`
}

// logVerbose logs at debug level (visible only in verbose mode).
func logVerbose(format string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(format, args...))
}

// writeJSON writes a JSON response to the http.ResponseWriter
func writeJSON(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("json encode failed", "error", err)
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
		slog.Error("write response failed", "error", err)
	}
}

func main() {
	// Parse command line flags
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&verbose, "v", false, "Enable verbose logging (shorthand)")
	flag.BoolVar(&jsonLogs, "json-logs", false, "Output logs in JSON format (for structured log shipping)")
	port := flag.String("port", "", "HTTP server port (default: 8080, or PORT env var)")
	flag.Parse()

	// Env var fallback for JSON logs
	if !jsonLogs && os.Getenv("LOG_FORMAT") == "json" {
		jsonLogs = true
	}

	// Configure structured logging
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	var logHandler slog.Handler
	if jsonLogs {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	}
	slog.SetDefault(slog.New(logHandler))

	// Determine port
	serverPort := *port
	if serverPort == "" {
		serverPort = os.Getenv("PORT")
	}
	if serverPort == "" {
		serverPort = "8080"
	}

	if verbose {
		slog.Debug("verbose mode enabled")
		slog.Debug("startup", "version", Version, "platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	}

	// Set verbose mode in registry client and scanner
	registry.SetVerbose(verbose)
	scanner.SetVerbose(verbose)

	// Configure Docker Hub authentication if credentials are provided.
	// Raises rate limit from 100 pulls/6h (anonymous) to 200/6h (free) or unlimited (paid).
	if token := os.Getenv("DOCKER_HUB_TOKEN"); token != "" {
		user := os.Getenv("DOCKER_HUB_USER")
		registry.ConfigureDockerHubAuth(user, token)
		slog.Info("Docker Hub authentication configured", "user", user)
	}

	// Initialize S3-backed response cache if CACHE_S3_BUCKET is set
	if bucket := os.Getenv("CACHE_S3_BUCKET"); bucket != "" {
		var err error
		cacheStore, err = cache.New(context.Background(), bucket)
		if err != nil {
			log.Fatalf("Failed to initialize cache: %v", err)
		}
		slog.Info("response cache enabled", "bucket", bucket)
	}

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
	api.HandleFunc("/scan", handleScanImage).Methods("GET", "OPTIONS")
	api.HandleFunc("/health", handleHealth).Methods("GET")
	api.HandleFunc("/openapi.yaml", docsHandler.ServeOpenAPISpec).Methods("GET")
	logVerbose("  - GET /api/inspect")
	logVerbose("  - GET /api/tags")
	logVerbose("  - GET /api/matching-tags")
	logVerbose("  - GET /api/sbom")
	logVerbose("  - GET /api/vex")
	logVerbose("  - GET /api/scan")
	logVerbose("  - GET /api/health")
	logVerbose("  - GET /api/openapi.yaml")

	// Serve documentation files at /docs/
	logVerbose("Setting up documentation file server...")
	logVerbose("  - GET /docs/")
	logVerbose("  - GET /docs/{file}")
	r.PathPrefix("/docs/").HandlerFunc(docsHandler.ServeDocs)

	// Serve embedded web files with cache-busting headers for HTML
	logVerbose("Setting up embedded web file server...")
	webContent, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	webFileServer := http.FileServer(http.FS(webContent))
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Vite hashed assets (e.g. /assets/index-Ab12Cd.js) are immutable
		if len(req.URL.Path) > 8 && req.URL.Path[:8] == "/assets/" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// HTML and other files: prevent stale cache after binary upgrades
			w.Header().Set("Cache-Control", "no-cache")
		}
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
	if cacheStore != nil {
		fmt.Printf("│  Cache:    %-37s│\n", os.Getenv("CACHE_S3_BUCKET"))
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

	_, trivyErr := exec.LookPath("trivy")

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":         "healthy",
			"platform":       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			"version":        Version,
			"trivyAvailable": trivyErr == nil,
			"cacheEnabled":   cacheStore != nil,
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

	slog.Info("inspect", "image", imageRef)
	slog.Debug("inspect", "image", imageRef, "remote_addr", r.RemoteAddr)

	if cacheStore != nil {
		digest, err := registry.ResolveDigest(imageRef)
		if err == nil {
			result, err := cacheStore.GetOrCompute(r.Context(), "inspect/"+digest, inspectCacheTTL, func() ([]byte, error) {
				client := registry.NewClient()
				info, err := client.InspectImage(imageRef)
				if err != nil {
					return nil, err
				}
				return json.Marshal(APIResponse{Success: true, Data: info})
			})
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				if result.FromCache {
					w.Header().Set("X-Cache", "HIT")
				} else {
					w.Header().Set("X-Cache", "MISS")
				}
				writeBytes(w, result.Data)
				return
			}
			slog.Warn("cache path failed, falling through", "image", imageRef, "error", err)
		} else {
			slog.Warn("digest resolution failed, falling through", "image", imageRef, "error", err)
		}
	}

	// Uncached path (cache disabled or cache error)
	client := registry.NewClient()
	imageInfo, err := client.InspectImage(imageRef)
	if err != nil {
		slog.Error("inspect failed", "image", imageRef, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	slog.Debug("inspect complete", "image", imageRef)
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

	slog.Info("sbom download", "repository", repo, "digest", digest)
	slog.Debug("sbom download", "repository", repo, "digest", digest, "remote_addr", r.RemoteAddr)

	if cacheStore != nil {
		result, err := cacheStore.GetOrCompute(r.Context(), "sbom/"+digest, sbomCacheTTL, func() ([]byte, error) {
			client := registry.NewClient()
			sbomData, _, err := client.FetchSBOMContent(repo, digest)
			if err != nil {
				return nil, err
			}
			return sbomData, nil
		})
		if err == nil {
			contentType := "application/json"
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"sbom-%s.json\"", digest[7:19]))
			if result.FromCache {
				w.Header().Set("X-Cache", "HIT")
			} else {
				w.Header().Set("X-Cache", "MISS")
			}
			writeBytes(w, result.Data)
			return
		}
		slog.Warn("cache path failed, falling through", "digest", digest, "error", err)
	}

	// Uncached path
	client := registry.NewClient()
	sbomData, contentType, err := client.FetchSBOMContent(repo, digest)
	if err != nil {
		slog.Error("sbom download failed", "repository", repo, "digest", digest, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

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

	slog.Info("vex fetch", "repository", repo, "digest", digest)
	slog.Debug("vex fetch", "repository", repo, "digest", digest, "remote_addr", r.RemoteAddr)

	if cacheStore != nil {
		result, err := cacheStore.GetOrCompute(r.Context(), "vex/"+digest, vexCacheTTL, func() ([]byte, error) {
			client := registry.NewClient()
			vexDoc, err := client.FetchVEXContent(repo, digest)
			if err != nil {
				return nil, err
			}
			return json.Marshal(APIResponse{Success: true, Data: vexDoc})
		})
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			if result.FromCache {
				w.Header().Set("X-Cache", "HIT")
			} else {
				w.Header().Set("X-Cache", "MISS")
			}
			writeBytes(w, result.Data)
			return
		}
		slog.Warn("cache path failed, falling through", "digest", digest, "error", err)
	}

	// Uncached path
	client := registry.NewClient()
	vexDoc, err := client.FetchVEXContent(repo, digest)
	if err != nil {
		slog.Error("vex fetch failed", "repository", repo, "digest", digest, "error", err)
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
	tags, err := remote.List(ref, remote.WithAuthFromKeychain(registry.Keychain()))
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
//
//	Docker Hub (returns matching tags):
//	  curl 'http://localhost:8080/api/matching-tags?image=alpine:latest'
//	GCR / Artifact Registry (single-request lookup):
//	  curl 'http://localhost:8080/api/matching-tags?image=gcr.io/google-containers/pause:3.2'
//	GHCR (unsupported — returns note):
//	  curl 'http://localhost:8080/api/matching-tags?image=ghcr.io/hkolvenbach/oci-explorer:0.2.2'
func handleMatchingTags(w http.ResponseWriter, r *http.Request) {
	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		logVerbose("Matching tags request rejected: missing image parameter")
		writeBadRequest(w, "image parameter is required")
		return
	}

	slog.Info("matching-tags", "image", imageRef)
	slog.Debug("matching-tags", "image", imageRef, "remote_addr", r.RemoteAddr)

	client := registry.NewClient()
	result, err := client.GetMatchingTags(imageRef)
	if err != nil {
		slog.Error("matching-tags failed", "image", imageRef, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logVerbose("Found %d matching tags for %s", len(result.Tags), imageRef)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{Success: true, Data: result})
}

func handleScanImage(w http.ResponseWriter, r *http.Request) {
	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		logVerbose("Scan request rejected: missing image parameter")
		writeBadRequest(w, "image parameter is required")
		return
	}

	slog.Info("scan", "image", imageRef)
	slog.Debug("scan", "image", imageRef, "remote_addr", r.RemoteAddr)

	force := r.URL.Query().Get("force") == "1"

	if cacheStore != nil && !force {
		digest, err := registry.ResolveDigest(imageRef)
		if err == nil {
			result, err := cacheStore.GetOrCompute(r.Context(), "scan/"+digest, scanCacheTTL, func() ([]byte, error) {
				scanResult, err := scanner.ScanImage(r.Context(), imageRef)
				if err != nil {
					return nil, err
				}
				return json.Marshal(APIResponse{Success: true, Data: scanResult})
			})
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				if result.FromCache {
					w.Header().Set("X-Cache", "HIT")
				} else {
					w.Header().Set("X-Cache", "MISS")
				}
				// Inject cachedAt for the frontend to show staleness
				if !result.CachedAt.IsZero() {
					w.Header().Set("X-Cached-At", result.CachedAt.UTC().Format(time.RFC3339))
				}
				writeBytes(w, result.Data)
				return
			}
			slog.Warn("cache path failed, falling through", "image", imageRef, "error", err)
		} else {
			slog.Warn("digest resolution failed, falling through", "image", imageRef, "error", err)
		}
	}

	// Force refresh or uncached path -- also store in cache if available
	scanResult, err := scanner.ScanImage(r.Context(), imageRef)
	if err != nil {
		slog.Error("scan failed", "image", imageRef, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// On force refresh with cache enabled, store the fresh result
	if cacheStore != nil && force {
		if digest, err := registry.ResolveDigest(imageRef); err == nil {
			data, err := json.Marshal(APIResponse{Success: true, Data: scanResult})
			if err == nil {
				go func() {
					if setErr := cacheStore.Set(context.Background(), "scan/"+digest, data, scanCacheTTL); setErr != nil {
						slog.Warn("cache set failed after force rescan", "image", imageRef, "error", setErr)
					}
				}()
			}
		}
	}

	logVerbose("Scan complete for %s: %d vulnerabilities found", imageRef, scanResult.TotalCount)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, APIResponse{Success: true, Data: scanResult})
}
