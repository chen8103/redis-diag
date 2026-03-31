# redis-diag

`redis-diag` 是一个使用 Go 编写的 Redis 终端诊断工具，目标是提供类似 `innotop` 的多面板 TUI 体验，支持单实例和 Cluster，并内置 `MONITOR`、慢日志、命令统计和大 Key 采样分析。

## 功能概览

| 面板 | 说明 |
|------|------|
| Dashboard | 汇总关键 INFO 指标 |
| Clients | 客户端列表、IP 统计概览、排序、过滤 |
| Slowlog | latest / new 两种查看模式 |
| Replication | 主从状态与复制信息 |
| Keys | 带预算控制的抽样式 Big Keys 分析 |
| Commands | 命令累计调用量与增量 QPS |
| Monitor | 独立连接的实时命令流 |

支持单实例和 Cluster 按节点分组展示。

## 快速开始

### 构建

```bash
make build
```

### 运行

```bash
# 基本用法
./bin/redis-top -h 127.0.0.1 -P 6379

# 带密码
./bin/redis-top -h 127.0.0.1 -P 6379 -p mypassword

# Cluster 模式
./bin/redis-top --cluster -h 127.0.0.1 -P 7000

# 带 URI
./bin/redis-top --uri "redis://127.0.0.1:6379/0"
./bin/redis-top --uri "rediss://user:pass@example:6379/0" --insecure

# 启用 Monitor
./bin/redis-top --monitor -h 127.0.0.1 -P 6379
```

## 命令行参数

### 连接相关

| 长参数 | 短参数 | 默认值 | 说明 |
|--------|--------|--------|------|
| `--host` | `-h` | `127.0.0.1` | Redis 主机地址 |
| `--port` | `-P` | `6379` | Redis 端口 |
| `--uri` | | | Redis URI（优先于 host/port） |
| `--username` | | | Redis ACL 用户名 |
| `--password` | `-p` | | Redis 密码 |
| `--tls` | | `false` | 启用 TLS |
| `--insecure` | | `false` | 跳过 TLS 证书验证 |
| `--cluster` | | `false` | 强制 Cluster 模式 |
| `--startup-timeout` | | `3s` | 启动连接超时 |

### 通用相关

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--refresh` | `1s` | 全局刷新间隔（Dashboard/Clients/Commands） |
| `--clients-limit` | `20` | Clients 面板最大显示行数 |
| `--slowlog-limit` | `20` | Slowlog 面板最大显示行数 |
| `--version` | | 打印版本号 |

### Keys 面板相关

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--keys-interval` | `15s` | Big Keys 面板刷新间隔 |
| `--keys-timeout` | `1.5s` | 每轮 Big Keys 扫描时间预算 |
| `--keys-topk` | `20` | 展示多少个大 Key |
| `--keys-scan-count` | `200` | 每次 SCAN 的 count |
| `--keys-sample-max` | `200` | 每轮最多采样多少个 key |

### Monitor 面板相关

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--monitor` | `false` | 启动时自动启用 Monitor |
| `--monitor-buffer` | `2000` | Ring buffer 容量 |
| `--monitor-ui-refresh` | `300ms` | Monitor 面板刷新频率 |
| `--monitor-window` | `5s` | 聚合时间窗口大小 |
| `--monitor-mask-args` | `true` | 是否脱敏命令参数 |
| `--monitor-max-arg-len` | `128` | 参数显示最大长度 |
| `--monitor-selected-node-only` | `true` | 是否限制为当前选中节点 |

## 面板显示逻辑

### Dashboard（面板 1）

显示 Redis `INFO` 命令返回的关键指标：

- **版本信息**：Redis 版本、运行角色
- **客户端统计**：连接客户端数、每秒操作数
- **内存统计**：使用内存、RSS、命中率
- **键空间统计**：过期键、驱逐键、各数据库 key 数量

**刷新逻辑**：每 `--refresh` 间隔刷新一次。

---

### Clients（面板 2）

显示当前连接的客户端信息。

**视图模式**：

1. **Clients 视图**（默认）：显示每个客户端的详细信息
   - ID、地址、idle 时间、age、当前执行的命令、输出缓冲区内存

2. **IP 视图**：按 IP 汇总连接数统计
   - 显示每个 IP 的连接数量，按数量降序排列

**显示内容**：

```
sort=idle filter="" view=clients

  connected=5 shown=5 limited=false quality=exact

  Clients:
    id=1     addr=10.0.0.1:5000    idle=3     age=10     cmd=get       omem=0
    ...

  IP Overview:
    10.0.0.1:          3
    10.0.0.2:          2
```

**刷新逻辑**：每 `--refresh` 间隔刷新一次。如果连接数 > 5000 且 5 秒内已刷新过，则跳过。

---

### Slowlog（面板 3）

显示 Redis 慢查询日志条目。

**模式切换**：

| 模式 | 说明 |
|------|------|
| `latest` | 显示所有慢查询条目 |
| `new` | 只显示自上次查看后新出现的慢查询（过滤已读过的） |

**显示内容**：

```
mode=latest

  id=1     duration_us=1000      at=2024-01-01T12:00:00Z   cmd=get a
  ...
