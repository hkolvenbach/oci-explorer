package docshandler

import (
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

// Handler serves documentation files from an embedded filesystem.
type Handler struct {
	docsFS  fs.FS
	verbose bool
}

// New creates a new documentation handler.
func New(docsFS fs.FS, verbose bool) *Handler {
	return &Handler{
		docsFS:  docsFS,
		verbose: verbose,
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
		htmlContent := h.markdownToHTML(string(content), path)
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

// markdownToHTML converts markdown content to a simple HTML page.
func (h *Handler) markdownToHTML(markdown string, title string) string {
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
