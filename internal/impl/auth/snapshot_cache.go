package auth

// snapshot_cache.go — 进程内 TTL 快照缓存（镜像 rails SnapshotCache：30min TTL、
// 按 userId 存取、miss/过期返 ok=false；**空值（空切片）也是 hit**——家族裁定
// tracker 5.111，me() 空 tenants 不得误判 miss 走降级）。并发加锁。

import (
	"sync"
	"time"
)

// SnapshotCacheTTL 默认 30 分钟。
const SnapshotCacheTTL = 30 * time.Minute

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// SnapshotCache TTL 快照缓存。
type SnapshotCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	store map[string]cacheEntry
}

// NewSnapshotCache 构造（ttlSeconds<=0 用于测试立刻过期）。
func NewSnapshotCache(ttl time.Duration) *SnapshotCache {
	return &SnapshotCache{ttl: ttl, store: map[string]cacheEntry{}}
}

// Put 写入（nil 值不写）。
func (c *SnapshotCache) Put(userID string, value any) {
	if userID == "" || value == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[userID] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
}

// Get 读取（miss/过期删除并返 ok=false）。
func (c *SnapshotCache) Get(userID string) (any, bool) {
	if userID == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.store[userID]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.store, userID)
		return nil, false
	}
	return entry.value, true
}
