# Redis Manager - 现代化 Web Redis 运维与可视化管理系统

<p align="center">
  <img src="frontend/public/favicon.svg" width="80" height="80" alt="Redis Manager Logo" />
</p>

<p align="center">
  <b>专为 Linux 原生服务器打造的现代化 Web Redis Server Manager</b><br/>
  将“宿主机 Redis 服务全生命周期管理”与“高性能 Redis 数据浏览器”融为一体
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%201.22+-00ADD8?style=flat-square&logo=go" />
  <img src="https://img.shields.io/badge/Frontend-React%2018%20%7C%20Vite%20%7C%20AntD-61DAFB?style=flat-square&logo=react" />
  <img src="https://img.shields.io/badge/Platform-Linux%20(amd64%20%2F%20arm64)-333333?style=flat-square&logo=linux" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" />
</p>

---

## 💡 项目定位

**Redis Manager** 不是单纯的 Redis 网页客户端（如 phpRedisAdmin），也不是老旧臃肿的面板插件，而是集成了：
- **宝塔 / 1Panel** 的宿主机底层服务生命周期管理、安装、升级、自动发现
- **Redis Insight / Redis Commander** 的现代化键值数据树状浏览、SCAN 游标查询、多类型编辑
- **真正的自动化 ACME / Let's Encrypt TLS** 证书签发、DNS 预检、端口冲突排查与双日自动续签
- **高可靠灾备引擎**：修改配置自动备份、启动异常零宕机自动回滚、恢复快照强制前置备份

---

## ✨ 核心特性

### 1. 🚀 原生 Redis 服务管理 (免 Docker)
- **智能接管**：自动探测宿主机已有 Redis（包括 `/usr/bin/redis-server`, `/etc/redis/redis.conf`, systemd 状态、PID 等），绝不强制重新安装。
- **一键安装**：支持在全新 Linux 服务器（Debian 11/12/13, Ubuntu 22.04/24.04）上一键安装官方稳定版 Redis 7.x、Redis 8.x 或系统最新版。
- **systemd 托管**：启停、重启、平滑重载、开机自启开关、journalctl 实时日志输出。

### 2. 🔒 一键自动申请与配置 TLS (ACME / Let's Encrypt)
- **零手动干预**：用户只需提前将域名 A/AAAA 记录解析到服务器，在面板输入域名并点击“开启 TLS”。
- **DNS 预检**：自动递归查询域名解析记录并与服务器公网 IP 比对，防止错误解析导致被 CA 封锁。
- **80 端口冲突检测**：自动排查是否有 nginx / caddy / apache 占用 80 端口，并给出友好提示。
- **全生命周期轮换**：后台每 12 小时自动巡检，在证书到期前 30 天自动完成 Let's Encrypt 续签、校验证书并热重载 Redis。
- **双模式支持**：
  - *仅 TLS (推荐)*：关闭普通明文连接，仅监听 6379 TLS 端口。
  - *双模式*：6379 保持普通明文 Redis，6380 监听 Redis TLS。

### 3. 🛡️ 配置安全与零宕机自动回滚
- **可视化设置**：网络（bind, port, timeout）、内存（maxmemory, maxmemory-policy）、客户端（maxclients）、持久化（save, appendonly）等核心参数直接修改。
- **原始 redis.conf 编辑**：支持在线编辑完整配置文件。
- **故障回滚保障**：修改前自动计算 SHA256 并生成 `.bak` 备份；启动后若探针探测失败，**系统自动瞬间恢复修改前配置**，绝不导致服务永久无法启动。

### 4. 📊 现代化监控与指标分析 (ECharts + SSE)
- **流式指标推送**：通过 Server-Sent Events (SSE) 每 1.5 秒更新一次内存消耗、QPS、命中率、在线连接数与网络吞吐。
- **历史趋势折线图**：支持 1 小时、6 小时、24 小时维度查看 QPS 吞吐、内存走势、网络 I/O 与缓存命中率。

