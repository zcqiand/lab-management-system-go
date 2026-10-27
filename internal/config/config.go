// Package config 装配运行环境配置：键只来自 env，缺失/非法即报错。
// 禁止默认值兜底（suite-hard-rules §1）。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Port 读取 SERVER_PORT（multi-repo-family §6 家族统一 key）并校验为合法端口。
func Port() (int, error) {
	raw := os.Getenv("SERVER_PORT")
	if raw == "" {
		return 0, fmt.Errorf("缺少环境变量 SERVER_PORT（全集见 .env.example；禁默认值兜底）")
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("SERVER_PORT 不是合法端口数字 %q: %w", raw, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("SERVER_PORT 超出 1-65535: %d", port)
	}
	return port, nil
}

// DatabaseURL 读取 DATABASE_URL（REQ-2026-004：GORM 连家族 PG 三库的入口，
// dev=lab_dev / test=saas_test / prod 由 deploy env-file 注入）。
func DatabaseURL() (string, error) {
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		return "", fmt.Errorf("缺少环境变量 DATABASE_URL（全集见 .env.example；禁默认值兜底）")
	}
	return raw, nil
}

// SaasIDPConfig saas IdP 出站五键（装配层透传给 saas.NewHTTPClient）。
type SaasIDPConfig struct {
	Base, ClientID, ClientSecret, DefaultTenantID, ServiceClientID string
}

// SaasIDP 读取 saas IdP 出站五键（REQ-2026-023 同款，fail-fast 禁兜底）。
func SaasIDP() (SaasIDPConfig, error) {
	c := SaasIDPConfig{
		Base:            strings.TrimRight(os.Getenv("LAB_SAAS_BASE_URL"), "/"),
		ClientID:        os.Getenv("LAB_SAAS_CLIENT_ID"),
		ClientSecret:    os.Getenv("LAB_SAAS_CLIENT_SECRET"),
		DefaultTenantID: os.Getenv("LAB_SAAS_DEFAULT_TENANT_ID"),
		ServiceClientID: os.Getenv("LAB_SAAS_SERVICE_CLIENT_ID"),
	}
	for key, val := range map[string]string{
		"LAB_SAAS_BASE_URL": c.Base, "LAB_SAAS_CLIENT_ID": c.ClientID,
		"LAB_SAAS_CLIENT_SECRET": c.ClientSecret, "LAB_SAAS_DEFAULT_TENANT_ID": c.DefaultTenantID,
		"LAB_SAAS_SERVICE_CLIENT_ID": c.ServiceClientID,
	} {
		if val == "" {
			return c, fmt.Errorf("缺少环境变量 %s（禁默认值兜底）", key)
		}
	}
	return c, nil
}
