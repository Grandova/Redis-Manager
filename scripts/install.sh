#!/usr/bin/env bash
# ==============================================================================
# Redis Manager 官方一键极速安装脚本
# 支持系统: Debian 11/12/13, Ubuntu 22.04/24.04 (amd64 / arm64)
# ==============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PLAIN='\033[0m'

echo -e "${RED}"
echo "=========================================================="
echo "          Redis Manager 现代化 Web 运维管理面板           "
echo "=========================================================="
echo -e "${PLAIN}"

# 1. 检查是否为 Root 用户
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}[错误] 必须使用 root 权限运行此安装脚本！${PLAIN}"
    exit 1
fi

# 2. 检查操作系统类型
if [[ -f /etc/os-release ]]; then
    . /etc/os-release
    OS=$ID
    OS_VERSION=$VERSION_ID
else
    echo -e "${RED}[错误] 无法识别当前操作系统发行版。${PLAIN}"
    exit 1
fi

echo -e "${CYAN}[1/6] 检查系统环境: ${OS} ${OS_VERSION}...${PLAIN}"
case "$OS" in
    ubuntu|debian)
        ;;
    *)
        echo -e "${YELLOW}[提示] 当前系统为 ${OS}，推荐使用 Debian 11/12/13 或 Ubuntu 22.04/24.04。脚本将继续尝试安装...${PLAIN}"
        ;;
esac

# 3. 检查系统架构 (amd64 / arm64)
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        TARGET_ARCH="amd64"
        ;;
    aarch64|arm64)
        TARGET_ARCH="arm64"
        ;;
    *)
        echo -e "${RED}[错误] 不支持的 CPU 架构: ${ARCH} (仅支持 amd64 / arm64)${PLAIN}"
        exit 1
        ;;
esac
echo -e "${GREEN}✓ CPU 架构匹配: ${TARGET_ARCH}${PLAIN}"

# 4. 创建系统必须目录
echo -e "${CYAN}[2/6] 创建系统所需目录...${PLAIN}"
mkdir -p /usr/local/bin
mkdir -p /etc/redis-manager/certificates
mkdir -p /var/lib/redis-manager/backups
mkdir -p /var/log/redis-manager
chmod 755 /etc/redis-manager/certificates
chmod 755 /var/lib/redis-manager

# ELF 二进制可执行文件校验函数
is_elf_binary() {
    local file="$1"
    if [[ ! -f "$file" ]] || [[ ! -s "$file" ]]; then
        return 1
    fi
    local magic
    magic=$(head -c 4 "$file" 2>/dev/null || true)
    if [[ "$magic" == $'\x7fELF' ]]; then
        return 0
    fi
    return 1
}

# 5. 下载或安装二进制可执行文件
echo -e "${CYAN}[3/6] 安装 Redis Manager 二进制程序...${PLAIN}"
BIN_PATH="/usr/local/bin/redis-manager"
INSTALL_SUCCESS=0

# 若本地当前目录存在 redis-manager，先检验其是否为有效 ELF 二进制
if [[ -f "./redis-manager" ]]; then
    if is_elf_binary "./redis-manager"; then
        echo -e "${BLUE}检测到当前目录存在有效的二进制安装包，直接安装本地二进制...${PLAIN}"
        cp -f ./redis-manager "$BIN_PATH"
        INSTALL_SUCCESS=1
    else
        echo -e "${YELLOW}[提示] 检测到当前目录存在 ./redis-manager 但不是有效的 Linux ELF 可执行程序 (可能为损坏的下载或 404 HTML 网页)，已自动清理...${PLAIN}"
        rm -f ./redis-manager
    fi
fi

# 检查上级目录的构建文件
if [[ $INSTALL_SUCCESS -eq 0 && -f "../redis-manager" ]]; then
    if is_elf_binary "../redis-manager"; then
        echo -e "${BLUE}检测到上级目录存在构建产物，安装本地二进制...${PLAIN}"
        cp -f ../redis-manager "$BIN_PATH"
        INSTALL_SUCCESS=1
    fi
fi

# 若本地无有效程序，则从 GitHub Release 下载
if [[ $INSTALL_SUCCESS -eq 0 ]]; then
    REPO="Grandova/Redis-Manager"
    TMP_TGZ="/tmp/redis-manager.tar.gz"
    TMP_BIN="/tmp/redis-manager-bin"
    rm -f "$TMP_TGZ" "$TMP_BIN" /tmp/redis-manager

    URL_TGZ="https://github.com/${REPO}/releases/latest/download/redis-manager-linux-${TARGET_ARCH}.tar.gz"
    URL_BIN="https://github.com/${REPO}/releases/latest/download/redis-manager-linux-${TARGET_ARCH}"

    echo -e "${BLUE}正在从 GitHub 下载最新版本...${PLAIN}"
    DOWNLOAD_OK=0

    # 方案 1: 下载 tar.gz 压缩包并解压
    if command -v curl &> /dev/null; then
        if curl -fSL --connect-timeout 15 --retry 2 "$URL_TGZ" -o "$TMP_TGZ" 2>/dev/null; then
            if tar -xzf "$TMP_TGZ" -C /tmp/ 2>/dev/null && is_elf_binary "/tmp/redis-manager"; then
                mv -f /tmp/redis-manager "$BIN_PATH"
                DOWNLOAD_OK=1
            fi
        fi
    elif command -v wget &> /dev/null; then
        if wget -q --timeout=15 --tries=2 "$URL_TGZ" -O "$TMP_TGZ" 2>/dev/null; then
            if tar -xzf "$TMP_TGZ" -C /tmp/ 2>/dev/null && is_elf_binary "/tmp/redis-manager"; then
                mv -f /tmp/redis-manager "$BIN_PATH"
                DOWNLOAD_OK=1
            fi
        fi
    fi
    rm -f "$TMP_TGZ"

    # 方案 2: 若 tar.gz 下载失败，尝试直接下载独立二进制文件
    if [[ $DOWNLOAD_OK -eq 0 ]]; then
        echo -e "${YELLOW}[提示] tar.gz 下载或解压未成功，尝试直接下载独立二进制文件...${PLAIN}"
        if command -v curl &> /dev/null; then
            curl -fSL --connect-timeout 15 --retry 2 "$URL_BIN" -o "$TMP_BIN" 2>/dev/null || true
        elif command -v wget &> /dev/null; then
            wget -q --timeout=15 --tries=2 "$URL_BIN" -O "$TMP_BIN" 2>/dev/null || true
        fi
        if is_elf_binary "$TMP_BIN"; then
            mv -f "$TMP_BIN" "$BIN_PATH"
            DOWNLOAD_OK=1
        fi
        rm -f "$TMP_BIN"
    fi

    if [[ $DOWNLOAD_OK -eq 0 ]]; then
        echo -e "${RED}[错误] 无法从 GitHub 下载到有效的可执行文件，请检查服务器外网网络连通性。${PLAIN}"
        exit 1
    fi
