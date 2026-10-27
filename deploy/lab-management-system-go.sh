#!/bin/sh
# Usage: lab-management-system-go.sh <DOCKER_USERNAME> <DOCKER_PASSWORD> [VERSION]
#
# 由 .github/workflows/ci.yml 的 deploy job 远程调用:
#   ssh deploy@vps -- cd /home/deploy/lab-management-system-go
#                    && sh lab-management-system-go.sh $DOCKER_USERNAME $DOCKER_PASSWORD $VERSION
#
# VERSION 默认是 latest。tag-based deploy 时显式传 tag 名（v0.x.y-YYYYMMDD）。
# CI 同时 push :latest + :<tag> 两份镜像,回滚只要手动指定旧 tag 再跑一次本脚本。
#
# 与姊妹仓 lab-management-system-kotlin.sh 的差异:
#   - 运行时 Go 1.27 静态二进制（golang:1.27-alpine builder, CGO_ENABLED=0,
#     alpine:3 runtime;镜像内零 JVM/解释器）
#   - env 契约 14 key = .env.example 全集（REQ-2026-004 全量业务切片起接 PG/JWT/
#     saas IdP 代理链）——PG_PASSWORD / LAB_SAAS_CLIENT_SECRET / 服务账号双键走 SSH
#     envs 转发（缺则 fail fast，服务账号 2026-08-28 纪律）；DATABASE_URL 由
#     PG_HOST/PG_PASSWORD 合成（远程 lab_prod）；lab JWT 无 aud（家族有意差异，
#     rails token_service 同款，不写 JWT_AUDIENCE）
#   - 绑全接口 ":port"（容器可达;kotor 先例 Netty 默认 0.0.0.0 同姿态）
#
# 前置: deploy 用户需在 docker 组中(sudo usermod -aG docker deploy)。
#        go.env 由本脚本首启自举（secret 类 key 必须由 CI secrets 转发）。

set -eu

USERNAME="${1:-}"
PASSWORD="${2:-}"
VERSION="${3:-latest}"
IMAGE="${USERNAME}/lab-management-system-go:${VERSION}"
BASE="/home/deploy/lab-management-system-go"
CONTAINER_NAME="lab-management-system-go"

NGINX_DOMAIN="${NGINX_DOMAIN:-lab-go.xiangru.uk}"
NGINX_CERT_BASENAME="${NGINX_CERT_BASENAME:-xiangru-uk}"

if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
  echo "Usage: $0 <DOCKER_USERNAME> <DOCKER_PASSWORD> [VERSION]" >&2
  exit 2
fi

# go.env 自举: env 契约 = .env.example 全集 14 key（L0.5 deploy parity 比对基线）。
# secret 类缺转发 = fail fast；JWT_SIGNING_KEY 转发缺省随机生成（持久化落 env-file）。
if [ -z "${PG_PASSWORD:-}" ] || [ -z "${LAB_SAAS_CLIENT_SECRET:-}" ] || [ -z "${LAB_SAAS_SERVICE_USER:-}" ] || [ -z "${LAB_SAAS_SERVICE_PASSWORD:-}" ]; then
  echo "ERROR: PG_PASSWORD / LAB_SAAS_CLIENT_SECRET / LAB_SAAS_SERVICE_USER / LAB_SAAS_SERVICE_PASSWORD secrets required (CI envs 转发;服务账号 fail-fast 2026-08-28 纪律)" >&2
  exit 1
fi
if [ ! -f "$BASE/go.env" ]; then
  echo "→ bootstrapping $BASE/go.env from env secrets"
  umask 077
  {
    printf 'SERVER_PORT=5211\n'
    printf 'DATABASE_URL=postgresql://postgres:%s@%s:5432/lab_prod\n' "$PG_PASSWORD" "${PG_HOST:-100.79.128.25}"
    printf 'JWT_SIGNING_KEY=%s\n' "${JWT_SIGNING_KEY:-$(head -c 48 /dev/urandom | base64 | tr -d '\n')}"
    printf 'JWT_ISSUER=lab-management-system\n'
    printf 'JWT_TTL_SECONDS=3600\n'
    printf 'JWT_REFRESH_TTL_SECONDS=604800\n'
    printf 'LAB_SAAS_BASE_URL=https://saas-go.xiangru.uk\n'
    printf 'LAB_SAAS_CLIENT_ID=lab-management\n'
    printf 'LAB_SAAS_CLIENT_SECRET=%s\n' "$LAB_SAAS_CLIENT_SECRET"
    printf 'LAB_SAAS_DEFAULT_TENANT_ID=%s\n' "${LAB_SAAS_DEFAULT_TENANT_ID:-00000000-0000-0000-0000-000000000001}"
    printf 'LAB_SAAS_SERVICE_USER=%s\n' "$LAB_SAAS_SERVICE_USER"
    printf 'LAB_SAAS_SERVICE_PASSWORD=%s\n' "$LAB_SAAS_SERVICE_PASSWORD"
    printf 'LAB_SAAS_SERVICE_CLIENT_ID=lab-management\n'
    printf 'LAB_SSO_LOGIN_URL=https://saas-react.xiangru.uk\n'
    printf 'LAB_CORS_ALLOWED_ORIGINS=https://lab-go.xiangru.uk,https://lab-react.xiangru.uk,https://lab-vue.xiangru.uk,https://lab-nextjs.xiangru.uk,https://lab-flutter.xiangru.uk\n'
  } > "$BASE/go.env"
  chown deploy:deploy "$BASE/go.env" 2>/dev/null || true
  chmod 600 "$BASE/go.env"
