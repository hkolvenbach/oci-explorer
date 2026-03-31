// Package trivydb caches the Trivy vulnerability and Java databases in S3
// to eliminate the 30-60s cold-start penalty when Trivy downloads from upstream.
//
// On startup the Manager restores the databases from S3. A background goroutine
// refreshes from Trivy's upstream every hour and uploads the new databases to S3
// so subsequent cold starts are fast.
package trivydb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	vulnDBKey = "trivy-db/vuln-db.tar.gz"
	javaDBKey = "trivy-db/java-db.tar.gz"

	refreshInterval = 6 * time.Hour
	trivyTimeout    = 10 * time.Minute // java-db OCI artifact is ~800 MB
)

// s3API is the subset of the S3 client used by Manager, enabling test mocks.
type s3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// Manager handles the Trivy DB lifecycle: restore from S3, periodic refresh,
// and upload back to S3.
type Manager struct {
	client    s3API
	bucket    string
	cacheDir  string
	trivyPath string
	trivySem  chan struct{} // shared with scanner to prevent concurrent Trivy processes

	ready      atomic.Bool
	hasVulnDB  atomic.Bool  // true if vuln DB was successfully restored
	hasJavaDB  atomic.Bool  // true if java DB was successfully restored
	lastUpdate atomic.Int64 // unix timestamp of last successful refresh

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a Manager backed by the given S3 bucket.
// cacheDir is the directory Trivy will use as its --cache-dir.
func New(ctx context.Context, bucket, cacheDir string, trivySem chan struct{}) (*Manager, error) {
	trivyPath, err := exec.LookPath("trivy")
	if err != nil {
		return nil, fmt.Errorf("trivydb: trivy not found: %w", err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("trivydb: load AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("trivydb: create cache dir: %w", err)
	}

	return &Manager{
		client:    client,
		bucket:    bucket,
		cacheDir:  cacheDir,
		trivyPath: trivyPath,
		trivySem:  trivySem,
	}, nil
}

// newWithClient creates a Manager with an injected S3 client (for testing).
func newWithClient(client s3API, bucket, cacheDir, trivyPath string) *Manager {
	return &Manager{
		client:    client,
		bucket:    bucket,
		cacheDir:  cacheDir,
		trivyPath: trivyPath,
	}
}

// Start restores databases from S3 (falling back to upstream) and launches the
// hourly refresh goroutine. It blocks until the initial restore is complete.
func (m *Manager) Start(ctx context.Context) error {
	refreshCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	if err := m.initialRestore(ctx); err != nil {
		cancel()
		return err
	}

	m.ready.Store(true)
	m.lastUpdate.Store(time.Now().Unix())

	m.wg.Add(1)
	go m.refreshLoop(refreshCtx)

	return nil
}

// Stop cancels the background refresh and waits for it to finish.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

// CacheDir returns the Trivy cache directory managed by this Manager.
func (m *Manager) CacheDir() string {
	return m.cacheDir
}

// Ready returns true once the initial DB restore is complete.
func (m *Manager) Ready() bool {
	return m.ready.Load()
}

// HasVulnDB returns true if the vulnerability DB is available locally.
func (m *Manager) HasVulnDB() bool {
	return m.hasVulnDB.Load()
}

// HasJavaDB returns true if the Java DB is available locally.
func (m *Manager) HasJavaDB() bool {
	return m.hasJavaDB.Load()
}

// DBAge returns the time since the last successful DB refresh.
func (m *Manager) DBAge() time.Duration {
	ts := m.lastUpdate.Load()
	if ts == 0 {
		return 0
	}
	return time.Since(time.Unix(ts, 0))
}

// initialRestore tries to restore both DBs from S3. If a key is missing,
// it downloads from Trivy's upstream and uploads to S3 for next time.
func (m *Manager) initialRestore(ctx context.Context) error {
	type dbSpec struct {
		s3Key   string
		subDir  string
		dlFlag  string
		logName string
		ready   *atomic.Bool // set to true when this DB is available
	}
	dbs := []dbSpec{
		{vulnDBKey, "db", "--download-db-only", "vuln-db", &m.hasVulnDB},
		{javaDBKey, "java-db", "--download-java-db-only", "java-db", &m.hasJavaDB},
	}

	for _, db := range dbs {
		destDir := filepath.Join(m.cacheDir, db.subDir)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("trivydb: create %s dir: %w", db.subDir, err)
		}

		start := time.Now()
		rc, err := m.downloadFromS3(ctx, db.s3Key)
		if err == nil {
			err = extractTar(rc, m.cacheDir)
			rc.Close()
			if err != nil {
				slog.Warn("trivydb: S3 restore extract failed, falling back to upstream",
					"db", db.logName, "error", err)
			} else {
				slog.Info("trivydb: restored from S3",
					"db", db.logName, "duration", time.Since(start).Round(time.Millisecond))
				db.ready.Store(true)
				continue
			}
		} else {
			slog.Info("trivydb: S3 miss, downloading from upstream", "db", db.logName)
		}

		// Fall back to upstream
		start = time.Now()
		if err := m.trivyDownload(ctx, db.dlFlag); err != nil {
			slog.Warn("trivydb: upstream download failed",
				"db", db.logName, "error", err)
			continue // non-fatal: Trivy will download on first scan
		}
		slog.Info("trivydb: downloaded from upstream",
			"db", db.logName, "duration", time.Since(start).Round(time.Millisecond))
		db.ready.Store(true)

		// Upload to S3 for next cold start
		go m.uploadDB(context.Background(), db.s3Key, db.subDir, db.logName)
	}

	return nil
}

// refreshLoop runs on an hourly ticker, downloading fresh DBs from upstream
// and uploading them to S3.
func (m *Manager) refreshLoop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

// refresh downloads fresh DBs to a temp directory, uploads to S3, then swaps
// them into the live cache directory.
func (m *Manager) refresh(ctx context.Context) {
	slog.Info("trivydb: hourly refresh starting")
	start := time.Now()

	tmpDir, err := os.MkdirTemp("", "trivy-refresh-*")
	if err != nil {
		slog.Error("trivydb: refresh failed to create temp dir", "error", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	type dbSpec struct {
		s3Key   string
		subDir  string
		dlFlag  string
		logName string
	}
	dbs := []dbSpec{
		{vulnDBKey, "db", "--download-db-only", "vuln-db"},
		{javaDBKey, "java-db", "--download-java-db-only", "java-db"},
	}

	// Download fresh DBs to temp dir
	for _, db := range dbs {
		release, err := m.acquireSem(ctx)
		if err != nil {
			slog.Error("trivydb: refresh cancelled waiting for semaphore", "error", err)
			return
		}
		dlCtx, cancel := context.WithTimeout(ctx, trivyTimeout)
		cmd := exec.CommandContext(dlCtx, m.trivyPath, "image", db.dlFlag, "--cache-dir", tmpDir)
		cmd.Env = envWithoutTrivyCreds()
		if output, err := cmd.CombinedOutput(); err != nil {
			cancel()
			release()
			slog.Error("trivydb: refresh download failed",
				"db", db.logName, "error", err, "output", string(output))
			return // don't partially update
		}
		cancel()
		release()
	}

	// Upload to S3 and swap into live dir
	for _, db := range dbs {
		if err := m.uploadDBFrom(ctx, db.s3Key, tmpDir, db.subDir, db.logName); err != nil {
			slog.Error("trivydb: refresh upload failed", "db", db.logName, "error", err)
			// continue anyway — swap the local copy even if S3 upload failed
		}

		// Acquire semaphore during swap to prevent scans from reading
		// partially-swapped DB files.
		release, err := m.acquireSem(ctx)
		if err != nil {
			slog.Error("trivydb: refresh swap cancelled", "error", err)
			continue
		}

		liveDir := filepath.Join(m.cacheDir, db.subDir)
		tmpSubDir := filepath.Join(tmpDir, db.subDir)

		if err := os.RemoveAll(liveDir); err != nil {
			slog.Error("trivydb: refresh remove old dir failed", "db", db.logName, "error", err)
			release()
			continue
		}
		if err := os.Rename(tmpSubDir, liveDir); err != nil {
			slog.Error("trivydb: refresh swap failed", "db", db.logName, "error", err)
			release()
			continue
		}
		release()
	}

	m.lastUpdate.Store(time.Now().Unix())
	slog.Info("trivydb: hourly refresh complete", "duration", time.Since(start).Round(time.Millisecond))
}

// trivyDownload runs trivy with the given download flag against the manager's cache dir.
func (m *Manager) trivyDownload(ctx context.Context, flag string) error {
	release, err := m.acquireSem(ctx)
	if err != nil {
		return fmt.Errorf("acquire trivy semaphore: %w", err)
	}
	defer release()

	dlCtx, cancel := context.WithTimeout(ctx, trivyTimeout)
	defer cancel()
	cmd := exec.CommandContext(dlCtx, m.trivyPath, "image", flag, "--cache-dir", m.cacheDir)
	cmd.Env = envWithoutTrivyCreds()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

// acquireSem acquires the shared Trivy subprocess semaphore to prevent
// concurrent Trivy processes from OOMing on memory-constrained hosts.
func (m *Manager) acquireSem(ctx context.Context) (func(), error) {
	if m.trivySem == nil {
		return func() {}, nil
	}
	select {
	case m.trivySem <- struct{}{}:
		return func() { <-m.trivySem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// envWithoutTrivyCreds returns the current environment with TRIVY_USERNAME and
// TRIVY_PASSWORD removed. DB downloads access ghcr.io (not Docker Hub) and
// must use anonymous auth to avoid sending wrong credentials.
func envWithoutTrivyCreds() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "TRIVY_USERNAME=") || strings.HasPrefix(e, "TRIVY_PASSWORD=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// uploadDB tars and uploads a DB subdirectory from the manager's cache dir to S3.
func (m *Manager) uploadDB(ctx context.Context, s3Key, subDir, logName string) {
	if err := m.uploadDBFrom(ctx, s3Key, m.cacheDir, subDir, logName); err != nil {
		slog.Warn("trivydb: upload to S3 failed", "db", logName, "error", err)
	}
}

// uploadDBFrom tars a DB subdirectory to a temp file, then uploads to S3.
// The temp file avoids buffering the entire archive in memory (the java-db
// can be hundreds of MB) while remaining seekable for S3 checksum computation.
func (m *Manager) uploadDBFrom(ctx context.Context, s3Key, baseDir, subDir, logName string) error {
	srcDir := filepath.Join(baseDir, subDir)

	tmp, err := os.CreateTemp("", "trivydb-upload-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err := tarDirTo(tmp, srcDir, subDir); err != nil {
		return fmt.Errorf("tar %s: %w", logName, err)
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek temp file: %w", err)
	}
	fi, err := tmp.Stat()
	if err != nil {
		return fmt.Errorf("stat temp file: %w", err)
	}

	_, err = m.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &m.bucket,
		Key:           &s3Key,
		Body:          tmp,
		ContentLength: ptr(fi.Size()),
		ContentType:   ptr("application/gzip"),
	})
	if err != nil {
		return fmt.Errorf("S3 put %s: %w", logName, err)
	}

	slog.Info("trivydb: uploaded to S3", "db", logName, "key", s3Key, "size", fi.Size())
	return nil
}

// downloadFromS3 fetches a key from S3 and returns a reader over its body.
// The caller must close the returned ReadCloser.
func (m *Manager) downloadFromS3(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &m.bucket,
		Key:    &key,
	})
	if err != nil {
		var nsk *types.NoSuchKey
		var nf *types.NotFound
		if errors.As(err, &nsk) || errors.As(err, &nf) {
			return nil, fmt.Errorf("not found: %s", key)
		}
		return nil, err
	}
	return out.Body, nil
}

// tarDirTo writes a tar.gz archive of srcDir to w. Files are stored with paths
// relative to the parent, prefixed with prefix (e.g., "db/trivy.db").
func tarDirTo(w io.Writer, srcDir, prefix string) error {
	gw, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gw)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Build archive path: prefix/filename
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		archivePath := filepath.Join(prefix, rel)
		if archivePath == prefix {
			// Root directory entry
			return tw.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     prefix + "/",
				Mode:     0o755,
				ModTime:  info.ModTime(),
			})
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = archivePath
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

// tarDir creates a tar.gz archive in memory (used by tests).
func tarDir(srcDir, prefix string) ([]byte, error) {
	var buf bytes.Buffer
	if err := tarDirTo(&buf, srcDir, prefix); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// extractTar extracts a tar.gz archive from r into destDir.
// Streams directly from the reader without buffering the entire archive.
func extractTar(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Prevent path traversal
		target := filepath.Join(destDir, filepath.Clean(header.Name))
		if !isSubpath(destDir, target) {
			return fmt.Errorf("tar entry %q escapes destination", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)&0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// isSubpath returns true if target is under base.
func isSubpath(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	// Reject "..", "../foo", or any absolute path
	if rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	if len(rel) >= 3 && rel[:3] == "../" {
		return false
	}
	return true
}

func ptr[T any](v T) *T { return &v }
