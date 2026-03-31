# redis-diag

`redis-diag` 是一个使用 Go 编写的 Redis 终端诊断工具，目标是提供类似 `innotop` 的多面板 TUI 体验，支持单实例和 Cluster，并内置 `MONITOR`、慢日志、命令统计和大 Key 采样分析。

## 功能概览

- `Dashboard`：汇总关键 `INFO` 指标
- `Clients`：客户端列表、排序、过滤
- `Slowlog`：`latest` / `new` 两种查看模式
- `Replication`：主从状态与复制信息
- `Keys`：带预算控制的抽样式 Big Keys 分析
- `Commands`：命令累计调用量与增量 QPS
- `Monitor`：独立连接的实时命令流
- 支持单实例和 Cluster 按节点分组展示

## 本地构建

直接构建当前平台二进制：

```bash
make build
```

或者：

```bash
go build ./cmd/redis-top
```

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

产物会放在 `dist/` 目录下，文件名格式类似：

- `dist/redis-top-dev-darwin-arm64`
- `dist/redis-top-v0.1.0-linux-amd64`

可覆盖版本号：

```bash
make release VERSION=v0.1.0
```

## 运行示例

```bash
./bin/redis-top --host 127.0.0.1 --port 6379
./bin/redis-top --uri "redis://127.0.0.1:6379/0"
./bin/redis-top --uri "rediss://user:pass@example:6379/0" --insecure
./bin/redis-top --cluster --host 127.0.0.1 --port 7000
```

## 常用参数

### 连接相关

- `--host`、`--port`
- `--uri`
- `--username`、`--password`
- `--tls`、`--insecure`
- `--cluster`

### 通用相关

- `--refresh`
- `--clients-limit`
- `--slowlog-limit`
- `--monitor`
- `--version`

### Keys 面板相关

- `--keys-interval`：Big Keys 面板刷新间隔
- `--keys-timeout`：每轮 Big Keys 扫描预算
- `--keys-topk`：展示多少个大 Key
- `--keys-scan-count`：每次 `SCAN` 的 count
- `--keys-sample-max`：每轮最多采样多少个 key

### Monitor 面板相关

- `--monitor-buffer`：MONITOR ring buffer 容量
- `--monitor-ui-refresh`：MONITOR 面板刷新频率
- `--monitor-window`：MONITOR 聚合窗口大小
- `--monitor-mask-args`：是否脱敏参数
- `--monitor-max-arg-len`：参数显示最大长度
- `--monitor-selected-node-only`：是否限制为当前选中节点

## 键位说明

- `1`：Dashboard
- `2`：Clients
- `3`：Slowlog
- `4`：Replication
- `5`：Keys
- `6`：Commands
- `7`：Monitor
- `s`：切换 Clients 排序
- `f`：设置 Clients 过滤条件
- `m`：切换 Slowlog 模式，或启动/停止 Monitor
- `Tab`：切换当前选中节点
- `q`：退出

## 开发命令

```bash
make test
make tidy
make clean
```

## 注意事项

- `Monitor`、`Slowlog`、`Keys` 面板都可能暴露业务数据，生产环境使用时请注意屏幕和录屏内容。
- 默认情况下 `Monitor` 参数会脱敏，但命令名、客户端地址、key 名等信息仍可能暴露敏感线索。
- 通过 `--password` 或带密码的 `--uri` 启动时，凭据可能出现在 shell history 或进程列表里。
- `Big Keys` 是抽样近似结果，不保证全量准确。
- `--insecure` 只适合测试环境。
