// Command server 组合根：读 SERVER_PORT / DATABASE_URL，装配生成路由并起 HTTP 服务。
// 装配期 fail-fast：缺键/非法直接退出非零，不兜底不重试。
package main

import (
	"fmt"
	"log"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"lab-management-system-go/internal/config"
	"lab-management-system-go/internal/impl/auth"
	"lab-management-system-go/internal/impl/harness"
	"lab-management-system-go/internal/impl/saas"
	"lab-management-system-go/internal/impl/service"
	"lab-management-system-go/internal/server"
)

func main() {
	port, err := config.Port()
	if err != nil {
		log.Fatalf("装配失败: %v", err)
	}
	dsn, err := config.DatabaseURL()
	if err != nil {
		log.Fatalf("装配失败: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("装配失败: 数据库连接不可用: %v", err)
	}
	// 认证域服务（Task 5）：saas IdP 出站五键 fail-fast（REQ-2026-023 同款）；
	// 目录/快照缓存进程内单例——跨请求存活（rails AuthService.instance 同语义）。
	idp, err := config.SaasIDP()
	if err != nil {
		log.Fatalf("装配失败: %v", err)
	}
	authSvc := service.NewAuthService(
		auth.NewDirectory(),
		auth.NewSnapshotCache(auth.SnapshotCacheTTL),
		auth.NewSnapshotCache(auth.SnapshotCacheTTL),
		saas.NewHTTPClient(idp.Base, idp.ClientID, idp.ClientSecret, idp.DefaultTenantID, idp.ServiceClientID),
	)
	// 绑全接口 ":port"（REQ-2026-002）：容器里绑 127.0.0.1 的话 host 端口映射
	// 打不进容器回环，deploy 探针必 connection refused。ktor 先例：
	// embeddedServer(Netty, port=…) 无 host 参数 = Netty 默认 0.0.0.0，dev/prod 同姿态。
	addr := fmt.Sprintf(":%d", port)
	srv := server.New(addr, harness.Assemble(db, authSvc))
	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}
