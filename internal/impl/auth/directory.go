package auth

// directory.go — 配置式内存目录（镜像 rails UserDirectory，lab-springboot
// ConfigUserDirectory 谱系）。lab 库无身份表：用户/租户在内存。
//   - 用户：alice（USER-A, admin）——账密校验已代理 saas IdP，本目录只承担
//     SSO/refresh/me 的用户与租户查找
//   - 租户：TENANT-001 city-lab / TENANT-002 district-lab / TENANT-003 third-party
//   - SSO 用户按 email upsert（幂等）；per-user saas refresh token（rotate-once）
//
// 并发：handler 面共享单例，map 读写加锁（ktor Netty 共享态教训同源）。

import "sync"

// CurrentUser 目录用户。
type CurrentUser struct {
	ID          string
	Username    string
	DisplayName string
	RoleCode    string
}

// MyTenant 目录租户。
type MyTenant struct {
	TenantID string
	Code     string
	Name     string
	RoleIDs  []string
}

// DirectoryTenantID 默认租户（DirectoryDefaultTenant 别名，service 层用）。
var directoryTenants = []MyTenant{
	{TenantID: "TENANT-001", Code: "city-lab", Name: "市住建工程质量检测中心", RoleIDs: []string{"admin"}},
	{TenantID: "TENANT-002", Code: "district-lab", Name: "区检测站", RoleIDs: []string{"technician"}},
	{TenantID: "TENANT-003", Code: "third-party", Name: "第三方检测实验室", RoleIDs: []string{"viewer"}},
}

// Directory 内存目录。
type Directory struct {
	mu               sync.Mutex
	demoUser         CurrentUser
	upserted         map[string]*CurrentUser // email => CurrentUser（指针稳定：同 email 复用同一对象）
	saasRefreshToken map[string]string       // user_id => saas refresh token
}

// NewDirectory 构造（demo 用户 alice/USER-A/admin）。
func NewDirectory() *Directory {
	return &Directory{
		demoUser:         CurrentUser{ID: "USER-A", Username: "alice", DisplayName: "管理员", RoleCode: "admin"},
		upserted:         map[string]*CurrentUser{},
		saasRefreshToken: map[string]string{},
	}
}

// FindByUsername 按用户名查（demo 用户优先）。
func (d *Directory) FindByUsername(username string) *CurrentUser {
	d.mu.Lock()
	defer d.mu.Unlock()
	if username == d.demoUser.Username {
		u := d.demoUser
		return &u
	}
	for _, u := range d.upserted {
		if u.Username == username {
			return u
		}
	}
	return nil
}

// FindByEmail 按 email 查（ADR-0008 username/email 桥接：demo 用户 username 即命中）。
func (d *Directory) FindByEmail(email string) *CurrentUser {
	d.mu.Lock()
	defer d.mu.Unlock()
	if email == d.demoUser.Username {
		u := d.demoUser
		return &u
	}
	for _, u := range d.upserted {
		if u.Username == email {
			return u
		}
	}
	return nil
}

// FindByID 按 id 查。
func (d *Directory) FindByID(id string) *CurrentUser {
	d.mu.Lock()
	defer d.mu.Unlock()
	if id == d.demoUser.ID {
		u := d.demoUser
		return &u
	}
	for _, u := range d.upserted {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// TenantsOf 目录租户全集（lab 单租户域：任意用户同三租户，rails 同构）。
func (d *Directory) TenantsOf(_ string) []MyTenant {
	out := make([]MyTenant, len(directoryTenants))
	copy(out, directoryTenants)
	return out
}

// DefaultTenant 默认租户（TENANT-001）。
func (d *Directory) DefaultTenant() MyTenant { return directoryTenants[0] }

// FindByTenantID 按租户 id 查。
func (d *Directory) FindByTenantID(tenantID string) *MyTenant {
	for i := range directoryTenants {
		if directoryTenants[i].TenantID == tenantID {
			t := directoryTenants[i]
			return &t
		}
	}
	return nil
}

// Upsert SSO 首次落地（email 键幂等；空 roleCode 降级 viewer）。
func (d *Directory) Upsert(id, email, displayName, roleCode string) *CurrentUser {
	d.mu.Lock()
	defer d.mu.Unlock()
	if u, ok := d.upserted[email]; ok {
		return u
	}
	if roleCode == "" {
		roleCode = "viewer"
	}
	u := &CurrentUser{ID: id, Username: email, DisplayName: displayName, RoleCode: roleCode}
	d.upserted[email] = u
	return u
}

// SetSaasRefreshToken 落 per-user saas refresh token（空键/空值忽略）。
func (d *Directory) SetSaasRefreshToken(userID, token string) {
	if userID == "" || token == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.saasRefreshToken[userID] = token
}

// GetSaasRefreshToken 取 per-user saas refresh token（未设置空串）。
func (d *Directory) GetSaasRefreshToken(userID string) string {
	if userID == "" {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.saasRefreshToken[userID]
}