```

**刷新逻辑**：每 2 秒刷新一次。

---

### Replication（面板 4）

显示主从复制状态和 replica 信息。

**显示内容**：

- 角色（master/replica）
- 主节点地址、连接状态、最后 IO 间隔
- 复制偏移量
- 各 replica 节点状态、偏移量、延迟

**刷新逻辑**：每 2 秒刷新一次。

---

### Keys（面板 5）

抽样分析 Redis 中的大 Key。

**采样逻辑**：

```
for !deadline && sampled < sampleMax:
    SCAN cursor "*" count=200
    for each key in keys:
        if deadline expired: break
        estimate size via MEMORY USAGE or TYPE+LLEN/SCARD/HLEN/etc
        record to candidates
    if cursor == 0: break  # 遍历完成
return top-k by size
```

**大小估算方式**：

| 类型 | 优先命令 | 回退命令 | 单位 |
|------|---------|---------|------|
| string | MEMORY USAGE | STRLEN | bytes |
| hash | MEMORY USAGE | HLEN | fields |
| list | MEMORY USAGE | LLEN | items |
| set | MEMORY USAGE | SCARD | members |
| zset | MEMORY USAGE | ZCARD | members |
| stream | MEMORY USAGE | XLEN | entries |

**显示内容**：

```
quality=sampled sampled=200 topk=20 budget=1.5s

  key1                 type=string   size=1048576    unit=bytes    via=MEMORY USAGE
  key2                 type=hash    size=10000      unit=fields   via=hash
  ...
```

**特点**：

- 是抽样近似结果，不保证全量准确
- 如果超时但 SCAN 未完成，会记录 `RemainingCursor`，下次继续
- 受 `--keys-sample-max` 限制，大 key 可能被遗漏

**刷新逻辑**：每 `--keys-interval`（默认 15s）刷新一次。

---

### Commands（面板 6）

显示命令统计和增量 QPS。

**显示内容**：

- 每条命令的累计调用次数、总耗时、平均耗时
- 增量 QPS（两次采样间隔内的平均每秒调用数）
- 首次采样时显示 "warming up delta baseline..."

**QPS 计算**：

```
QPSDelta = (calls_current - calls_previous) / elapsed_seconds
```

**刷新逻辑**：每 `--refresh` 间隔刷新一次。

---

### Monitor（面板 7）

实时显示 Redis 命令执行流。

**功能**：

- 独立连接 MONITOR 命令
- 解析 Redis MONITOR 输出
- Ring buffer 存储，支持时间窗口聚合

**聚合统计**：

```
window_reads=N    # 读命令数量（get/mget/exists/ttl/hget/hmget/scard/zrange）
window_writes=N  # 写命令数量
```

按 `cmd` 或 `addr` 统计命令频次。

**显示内容**：

```
running=true node=10.0.0.1:6379
window_reads=100 window_writes=50

2024-01-01T12:00:00Z db=0 addr=10.0.0.1:5000 cmd=get args=<redacted>
2024-01-01T12:00:00Z db=0 addr=10.0.0.1:5001 cmd=set args=<redacted>
...
```

**刷新逻辑**：每 `--monitor-ui-refresh`（默认 300ms）刷新一次。

---

## 操作方法

### 切换面板

| 按键 | 功能 |
|------|------|
| `1` | Dashboard |
| `2` | Clients |
| `3` | Slowlog |
| `4` | Replication |
| `5` | Keys |
| `6` | Commands |
| `7` | Monitor |

### Clients 面板操作

| 按键 | 功能 |
|------|------|
| `s` | 切换排序：idle → age → cmd → omem |
| `f` | 设置过滤条件（地址/命令关键字） |
| `i` | 切换视图：clients ↔ ip |

### Slowlog 面板操作

| 按键 | 功能 |
|------|------|
| `m` | 切换模式：latest ↔ new |

### Monitor 面板操作

| 按键 | 功能 |
|------|------|
| `m` | 启动/停止 Monitor |

### 通用操作

| 按键 | 功能 |
|------|------|
| `Tab` | 切换当前选中节点（Cluster 模式） |
| `r` | 手动刷新当前面板 |
| `q` | 退出程序 |
| `Ctrl+C` | 强制退出 |

## 发布构建

生成多平台发布产物到 `dist/`：

```bash
make release
```

默认会生成：

- `darwin/arm64`
- `darwin/amd64`
- `linux/amd64`
- `linux/arm64`

可覆盖版本号：

```bash
make release VERSION=v0.1.0
```

## 开发命令

```bash
make build   # 构建
make test    # 测试
make tidy    # 整理依赖
make clean   # 清理
```

## 注意事项

- `Monitor`、`Slowlog`、`Keys` 面板都可能暴露业务数据，生产环境使用时请注意屏幕和录屏内容。
- 默认情况下 `Monitor` 参数会脱敏，但命令名、客户端地址、key 名等信息仍可能暴露敏感线索。
- 通过 `--password` 或带密码的 `--uri` 启动时，凭据可能出现在 shell history 或进程列表里。
- `Big Keys` 是抽样近似结果，不保证全量准确。
- `--insecure` 只适合测试环境。
