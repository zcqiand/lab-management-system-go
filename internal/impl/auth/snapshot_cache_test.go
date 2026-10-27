package auth

// snapshot_cache_test.go — 进程内 TTL 快照缓存（镜像 rails SnapshotCache：30min TTL、
// miss/过期返 ok=false、空切片也是 hit——家族裁定 tracker 5.111）。

import (
	"testing"
	"time"
)

func TestSnapshotCachePutGet(t *testing.T) {
	// 60*time.Second：裸 60 = 60ns，Windows 粗时钟侥幸绿、Linux runner 必过期（CI 实证）
	c := NewSnapshotCache(60 * time.Second)
	c.Put("u1", []string{"a"})
	v, ok := c.Get("u1")
	if !ok || v.([]string)[0] != "a" {
		t.Fatalf("hit 不符: %v %v", v, ok)
	}
}

func TestSnapshotCacheMiss(t *testing.T) {
	c := NewSnapshotCache(60 * time.Second)
	if _, ok := c.Get("nobody"); ok {
		t.Fatal("miss 应 ok=false")
	}
}

func TestSnapshotCacheEmptyValueIsHit(t *testing.T) {
	c := NewSnapshotCache(60 * time.Second)
	c.Put("u1", []string{})
	v, ok := c.Get("u1")
	if !ok || v == nil {
		t.Fatalf("空切片也是 hit（tracker 5.111）: %v %v", v, ok)
	}
}

func TestSnapshotCacheExpiry(t *testing.T) {
	c := NewSnapshotCache(-time.Second) // TTL 已过：立刻过期
	c.Put("u1", "v")
	if _, ok := c.Get("u1"); ok {
		t.Fatal("过期条目应 miss")
	}
}
