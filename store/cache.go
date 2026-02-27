package store

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
)

// discoveryCache provides an in-memory TTL cache for infrequently changing
// discovery queries (services, operations, dependencies).
type discoveryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

func newDiscoveryCache(ttl time.Duration) *discoveryCache {
	return &discoveryCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// get returns cached data for key if it exists and hasn't expired.
func (c *discoveryCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

// set stores data for key with the configured TTL.
func (c *discoveryCache) set(key string, data interface{}) {
	c.mu.Lock()
	c.entries[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// dependencyRefresher runs a background goroutine that periodically refreshes
// the dependency cache so user requests never block on the expensive query.
type dependencyRefresher struct {
	reader   *kustoSpanReader
	cache    *discoveryCache
	lookback time.Duration
	interval time.Duration
	logger   hclog.Logger
	stopCh   chan struct{}
}

func newDependencyRefresher(reader *kustoSpanReader, cache *discoveryCache, interval time.Duration, logger hclog.Logger) *dependencyRefresher {
	return &dependencyRefresher{
		reader:   reader,
		cache:    cache,
		lookback: maxDependencyLookback,
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

const dependencyCacheKey = "dependencies"

// start launches the background refresh loop.
func (d *dependencyRefresher) start() {
	go func() {
		d.logger.Info("Dependency cache refresher started", "interval", d.interval, "lookback", d.lookback)

		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.refresh()
			case <-d.stopCh:
				d.logger.Info("Dependency cache refresher stopped")
				return
			}
		}
	}()
}

func (d *dependencyRefresher) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	endTs := time.Now()
	links, err := d.reader.fetchDependencies(ctx, endTs, d.lookback)
	if err != nil {
		d.logger.Error("Background dependency refresh failed", "error", err)
		return
	}

	d.cache.set(dependencyCacheKey, links)
	d.logger.Info("Background dependency refresh complete", "links", len(links))
}

func (d *dependencyRefresher) stop() {
	close(d.stopCh)
}

// getCachedDependencies returns cached results. On cold start (first request),
// triggers a synchronous fetch and caches the result for subsequent requests.
func (d *dependencyRefresher) getCachedDependencies(ctx context.Context) ([]model.DependencyLink, bool) {
	if cached, ok := d.cache.get(dependencyCacheKey); ok {
		return cached.([]model.DependencyLink), true
	}
	// Cold start: fetch synchronously on first request
	d.logger.Info("Dependency cache cold start, fetching synchronously")
	d.refresh()
	if cached, ok := d.cache.get(dependencyCacheKey); ok {
		return cached.([]model.DependencyLink), true
	}
	return nil, false
}
