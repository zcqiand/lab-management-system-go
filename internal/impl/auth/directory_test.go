package auth

// directory_test.go — 内存目录（镜像 rails UserDirectory：demo 用户 + 三租户 +
// email 桥接 upsert + per-user saas refresh token）。

import "testing"

func TestDirectoryDemoUserLookups(t *testing.T) {
	d := NewDirectory()
	if u := d.FindByUsername("alice"); u == nil || u.ID != "USER-A" || u.RoleCode != "admin" {
		t.Fatalf("demo 用户查找不符: %+v", u)
	}
	// ADR-0008 email 桥接：email=alice 命中 demo 用户
	if u := d.FindByEmail("alice"); u == nil || u.ID != "USER-A" {
		t.Fatalf("email 桥接应命中 demo 用户: %+v", u)
	}
	if u := d.FindByID("USER-A"); u == nil {
		t.Fatal("按 id 应命中 demo 用户")
	}
	if u := d.FindByUsername("nobody"); u != nil {
		t.Fatalf("未知用户应 nil: %+v", u)
	}
}

func TestDirectoryTenants(t *testing.T) {
	d := NewDirectory()
	ts := d.TenantsOf("alice")
	if len(ts) != 3 || ts[0].TenantID != "TENANT-001" || ts[0].Code != "city-lab" {
		t.Fatalf("三租户不符: %+v", ts)
	}
	if d.DefaultTenant().TenantID != "TENANT-001" {
		t.Fatal("默认租户应 TENANT-001")
	}
	if tt := d.FindByTenantID("TENANT-003"); tt == nil || tt.Code != "third-party" {
		t.Fatalf("按租户 id 查找不符: %+v", tt)
	}
}

func TestDirectoryUpsertIdempotentByEmail(t *testing.T) {
	d := NewDirectory()
	u1 := d.Upsert("saas-uuid-1", "bob@x.uk", "Bob", "viewer")
	u2 := d.Upsert("saas-uuid-1", "bob@x.uk", "Bob", "viewer")
	if u1 != u2 {
		t.Fatal("同 email upsert 应复用同一用户")
	}
	if got := d.FindByID("saas-uuid-1"); got == nil || got.Username != "bob@x.uk" {
		t.Fatalf("upsert 后按 id 查找不符: %+v", got)
	}
	// 空 roleCode 降级 viewer
	u3 := d.Upsert("saas-uuid-2", "carol@x.uk", "Carol", "")
	if u3.RoleCode != "viewer" {
		t.Fatalf("空 roleCode 应降级 viewer: %+v", u3)
	}
}

func TestDirectorySaasRefreshTokens(t *testing.T) {
	d := NewDirectory()
	if got := d.GetSaasRefreshToken("u1"); got != "" {
		t.Fatalf("未设置应空串: %q", got)
	}
	d.SetSaasRefreshToken("u1", "rt-1")
	if got := d.GetSaasRefreshToken("u1"); got != "rt-1" {
		t.Fatalf("取回不符: %q", got)
	}
}
