package docshandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() *Handler {
	fs := fstest.MapFS{
		"docs/api.md":        {Data: []byte("# API Reference\n\nSome content here.")},
		"docs/openapi.yaml":  {Data: []byte("openapi: 3.0.3\ninfo:\n  title: Test API")},
		"docs/data.json":     {Data: []byte(`{"key": "value"}`)},
		"docs/readme.txt":    {Data: []byte("plain text content")},
	}
	return &Handler{docsFS: fs, verbose: false}
}

func TestMarkdownToHTML_Headers(t *testing.T) {
	h := testFS()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"h1", "# Hello World", "<h1>Hello World</h1>"},
		{"h2", "## Section Two", "<h2>Section Two</h2>"},
		{"h3", "### Sub Section", "<h3>Sub Section</h3>"},
		{"h4", "#### Deep Section", "<h4>Deep Section</h4>"},
		{"h5", "##### Deeper", "<h5>Deeper</h5>"},
		{"h6", "###### Deepest", "<h6>Deepest</h6>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.markdownToHTML(tt.input, "test.md")
			if !strings.Contains(result, tt.contains) {
				t.Errorf("markdownToHTML(%q) does not contain %q", tt.input, tt.contains)
			}
		})
	}
}

func TestMarkdownToHTML_CodeBlocks(t *testing.T) {
	h := testFS()

	input := "```go\nfmt.Println(\"hello\")\n```"
	result := h.markdownToHTML(input, "test.md")

	if !strings.Contains(result, `<pre><code class="language-go">`) {
		t.Error("Expected code block with language class")
	}

	// Test inline code
	input2 := "Use `fmt.Println` to print"
	result2 := h.markdownToHTML(input2, "test.md")
	if !strings.Contains(result2, "<code>fmt.Println</code>") {
		t.Error("Expected inline code")
	}
}

func TestMarkdownToHTML_Links(t *testing.T) {
	h := testFS()

	input := "[Click here](https://example.com)"
	result := h.markdownToHTML(input, "test.md")

	if !strings.Contains(result, `<a href="https://example.com">Click here</a>`) {
		t.Errorf("Expected link in output, got: %s", result)
	}
}

func TestMarkdownToHTML_Tables(t *testing.T) {
	h := testFS()

	input := "| Name | Value |\n|------|-------|\n| foo  | bar   |"
	result := h.markdownToHTML(input, "test.md")

	if !strings.Contains(result, "<table>") {
		t.Error("Expected table tag in output")
	}
	if !strings.Contains(result, "<td>") {
		t.Error("Expected td tags in output")
	}
}

func TestMarkdownToHTML_Title(t *testing.T) {
	h := testFS()

	result := h.markdownToHTML("# Test", "api.md")
	if !strings.Contains(result, "<title>api - OCI Explorer Docs</title>") {
		t.Error("Expected title with .md suffix stripped")
	}
}

func TestServeDocs_MarkdownFile(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/docs/api.md", nil)
	w := httptest.NewRecorder()

	h.ServeDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Expected text/html content type, got %s", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<h1>") {
		t.Error("Expected HTML conversion of markdown")
	}
}

func TestServeDocs_YAMLFile(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/docs/openapi.yaml", nil)
	w := httptest.NewRecorder()

	h.ServeDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/yaml; charset=utf-8" {
		t.Errorf("Expected text/yaml content type, got %s", ct)
	}
}

func TestServeDocs_JSONFile(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/docs/data.json", nil)
	w := httptest.NewRecorder()

	h.ServeDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Expected application/json content type, got %s", ct)
	}
}

func TestServeDocs_PlainTextFile(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/docs/readme.txt", nil)
	w := httptest.NewRecorder()

	h.ServeDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Expected text/plain content type, got %s", ct)
	}
}

func TestServeDocs_NotFound(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/docs/nonexistent.md", nil)
	w := httptest.NewRecorder()

	h.ServeDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

func TestServeDocs_DefaultPath(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/docs/", nil)
	w := httptest.NewRecorder()

	h.ServeDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for default path, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Expected text/html for default api.md, got %s", ct)
	}
}

func TestServeOpenAPISpec(t *testing.T) {
	h := testFS()

	req := httptest.NewRequest("GET", "/api/openapi.yaml", nil)
	w := httptest.NewRecorder()

	h.ServeOpenAPISpec(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/yaml; charset=utf-8" {
		t.Errorf("Expected text/yaml content type, got %s", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "openapi: 3.0.3") {
		t.Error("Expected OpenAPI content")
	}
}

func TestServeOpenAPISpec_NotFound(t *testing.T) {
	fs := fstest.MapFS{}
	h := &Handler{docsFS: fs, verbose: false}

	req := httptest.NewRequest("GET", "/api/openapi.yaml", nil)
	w := httptest.NewRecorder()

	h.ServeOpenAPISpec(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 when openapi.yaml missing, got %d", resp.StatusCode)
	}
}