### 5. 🔍 高性能数据管理 (严禁 KEYS *)
- **游标安全扫描**：全程使用 `SCAN` 分页分批检索，杜绝大 key 阻塞 Redis。
- **全类型支持**：String、Hash、List、Set、Sorted Set (ZSet)、Stream。
- **字段级编辑**：Hash 散列字段增删改、List 左右压入与弹出、Set 成员管理、ZSet 分数调整、Stream 条目查看。
- **TTL 控制**：查看倒计时、快捷修改 TTL、设为永久不过期。
- **批量操作**：批量选择并执行非阻塞 `UNLINK` 异步删除。

### 6. 💻 内置 Web Redis CLI
- 交互式命令行，支持命令历史（↑ ↓ 键切换）。
- **高危操作拦截**：对 `FLUSHALL`, `FLUSHDB`, `CONFIG SET`, `DEBUG`, `SHUTDOWN` 等危险指令进行强制弹窗拦截，必须二次确认才可执行。

### 7. 💾 数据备份与灾难恢复
- 支持调用 `BGSAVE` 自动归档 RDB 快照。
- 支持备份下载、删除与历史查看。
- **恢复防丢机制**：点击恢复任一历史备份前，系统强制前置创建当前最新状态快照，杜绝误操作。

---

## ⚡ 极速一键安装 (Linux)

在全新的一台 Linux 服务器（Ubuntu 或 Debian）上以 root 身份执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Grandova/Redis-Manager/main/scripts/install.sh | bash
```

安装脚本将自动：
1. 校验系统发行版与 CPU 架构 (amd64 / arm64)
2. 安装单二进制主程序至 `/usr/local/bin/redis-manager`
3. 创建配置目录 `/etc/redis-manager` 与数据目录 `/var/lib/redis-manager`
4. 注册并启动 systemd 开机自启服务
5. 随机生成强随机密码并输出面板访问地址：

```text
==========================================================
             Redis Manager 安装成功！                     
==========================================================
访问地址:
  http://你的服务器IP:9080
  http://127.0.0.1:9080

管理员用户名:
  admin

初始密码:
  xxxxxxxxxxxxxxxx
==========================================================
```

---

## 🛠️ CLI 运维管理命令

系统提供全局 `redis-manager` 终端命令：

```bash
# 查看面板及 Redis 运行状态
redis-manager status

# 启动面板服务
redis-manager start

# 停止面板服务
redis-manager stop

# 重启面板服务
redis-manager restart

# 重置并打印全新的管理员密码
redis-manager reset-password

# 跟踪查看面板实时日志
redis-manager logs

# 查看版本信息
redis-manager version

# 卸载面板
redis-manager uninstall
```

---

## 🏗️ 源码构建与开发

### 依赖环境
- Node.js 18+ & npm
- Go 1.22+

### 本地编译

```bash
# 1. 克隆代码仓库
git clone https://github.com/Grandova/Redis-Manager.git
cd Redis-Manager

# 2. 构建前端
cd frontend
npm install
npm run build
cd ..

# 3. 编译后端单二进制 (前端已自动通过 embed.FS 打包进二进制)
go build -ldflags "-s -w" -o redis-manager ./cmd/redis-manager

# 4. 运行服务
./redis-manager server
```

### 交叉编译 Linux 发行包 (无须 CGO)

由于底层采用了纯 Go 实现的 SQLite 驱动（`github.com/glebarez/sqlite`），在 Windows 或 macOS 上也可以直接交叉编译出免依赖的静态 Linux 二进制：

```bash
# 编译 Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/redis-manager-linux-amd64 ./cmd/redis-manager

# 编译 Linux arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/redis-manager-linux-arm64 ./cmd/redis-manager
```

---

## 🔒 安全规范

1. **零命令注入**：所有系统调用统一采用受控标准参数数组（`exec.Command(name, args...)`），严禁拼接 Shell 字符串。
2. **密码高强加密**：管理员身份密码采用工业级 Bcrypt 散列算法。
3. **防暴力破解**：内置 IP 频次熔断限速器，连续 5 次登录失败将强制锁定 5 分钟。
4. **日志隐私保护**：所有日志处理与审计记录自动对密码、私钥内容进行掩码脱敏。

---

## 📄 开源许可证

本项目采用 [MIT License](LICENSE) 开源。