fi
if ! grep -q '^SERVER_PORT=..*' "$BASE/go.env"; then
  echo "ERROR: $BASE/go.env has no SERVER_PORT line" >&2
  exit 1
fi
if ! grep -q '^DATABASE_URL=..*' "$BASE/go.env"; then
  echo "ERROR: $BASE/go.env has no DATABASE_URL line (REQ-2026-004 起业务面必需；存量老 env 请删除后由本脚本重新自举)" >&2
  exit 1
fi

# 存量 go.env 补键/占位值 reconcile（rails 同款：bootstrap 残留占位会让容器起来即坏）
append_if_missing() {
  key="$1"; val="$2"
  if ! grep -q "^${key}=" "$BASE/go.env"; then
    echo "→ append ${key} to existing $BASE/go.env"
    umask 077
    printf '%s=%s\n' "$key" "$val" >> "$BASE/go.env"
  fi
}
append_if_missing SERVER_PORT '5211'
append_if_missing JWT_ISSUER 'lab-management-system'
append_if_missing JWT_TTL_SECONDS '3600'
append_if_missing JWT_REFRESH_TTL_SECONDS '604800'
append_if_missing LAB_SAAS_BASE_URL 'https://saas-go.xiangru.uk'
append_if_missing LAB_SSO_LOGIN_URL 'https://saas-react.xiangru.uk'
append_if_missing LAB_SAAS_CLIENT_ID 'lab-management'
append_if_missing LAB_SAAS_DEFAULT_TENANT_ID '00000000-0000-0000-0000-000000000001'
append_if_missing LAB_SAAS_SERVICE_CLIENT_ID 'lab-management'
append_if_missing LAB_CORS_ALLOWED_ORIGINS 'https://lab-go.xiangru.uk,https://lab-react.xiangru.uk,https://lab-nextjs.xiangru.uk,https://lab-vue.xiangru.uk,https://lab-flutter.xiangru.uk'
upsert_if_placeholder() {
  key="$1"; val="$2"
  if ! grep -q "^${key}=..*" "$BASE/go.env" || grep -q "^${key}=$" "$BASE/go.env" || grep -q "^${key}=CHANGE_ME$" "$BASE/go.env"; then
    echo "→ upsert ${key} to existing $BASE/go.env"
    sed -i "s#^${key}=.*#${key}=${val}#" "$BASE/go.env"
  fi
}
upsert_if_placeholder PG_PASSWORD "${PG_PASSWORD}"
upsert_if_placeholder JWT_SIGNING_KEY "${JWT_SIGNING_KEY:-$(head -c 48 /dev/urandom | base64 | tr -d '\n')}"
upsert_if_placeholder LAB_SAAS_CLIENT_SECRET "$LAB_SAAS_CLIENT_SECRET"
upsert_if_placeholder LAB_SAAS_SERVICE_USER "$LAB_SAAS_SERVICE_USER"
upsert_if_placeholder LAB_SAAS_SERVICE_PASSWORD "$LAB_SAAS_SERVICE_PASSWORD"
if ! grep -q '^DATABASE_URL=..*' "$BASE/go.env"; then
  printf 'DATABASE_URL=postgresql://postgres:%s@%s:5432/lab_prod\n' "$PG_PASSWORD" "${PG_HOST:-100.79.128.25}" >> "$BASE/go.env"
fi

# nginx vhost 重渲染(每次 deploy 都跑,ADR-0018:容器端口变了 vhost 必须跟)。
# 模板每次都从 master 拉最新 —— VPS 本地老模板会渲染出老端口全家族 502(2026-09-03 事故)。
# ?v=$(date +%s) cache-buster：CDN/raw 缓存防在案（flutter 先例）。
NGINX_SITES_AVAILABLE="/etc/nginx/sites-available"
NGINX_SITES_ENABLED="/etc/nginx/sites-enabled"
NGINX_VHOST_FILE="${NGINX_SITES_AVAILABLE}/${NGINX_DOMAIN}"
NGINX_VHOST_LINK="${NGINX_SITES_ENABLED}/${NGINX_DOMAIN}"
NGINX_TEMPLATE="${BASE}/nginx-vps.conf.example"

echo "→ fetching nginx-vps.conf.example template (always fresh from master)"
curl -fsSL "https://raw.githubusercontent.com/zcqiand/lab-management-system-go/refs/heads/master/deploy/nginx-vps.conf.example?v=$(date +%s)" -o "${NGINX_TEMPLATE}"

