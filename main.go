package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
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

	logVerbose("Initializing HTTP router...")
	r := mux.NewRouter()

	// API routes
	logVerbose("Registering API routes...")
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/inspect", handleInspect).Methods("GET", "OPTIONS")
	api.HandleFunc("/tags", handleListTags).Methods("GET", "OPTIONS")
	api.HandleFunc("/sbom", handleDownloadSBOM).Methods("GET", "OPTIONS")
	api.HandleFunc("/health", handleHealth).Methods("GET")
	api.HandleFunc("/openapi.yaml", handleOpenAPISpec).Methods("GET")
	logVerbose("  - GET /api/inspect")
	logVerbose("  - GET /api/tags")
	logVerbose("  - GET /api/sbom")
	logVerbose("  - GET /api/health")
	logVerbose("  - GET /api/openapi.yaml")

	// Serve documentation files at /docs/
	logVerbose("Setting up documentation file server...")
	logVerbose("  - GET /docs/")
	logVerbose("  - GET /docs/{file}")
	r.PathPrefix("/docs/").HandlerFunc(handleDocs)

	// Serve embedded web files
	logVerbose("Setting up embedded web file server...")
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	r.PathPrefix("/").Handler(http.FileServer(http.FS(webContent)))

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

func handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	logVerbose("OpenAPI spec requested from %s", r.RemoteAddr)
	content, err := docsFS.ReadFile("docs/openapi.yaml")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(content)
}

func handleInspect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		logVerbose("Inspect request rejected: missing image parameter")
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, APIResponse{
			Success: false,
			Error:   "image parameter is required",
		})
		return
	}

	log.Printf("Inspecting image: %s", imageRef)
	logVerbose("Request from: %s", r.RemoteAddr)

	client := registry.NewClient()
	imageInfo, err := client.InspectImage(imageRef)
	if err != nil {
		log.Printf("Error inspecting image: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	logVerbose("Successfully fetched image info for %s", imageRef)
	writeJSON(w, APIResponse{
		Success: true,
		Data:    imageInfo,
	})
}

func handleDownloadSBOM(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repository")
	digest := r.URL.Query().Get("digest")

	if repo == "" || digest == "" {
		logVerbose("SBOM download request rejected: missing parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "repository and digest parameters are required",
		})
		return
	}

	log.Printf("Downloading SBOM from %s@%s", repo, digest)
	logVerbose("Request from: %s", r.RemoteAddr)

	client := registry.NewClient()
	sbomData, contentType, err := client.FetchSBOMContent(repo, digest)
	if err != nil {
		log.Printf("Error fetching SBOM: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
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

func handleListTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	repo := r.URL.Query().Get("repository")
	if repo == "" {
		logVerbose("Tags request rejected: missing repository parameter")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "repository parameter is required",
		})
		return
	}

	logVerbose("Listing tags for repository: %s", repo)

	ref, err := name.NewRepository(repo)
	if err != nil {
		logVerbose("Invalid repository reference: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid repository: %v", err),
		})
		return
	}

	logVerbose("Fetching tags from registry...")
	tags, err := remote.List(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		logVerbose("Failed to list tags: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	logVerbose("Found %d tags for %s", len(tags), repo)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    tags,
	})
}

// handleDocs serves documentation files from the embedded docs folder
func handleDocs(w http.ResponseWriter, r *http.Request) {
	// Get the requested path after /docs/
	path := strings.TrimPrefix(r.URL.Path, "/docs/")

	// Default to api.md for root path or index.html request
	if path == "" || path == "index.html" {
		path = "api.md"
	}

	logVerbose("Docs request: %s -> %s", r.URL.Path, path)

	// Read the file from embedded filesystem
	filePath := filepath.Join("docs", path)
	content, err := docsFS.ReadFile(filePath)
	if err != nil {
		logVerbose("Docs file not found: %s", filePath)
		http.NotFound(w, r)
		return
	}

	// Determine content type and potentially convert markdown
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md":
		// Convert markdown to HTML
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		htmlContent := markdownToHTML(string(content), path)
		w.Write([]byte(htmlContent))
	case ".yaml", ".yml":
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.Write(content)
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(content)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)
	}
}

