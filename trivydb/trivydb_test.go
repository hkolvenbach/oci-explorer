package trivydb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3 implements s3API backed by an in-memory map.
type mockS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMockS3() *mockS3 {
	return &mockS3{objects: make(map[string][]byte)}
}

func (m *mockS3) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[*input.Key] = data
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[*input.Key]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (m *mockS3) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok
}

func TestTarExtractRoundTrip(t *testing.T) {
	// Create a source directory with files
	srcRoot := t.TempDir()
	subDir := filepath.Join(srcRoot, "db")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "trivy.db"), []byte("fake-db-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "metadata.json"), []byte(`{"Version":2}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Tar it
	data, err := tarDir(subDir, "db")
	if err != nil {
		t.Fatalf("tarDir: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("tarDir produced empty archive")
	}

	// Extract to a new directory
	destRoot := t.TempDir()
	if err := extractTar(data, destRoot); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	// Verify files exist with correct content
	got, err := os.ReadFile(filepath.Join(destRoot, "db", "trivy.db"))
	if err != nil {
		t.Fatalf("read extracted trivy.db: %v", err)
	}
	if string(got) != "fake-db-content" {
		t.Errorf("trivy.db content = %q, want %q", got, "fake-db-content")
	}

	got, err = os.ReadFile(filepath.Join(destRoot, "db", "metadata.json"))
	if err != nil {
		t.Fatalf("read extracted metadata.json: %v", err)
	}
	if string(got) != `{"Version":2}` {
		t.Errorf("metadata.json content = %q, want %q", got, `{"Version":2}`)
	}
}

func TestExtractTarPathTraversal(t *testing.T) {
	// Build a tar.gz with a path-traversal entry manually
	var buf bytes.Buffer
	if err := writeMaliciousTar(&buf); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	err := extractTar(buf.Bytes(), dest)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestUploadDBFrom(t *testing.T) {
	mock := newMockS3()
	cacheDir := t.TempDir()

	// Create a fake DB directory
	dbDir := filepath.Join(cacheDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "trivy.db"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := newWithClient(mock, "test-bucket", cacheDir, "/fake/trivy")

	err := mgr.uploadDBFrom(context.Background(), vulnDBKey, cacheDir, "db", "vuln-db")
	if err != nil {
		t.Fatalf("uploadDBFrom: %v", err)
	}

	if !mock.has(vulnDBKey) {
		t.Error("expected vuln-db key in S3 after upload")
	}
}

func TestDownloadFromS3NotFound(t *testing.T) {
	mock := newMockS3()
	mgr := newWithClient(mock, "test-bucket", t.TempDir(), "/fake/trivy")

	_, err := mgr.downloadFromS3(context.Background(), "nonexistent-key")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func TestUploadAndRestoreRoundTrip(t *testing.T) {
	mock := newMockS3()

	// Create source with DB files and upload
	srcDir := t.TempDir()
	dbDir := filepath.Join(srcDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "trivy.db"), []byte("roundtrip-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	srcMgr := newWithClient(mock, "test-bucket", srcDir, "/fake/trivy")
	if err := srcMgr.uploadDBFrom(context.Background(), vulnDBKey, srcDir, "db", "vuln-db"); err != nil {
		t.Fatalf("upload: %v", err)
	}

	// Download to a new location and extract
	destDir := t.TempDir()
	destMgr := newWithClient(mock, "test-bucket", destDir, "/fake/trivy")
	data, err := destMgr.downloadFromS3(context.Background(), vulnDBKey)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if err := extractTar(data, destDir); err != nil {
		t.Fatalf("extract: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "db", "trivy.db"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(got) != "roundtrip-data" {
		t.Errorf("restored content = %q, want %q", got, "roundtrip-data")
	}
}

func TestIsSubpath(t *testing.T) {
	tests := []struct {
		base, target string
		want         bool
	}{
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/b/c/d", true},
		{"/a/b", "/a/c", false},
		{"/a/b", "/a", false},
	}
	for _, tt := range tests {
		got := isSubpath(tt.base, tt.target)
		if got != tt.want {
			t.Errorf("isSubpath(%q, %q) = %v, want %v", tt.base, tt.target, got, tt.want)
		}
	}
}

func TestManagerReadyAndDBAge(t *testing.T) {
	mgr := newWithClient(newMockS3(), "bucket", t.TempDir(), "/fake/trivy")

	if mgr.Ready() {
		t.Error("expected Ready()=false before Start")
	}
	if mgr.DBAge() != 0 {
		t.Errorf("expected DBAge()=0 before Start, got %v", mgr.DBAge())
	}
}

// writeMaliciousTar creates a tar.gz with a "../escape" path entry.
func writeMaliciousTar(w *bytes.Buffer) error {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	header := &tar.Header{
		Name: "../../../etc/evil",
		Mode: 0o644,
		Size: 4,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write([]byte("evil")); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}