# 渲染到临时文件 —— sed 顺序:cert 归一化规则必须排在 <domain>/YOUR_DOMAIN 通配之前
# (先替换 <domain> 会把 cert 路径占位符一并吃掉,2026-09-03 事故根因)。
TMP_VHOST="$(mktemp -t vpstpl.XXXXXX)"
sed \
  -e "s|/etc/nginx/ssl/<domain>\.crt|/etc/nginx/ssl/${NGINX_CERT_BASENAME}.cert|g" \
  -e "s|/etc/nginx/ssl/<domain>\.cert|/etc/nginx/ssl/${NGINX_CERT_BASENAME}.cert|g" \
  -e "s|/etc/nginx/ssl/<domain>\.key|/etc/nginx/ssl/${NGINX_CERT_BASENAME}.key|g" \
  -e "s|/etc/nginx/ssl/your-cert\.crt|/etc/nginx/ssl/${NGINX_CERT_BASENAME}.cert|g" \
  -e "s|/etc/nginx/ssl/your-cert\.cert|/etc/nginx/ssl/${NGINX_CERT_BASENAME}.cert|g" \
  -e "s|/etc/nginx/ssl/your-cert\.key|/etc/nginx/ssl/${NGINX_CERT_BASENAME}.key|g" \
  -e "s|<domain>|${NGINX_DOMAIN}|g" \
  -e "s|lab\.YOUR_DOMAIN|${NGINX_DOMAIN}|g" \
  -e "s|saas\.YOUR_DOMAIN|${NGINX_DOMAIN}|g" \
  "${NGINX_TEMPLATE}" > "${TMP_VHOST}"

if [ -e "${NGINX_VHOST_FILE}" ] && diff -q "${TMP_VHOST}" "${NGINX_VHOST_FILE}" >/dev/null 2>&1; then
  echo "→ nginx vhost ${NGINX_VHOST_FILE} unchanged, skip"
  rm -f "${TMP_VHOST}"
else
  echo "→ rendering nginx vhost ${NGINX_VHOST_FILE} (domain=${NGINX_DOMAIN} cert=${NGINX_CERT_BASENAME})"
  if [ -w "${NGINX_SITES_AVAILABLE}" ]; then
    cp "${TMP_VHOST}" "${NGINX_VHOST_FILE}"
  else
    sudo cp "${TMP_VHOST}" "${NGINX_VHOST_FILE}" \
      || { echo "ERROR: sudo cp ${NGINX_VHOST_FILE} failed"; rm -f "${TMP_VHOST}"; exit 1; }
  fi
  if [ -w "${NGINX_SITES_ENABLED}" ]; then
    ln -sf "${NGINX_VHOST_FILE}" "${NGINX_VHOST_LINK}"
  else
    sudo ln -sf "${NGINX_VHOST_FILE}" "${NGINX_VHOST_LINK}" \
      || { echo "ERROR: sudo ln ${NGINX_VHOST_LINK} failed"; rm -f "${TMP_VHOST}"; exit 1; }
  fi
  rm -f "${TMP_VHOST}"
  echo "→ nginx -t"
  sudo nginx -t
  echo "→ systemctl reload nginx"
  sudo systemctl reload nginx
  echo "✓ nginx reloaded"
fi

echo "→ image: $IMAGE"
echo "→ docker login"
printf '%s' "$PASSWORD" | docker login -u "$USERNAME" --password-stdin

echo "→ docker pull"
docker pull "$IMAGE"

echo "→ docker stop & rm $CONTAINER_NAME"
docker stop "$CONTAINER_NAME" 2>/dev/null || true
docker rm "$CONTAINER_NAME" 2>/dev/null || true

echo "→ docker run"
docker run -d \
  --name "$CONTAINER_NAME" \
  --restart unless-stopped \
  -p "127.0.0.1:5211:5211" \
  --env-file "$BASE/go.env" \
  "$IMAGE"

echo "→ docker image prune"
docker image prune -f

echo "→ docker ps"
docker ps --filter name="$CONTAINER_NAME"

# 健康检查: 直接 wget /health 探 200。容器死亡提前终止循环, 立刻报失败。
i=0
while [ $i -lt 120 ]; do
  if wget --tries=1 --timeout=3 -q "http://127.0.0.1:5211/health" -O /dev/null 2>/dev/null; then
    echo "→ /health 200 (host 127.0.0.1:5211) after ${i}s"
    break
  fi
  if ! docker inspect --format='{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null | grep -q true; then
    echo "→ container not running, logs:"
    docker logs --tail 30 "$CONTAINER_NAME"
    exit 1
  fi
  i=$((i+1))
  sleep 1
done

if [ $i -ge 120 ]; then
  echo "→ /health 仍未 200(120s 上限), logs:"
  docker logs --tail 30 "$CONTAINER_NAME"
  exit 1
fi

echo "→ deploy done at $(date -u)"
