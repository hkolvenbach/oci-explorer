package docshandler

import (
	"bytes"
	htmltemplate "html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// Handler serves documentation files from an embedded filesystem.
type Handler struct {
	docsFS  fs.FS
	verbose bool
	md      goldmark.Markdown
}

// New creates a new documentation handler.
func New(docsFS fs.FS, verbose bool) *Handler {
	return &Handler{
		docsFS:  docsFS,
		verbose: verbose,
		md: goldmark.New(
			goldmark.WithExtensions(extension.Table),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
	}
}

func (h *Handler) logVerbose(format string, args ...interface{}) {
	if h.verbose {
		log.Printf("[VERBOSE] "+format, args...)
	}
}

// ServeOpenAPISpec serves the OpenAPI specification YAML file.
func (h *Handler) ServeOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	h.logVerbose("OpenAPI spec requested from %s", r.RemoteAddr)
	content, err := fs.ReadFile(h.docsFS, "docs/openapi.yaml")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(content)
}

// ServeDocs serves documentation files from the embedded docs folder.
func (h *Handler) ServeDocs(w http.ResponseWriter, r *http.Request) {
	// Get the requested path after /docs/
	path := strings.TrimPrefix(r.URL.Path, "/docs/")

	// Default to api.md for root path or index.html request
	if path == "" || path == "index.html" {
		path = "api.md"
	}

	h.logVerbose("Docs request: %s -> %s", r.URL.Path, path)

	// Read the file from embedded filesystem
	filePath := filepath.Join("docs", path)
	content, err := fs.ReadFile(h.docsFS, filePath)
	if err != nil {
		h.logVerbose("Docs file not found: %s", filePath)
		http.NotFound(w, r)
		return
	}

	// Determine content type and potentially convert markdown
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md":
		// Convert markdown to HTML
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		htmlContent := h.markdownToHTML(content, path)
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

// markdownToHTML converts markdown content to a styled HTML page using goldmark.
func (h *Handler) markdownToHTML(markdown []byte, title string) string {
	var htmlBuf bytes.Buffer
	if err := h.md.Convert(markdown, &htmlBuf); err != nil {
		return string(markdown)
	}

	// Load HTML template from embedded filesystem
	tmplContent, err := fs.ReadFile(h.docsFS, "docs/template.html")
	if err != nil {
		return htmlBuf.String()
	}

	tmpl, err := htmltemplate.New("docs").Parse(string(tmplContent))
	if err != nil {
		return htmlBuf.String()
	}

	data := struct {
		Title   string
		Content htmltemplate.HTML
	}{
		Title:   strings.TrimSuffix(title, ".md"),
		Content: htmltemplate.HTML(htmlBuf.String()),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return htmlBuf.String()
	}

	return buf.String()
}