// markdownToHTML converts markdown content to a simple HTML page
func markdownToHTML(markdown string, title string) string {
	// Escape HTML in the content first
	content := html.EscapeString(markdown)

	// Convert markdown elements to HTML

	// Code blocks (triple backticks) - must be done before inline code
	codeBlockRe := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	content = codeBlockRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := codeBlockRe.FindStringSubmatch(match)
		lang := parts[1]
		code := parts[2]
		if lang != "" {
			return fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>", lang, code)
		}
		return fmt.Sprintf("<pre><code>%s</code></pre>", code)
	})

	// Inline code (single backticks)
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")
	content = inlineCodeRe.ReplaceAllString(content, "<code>$1</code>")

	// Headers (must process h6-h1 in order to avoid conflicts)
	h6Re := regexp.MustCompile("(?m)^######\\s+(.+)$")
	content = h6Re.ReplaceAllString(content, "<h6>$1</h6>")
	h5Re := regexp.MustCompile("(?m)^#####\\s+(.+)$")
	content = h5Re.ReplaceAllString(content, "<h5>$1</h5>")
	h4Re := regexp.MustCompile("(?m)^####\\s+(.+)$")
	content = h4Re.ReplaceAllString(content, "<h4>$1</h4>")
	h3Re := regexp.MustCompile("(?m)^###\\s+(.+)$")
	content = h3Re.ReplaceAllString(content, "<h3>$1</h3>")
	h2Re := regexp.MustCompile("(?m)^##\\s+(.+)$")
	content = h2Re.ReplaceAllString(content, "<h2>$1</h2>")
	h1Re := regexp.MustCompile("(?m)^#\\s+(.+)$")
	content = h1Re.ReplaceAllString(content, "<h1>$1</h1>")

	// Bold text
	boldRe := regexp.MustCompile("\\*\\*([^*]+)\\*\\*")
	content = boldRe.ReplaceAllString(content, "<strong>$1</strong>")

	// Italic text
	italicRe := regexp.MustCompile("\\*([^*]+)\\*")
	content = italicRe.ReplaceAllString(content, "<em>$1</em>")

	// Links (but not image links)
	linkRe := regexp.MustCompile("\\[([^\\]]+)\\]\\(([^)]+)\\)")
	content = linkRe.ReplaceAllString(content, "<a href=\"$2\">$1</a>")

	// Unordered list items
	ulRe := regexp.MustCompile("(?m)^-\\s+(.+)$")
	content = ulRe.ReplaceAllString(content, "<li>$1</li>")

	// Ordered list items
	olRe := regexp.MustCompile("(?m)^\\d+\\.\\s+(.+)$")
	content = olRe.ReplaceAllString(content, "<li>$1</li>")

	// Horizontal rule
	hrRe := regexp.MustCompile("(?m)^---+$")
	content = hrRe.ReplaceAllString(content, "<hr>")

	// Table support (basic)
	tableRowRe := regexp.MustCompile("(?m)^\\|(.+)\\|$")
	content = tableRowRe.ReplaceAllStringFunc(content, func(match string) string {
		// Check if it's a separator row
		if strings.Contains(match, "---") {
			return "" // Skip separator rows
		}
		cells := strings.Split(strings.Trim(match, "|"), "|")
		var row strings.Builder
		row.WriteString("<tr>")
		for _, cell := range cells {
			row.WriteString("<td>" + strings.TrimSpace(cell) + "</td>")
		}
		row.WriteString("</tr>")
		return row.String()
	})

	// Wrap consecutive table rows in table tags
	tableRe := regexp.MustCompile("(?s)(<tr>.*?</tr>\\s*)+")
	content = tableRe.ReplaceAllString(content, "<table>$0</table>")

	// Paragraphs - wrap non-HTML content in <p> tags
	lines := strings.Split(content, "\n")
	var result strings.Builder
	inParagraph := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			if inParagraph {
				result.WriteString("</p>\n")
				inParagraph = false
			}
			continue
		}

		// Check if line is already an HTML block element
		isBlockElement := strings.HasPrefix(trimmed, "<h") ||
			strings.HasPrefix(trimmed, "<pre") ||
			strings.HasPrefix(trimmed, "<table") ||
			strings.HasPrefix(trimmed, "<li") ||
			strings.HasPrefix(trimmed, "<hr") ||
			strings.HasPrefix(trimmed, "</")

		if isBlockElement {
			if inParagraph {
				result.WriteString("</p>\n")
				inParagraph = false
			}
			result.WriteString(line + "\n")
		} else {
			if !inParagraph {
				result.WriteString("<p>")
				inParagraph = true
			} else {
				result.WriteString(" ")
			}
			result.WriteString(trimmed)
		}
	}

	if inParagraph {
		result.WriteString("</p>\n")
	}

	// Build the HTML page
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - OCI Explorer Docs</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            max-width: 900px;
            margin: 0 auto;
            padding: 2rem;
            color: #333;
            background: #fafafa;
        }
        h1, h2, h3, h4, h5, h6 {
            color: #1a202c;
            margin-top: 1.5em;
            margin-bottom: 0.5em;
        }
        h1 { border-bottom: 2px solid #3182ce; padding-bottom: 0.3em; }
        h2 { border-bottom: 1px solid #e2e8f0; padding-bottom: 0.2em; }
        code {
            background: #e2e8f0;
            padding: 0.2em 0.4em;
            border-radius: 3px;
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.9em;
        }
        pre {
            background: #1a202c;
            color: #e2e8f0;
            padding: 1em;
            border-radius: 6px;
            overflow-x: auto;
        }
        pre code {
            background: none;
            padding: 0;
            color: inherit;
        }
        a { color: #3182ce; text-decoration: none; }
        a:hover { text-decoration: underline; }
        table {
            border-collapse: collapse;
            width: 100%%;
            margin: 1em 0;
        }
        th, td {
            border: 1px solid #e2e8f0;
            padding: 0.5em 1em;
            text-align: left;
        }
        th { background: #f7fafc; }
        li { margin: 0.3em 0; }
        hr { border: none; border-top: 1px solid #e2e8f0; margin: 2em 0; }
        .nav {
            background: #2d3748;
            padding: 1em;
            border-radius: 6px;
            margin-bottom: 2em;
        }
        .nav a { color: #90cdf4; margin-right: 1.5em; }
        .nav a:hover { color: #fff; }
    </style>
</head>
<body>
    <nav class="nav">
        <a href="/docs/">API Reference</a>
        <a href="/api/openapi.yaml">OpenAPI Spec</a>
        <a href="/">Back to App</a>
    </nav>
    %s
</body>
</html>`, strings.TrimSuffix(title, ".md"), result.String())
}
