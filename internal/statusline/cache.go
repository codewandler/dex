package statusline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileCache provides file-based caching with TTL for status line data
type FileCache struct {
	sessionID string
	cacheDir  string
}

// CacheData represents the cached data structure
type CacheData struct {
	Segments  map[string]SegmentCache `json:"segments"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// SegmentCache holds cached data for a single segment
type SegmentCache struct {
	Data      map[string]any `json:"data"`
	UpdatedAt time.Time      `json:"updated_at"`
	TTL       time.Duration  `json:"ttl"`
}

// NewFileCache creates a new file-based cache for the given session
func NewFileCache(sessionID string) (*FileCache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(home, ".dex", "claude", "statusline")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, err
	}

	return &FileCache{
		sessionID: sessionID,
		cacheDir:  cacheDir,
	}, nil
}

func (c *FileCache) cacheFile() string {
	// Use session ID if available, otherwise use "default"
	id := c.sessionID
	if id == "" {
		id = "default"
	}
	return filepath.Join(c.cacheDir, id+".json")
}

// Load reads the cache from disk
func (c *FileCache) Load() (*CacheData, error) {
	data, err := os.ReadFile(c.cacheFile())
	if err != nil {
		if os.IsNotExist(err) {
			return &CacheData{Segments: make(map[string]SegmentCache)}, nil
		}
		return nil, err
	}

	var cache CacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted cache, start fresh
		return &CacheData{Segments: make(map[string]SegmentCache)}, nil
	}

	if cache.Segments == nil {
		cache.Segments = make(map[string]SegmentCache)
	}

	return &cache, nil
}

// Save writes the cache to disk
func (c *FileCache) Save(cache *CacheData) error {
	cache.UpdatedAt = time.Now()
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(c.cacheFile(), data, 0600)
}

// Get retrieves segment data from cache
func (c *FileCache) Get(cache *CacheData, segment string) (data map[string]any, ok bool, stale bool) {
	seg, exists := cache.Segments[segment]
	if !exists {
		return nil, false, false
	}

	isStale := time.Since(seg.UpdatedAt) > seg.TTL
	return seg.Data, true, isStale
}

// Set stores segment data in cache
func (c *FileCache) Set(cache *CacheData, segment string, data map[string]any, ttl time.Duration) {
	cache.Segments[segment] = SegmentCache{
		Data:      data,
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}
}

// BackgroundRefresher handles async cache updates
type BackgroundRefresher struct {
	cache     *FileCache
	cacheData *CacheData
	mu        sync.Mutex
	pending   map[string]bool
}

// NewBackgroundRefresher creates a refresher for async updates
func NewBackgroundRefresher(cache *FileCache, cacheData *CacheData) *BackgroundRefresher {
	return &BackgroundRefresher{
		cache:     cache,
		cacheData: cacheData,
		pending:   make(map[string]bool),
	}
}

// MarkPending marks a segment as needing refresh
func (r *BackgroundRefresher) MarkPending(segment string) {
	r.mu.Lock()
	r.pending[segment] = true
	r.mu.Unlock()
}

// IsPending checks if a segment is marked for refresh
func (r *BackgroundRefresher) IsPending(segment string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending[segment]
}
