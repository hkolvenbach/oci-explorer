// Package cache provides an S3-compatible response cache with singleflight
// deduplication and gzip compression. It is designed for use with Tigris,
// AWS S3, or any S3-compatible object store.
package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/sync/singleflight"
)

// ErrNotFound is returned when a cache entry does not exist or has expired.
var ErrNotFound = errors.New("cache: not found")

// s3API is the subset of the S3 client used by Store, enabling test mocks.
type s3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// Store is an S3-backed response cache.
type Store struct {
	client s3API
	bucket string
	group  singleflight.Group
}

// New creates a Store backed by the given S3 bucket.
// It uses config.LoadDefaultConfig which reads AWS_ENDPOINT_URL_S3 (for Tigris
// or custom endpoints), AWS_REGION, and credentials from the environment.
func New(ctx context.Context, bucket string) (*Store, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("cache: load AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg)
	return &Store{client: client, bucket: bucket}, nil
}

// newWithClient creates a Store with an injected S3 client (for testing).
func newWithClient(client s3API, bucket string) *Store {
	return &Store{client: client, bucket: bucket}
}

// Result holds the data returned by GetOrCompute along with cache metadata.
type Result struct {
	Data      []byte
	FromCache bool
	CachedAt  time.Time // zero value if not from cache
}

// Get retrieves a cached entry. Returns ErrNotFound if the key does not exist
// or the entry has expired.
func (s *Store) Get(ctx context.Context, key string) ([]byte, time.Time, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		// Treat NoSuchKey as ErrNotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, time.Time{}, ErrNotFound
		}
		// Some S3-compatible backends return a generic error for missing keys
		var nf *types.NotFound
		if errors.As(err, &nf) {
			return nil, time.Time{}, ErrNotFound
		}
		return nil, time.Time{}, fmt.Errorf("cache get %q: %w", key, err)
	}
	defer out.Body.Close()

	// Check TTL via metadata
	if expiresStr, ok := out.Metadata["expires-at"]; ok {
		expiresSec, err := strconv.ParseInt(expiresStr, 10, 64)
		if err == nil && time.Now().Unix() > expiresSec {
			return nil, time.Time{}, ErrNotFound
		}
	}

	// Parse cached-at timestamp
	var cachedAt time.Time
	if cachedAtStr, ok := out.Metadata["cached-at"]; ok {
		if t, err := time.Parse(time.RFC3339, cachedAtStr); err == nil {
			cachedAt = t
		}
	}

	// Decompress
	gr, err := gzip.NewReader(out.Body)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("cache decompress %q: %w", key, err)
	}
	defer gr.Close()

	data, err := io.ReadAll(gr)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("cache read %q: %w", key, err)
	}

	return data, cachedAt, nil
}

// Set compresses and stores data with the given TTL.
func (s *Store) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("cache compress init: %w", err)
	}
	if _, err := gw.Write(data); err != nil {
		return fmt.Errorf("cache compress write: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("cache compress close: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(ttl)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          &s.bucket,
		Key:             &key,
		Body:            bytes.NewReader(buf.Bytes()),
		ContentEncoding: aws.String("gzip"),
		ContentType:     aws.String("application/json"),
		Metadata: map[string]string{
			"expires-at": strconv.FormatInt(expiresAt.Unix(), 10),
			"cached-at":  now.UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return fmt.Errorf("cache set %q: %w", key, err)
	}

	slog.Debug("cache set", "key", key, "size", len(data), "compressed", buf.Len(), "ttl", ttl)
	return nil
}

// GetOrCompute returns cached data for the key, or computes it via fn on a miss.
// Uses singleflight to deduplicate concurrent calls for the same key.
func (s *Store) GetOrCompute(ctx context.Context, key string, ttl time.Duration, fn func() ([]byte, error)) (*Result, error) {
	// Try cache first
	data, cachedAt, err := s.Get(ctx, key)
	if err == nil {
		slog.Debug("cache hit", "key", key)
		return &Result{Data: data, FromCache: true, CachedAt: cachedAt}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		// Log S3 errors but proceed to compute (cache is best-effort)
		slog.Warn("cache get error, computing fresh", "key", key, "error", err)
	}

	// Cache miss -- compute with singleflight deduplication
	type sfResult struct {
		data     []byte
		cachedAt time.Time
	}

	v, err, _ := s.group.Do(key, func() (any, error) {
		slog.Debug("cache miss, computing", "key", key)
		result, err := fn()
		if err != nil {
			return nil, err
		}

		// Store in background (don't block the response on S3 write)
		now := time.Now().UTC()
		go func() {
			if setErr := s.Set(context.Background(), key, result, ttl); setErr != nil {
				slog.Warn("cache set failed", "key", key, "error", setErr)
			}
		}()

		return &sfResult{data: result, cachedAt: now}, nil
	})
	if err != nil {
		return nil, err
	}

	sf := v.(*sfResult)
	return &Result{Data: sf.data, FromCache: false, CachedAt: sf.cachedAt}, nil
}
