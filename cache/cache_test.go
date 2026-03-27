package cache

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockObject stores a cached S3 object in memory.
type mockObject struct {
	body     []byte
	metadata map[string]string
}

// mockS3 implements s3API backed by an in-memory map.
type mockS3 struct {
	mu      sync.Mutex
	objects map[string]*mockObject
}

func newMockS3() *mockS3 {
	return &mockS3{objects: make(map[string]*mockObject)}
}

func (m *mockS3) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[*input.Key] = &mockObject{
		body:     data,
		metadata: input.Metadata,
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj, ok := m.objects[*input.Key]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body:     io.NopCloser(bytes.NewReader(obj.body)),
		Metadata: obj.metadata,
	}, nil
}

func TestSetGetRoundTrip(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	original := []byte(`{"success":true,"data":{"digest":"sha256:abc"}}`)

	if err := store.Set(ctx, "inspect/sha256:abc", original, 1*time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, _, err := store.Get(ctx, "inspect/sha256:abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !bytes.Equal(got, original) {
		t.Errorf("round-trip mismatch:\n  got:  %s\n  want: %s", got, original)
	}
}

func TestGetNotFound(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	_, _, err := store.Get(ctx, "nonexistent/key")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestTTLExpiry(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	data := []byte(`{"expired":true}`)

	// Set with a TTL in the past by manually crafting the metadata
	if err := store.Set(ctx, "scan/sha256:old", data, 1*time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Overwrite the metadata to make it expired
	mock.mu.Lock()
	obj := mock.objects["scan/sha256:old"]
	obj.metadata["expires-at"] = strconv.FormatInt(time.Now().Add(-1*time.Second).Unix(), 10)
	mock.mu.Unlock()

	_, _, err := store.Get(ctx, "scan/sha256:old")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for expired entry, got: %v", err)
	}
}

func TestGetOrComputeCacheHit(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	original := []byte(`{"cached":"data"}`)
	if err := store.Set(ctx, "inspect/sha256:hit", original, 1*time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var calls atomic.Int32
	result, err := store.GetOrCompute(ctx, "inspect/sha256:hit", 1*time.Hour, func() ([]byte, error) {
		calls.Add(1)
		return []byte(`{"fresh":"data"}`), nil
	})
	if err != nil {
		t.Fatalf("GetOrCompute: %v", err)
	}

	if calls.Load() != 0 {
		t.Errorf("fn was called %d times on cache hit, expected 0", calls.Load())
	}
	if !result.FromCache {
		t.Error("expected FromCache=true")
	}
	if !bytes.Equal(result.Data, original) {
		t.Errorf("data mismatch: got %s, want %s", result.Data, original)
	}
}

func TestGetOrComputeCacheMiss(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	fresh := []byte(`{"fresh":"computed"}`)
	result, err := store.GetOrCompute(ctx, "scan/sha256:miss", 1*time.Hour, func() ([]byte, error) {
		return fresh, nil
	})
	if err != nil {
		t.Fatalf("GetOrCompute: %v", err)
	}

	if result.FromCache {
		t.Error("expected FromCache=false on miss")
	}
	if !bytes.Equal(result.Data, fresh) {
		t.Errorf("data mismatch: got %s, want %s", result.Data, fresh)
	}

	// Wait briefly for background Set goroutine
	time.Sleep(50 * time.Millisecond)

	// Verify it was stored
	got, _, err := store.Get(ctx, "scan/sha256:miss")
	if err != nil {
		t.Fatalf("Get after compute: %v", err)
	}
	if !bytes.Equal(got, fresh) {
		t.Errorf("stored data mismatch: got %s, want %s", got, fresh)
	}
}

func TestSingleflightDedup(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	var calls atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.GetOrCompute(ctx, "scan/sha256:dedup", 1*time.Hour, func() ([]byte, error) {
				calls.Add(1)
				time.Sleep(50 * time.Millisecond) // simulate work
				return []byte(`{"result":"ok"}`), nil
			})
			if err != nil {
				t.Errorf("GetOrCompute: %v", err)
			}
		}()
	}
	wg.Wait()

	if c := calls.Load(); c != 1 {
		t.Errorf("singleflight: fn called %d times, expected 1", c)
	}
}

func TestCachedAtTimestamp(t *testing.T) {
	mock := newMockS3()
	store := newWithClient(mock, "test-bucket")
	ctx := context.Background()

	before := time.Now().Add(-1 * time.Second)

	if err := store.Set(ctx, "inspect/sha256:ts", []byte(`{}`), 1*time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, cachedAt, err := store.Get(ctx, "inspect/sha256:ts")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if cachedAt.Before(before) {
		t.Errorf("cachedAt %v is before test start %v", cachedAt, before)
	}
	if cachedAt.After(time.Now().Add(1 * time.Second)) {
		t.Errorf("cachedAt %v is in the future", cachedAt)
	}
}