fi

chmod +x "$BIN_PATH"

# 最终校验安装后的二进制文件
if ! is_elf_binary "$BIN_PATH"; then
    echo -e "${RED}[错误] $BIN_PATH 校验失败，不是有效的 Linux 可执行程序！${PLAIN}"
    exit 1
fi
echo -e "${GREEN}✓ 主程序已成功部署并验证: ${BIN_PATH}${PLAIN}"

# 6. 配置 systemd 服务
echo -e "${CYAN}[4/6] 配置 systemd 系统服务...${PLAIN}"
SERVICE_FILE="/etc/systemd/system/redis-manager.service"
cat << 'EOF' > "$SERVICE_FILE"
[Unit]
Description=Redis Manager - Modern Web Redis Server & Browser
After=network.target network-online.target systemd-sysctl.service
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/var/lib/redis-manager
ExecStart=/usr/local/bin/redis-manager server
Restart=always
RestartSec=5s
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable redis-manager.service
echo -e "${GREEN}✓ 开机自启服务已注册${PLAIN}"

# 7. 启动服务并生成初始密码
echo -e "${CYAN}[5/6] 启动 Redis Manager 服务...${PLAIN}"
systemctl restart redis-manager.service

# 等待数据库初始化
sleep 2

# 检查服务是否启动成功
if ! systemctl is-active --quiet redis-manager.service; then
    echo -e "${YELLOW}[警告] redis-manager 服务尚未就绪，等待额外 2 秒...${PLAIN}"
    sleep 2
fi

if ! systemctl is-active --quiet redis-manager.service; then
    echo -e "${RED}[警告] redis-manager 服务未能正常启动，以下为最近服务日志：${PLAIN}"
    journalctl -u redis-manager.service -n 20 --no-pager || true
fi

# 8. 获取公网 IP 与初始化凭据
echo -e "${CYAN}[6/6] 生成管理员账户与访问地址...${PLAIN}"
SERVER_IP=$(curl -s --connect-timeout 3 https://api.ipify.org 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}')
if [[ -z "$SERVER_IP" ]]; then
    SERVER_IP="127.0.0.1"
fi

# 重置并提取全新的随机初始密码
INIT_PWD=$($BIN_PATH reset-password 2>/dev/null | grep "新密码:" | awk '{print $2}')
if [[ -z "$INIT_PWD" ]]; then
    INIT_PWD=$($BIN_PATH reset-password 2>/dev/null | grep "新密码:" | awk '{print $2}')
fi
if [[ -z "$INIT_PWD" ]]; then
    INIT_PWD="请执行 'redis-manager reset-password' 查看密码"
fi

echo -e "\n${GREEN}==========================================================${PLAIN}"
echo -e "${GREEN}             Redis Manager 安装成功！                     ${PLAIN}"
echo -e "${GREEN}==========================================================${PLAIN}"
echo -e "访问地址:"
echo -e "  ${CYAN}http://${SERVER_IP}:9080${PLAIN}"
echo -e "  ${CYAN}http://127.0.0.1:9080${PLAIN}"
echo -e ""
echo -e "管理员用户名:"
echo -e "  ${GREEN}admin${PLAIN}"
echo -e ""
echo -e "初始密码:"
echo -e "  ${YELLOW}${INIT_PWD}${PLAIN}"
echo -e ""
echo -e "常用 CLI 管理命令:"
echo -e "  ${BLUE}redis-manager status${PLAIN}          # 查看运行状态"
echo -e "  ${BLUE}redis-manager start${PLAIN}           # 启动服务"
echo -e "  ${BLUE}redis-manager stop${PLAIN}            # 停止服务"
echo -e "  ${BLUE}redis-manager restart${PLAIN}         # 重启服务"
echo -e "  ${BLUE}redis-manager reset-password${PLAIN}  # 重置管理员密码"
echo -e "  ${BLUE}redis-manager logs${PLAIN}            # 查看运行日志"
echo -e "  ${BLUE}redis-manager uninstall${PLAIN}       # 卸载面板"
echo -e "${GREEN}==========================================================${PLAIN}\n"
