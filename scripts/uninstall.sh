#!/usr/bin/env bash
# ==============================================================================
# Redis Manager 官方卸载脚本
# ==============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
PLAIN='\033[0m'

if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[错误] 必须使用 root 权限运行此卸载脚本！${PLAIN}"
    exit 1
fi

echo -e "${YELLOW}==========================================================${PLAIN}"
echo -e "${YELLOW}       确定要彻底卸载 Redis Manager 面板吗？              ${PLAIN}"
echo -e "${YELLOW}==========================================================${PLAIN}"
read -p "请输入 'y' 确认继续卸载操作 (y/N): " CONFIRM
if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo "操作已取消。"
    exit 0
fi

echo "正在停止 Redis Manager 服务..."
systemctl stop redis-manager.service || true
systemctl disable redis-manager.service || true

rm -f /etc/systemd/system/redis-manager.service
systemctl daemon-reload

echo "正在删除主程序二进制..."
rm -f /usr/local/bin/redis-manager

read -p "是否同时清空数据库与快照目录 /var/lib/redis-manager？(默认保留) [y/N]: " CLEAN_DATA
if [[ "$CLEAN_DATA" == "y" || "$CLEAN_DATA" == "Y" ]]; then
    rm -rf /var/lib/redis-manager
    rm -rf /etc/redis-manager
    echo "数据与配置文件已清除。"
else
    echo "数据与配置文件已安全保留在 /var/lib/redis-manager 和 /etc/redis-manager。"
fi

echo -e "${GREEN}✓ Redis Manager 已成功从系统卸载。${PLAIN}"
