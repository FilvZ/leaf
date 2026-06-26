# Leaf — Go 游戏服务器框架

**版本**: 1.1.3  |  **协议**: Apache License 2.0  |  **语言**: Go (golang)

Leaf 是一个由 Go 语言编写的**开发效率和执行效率并重**的开源游戏服务器框架。它适用于各类游戏服务器的开发，包括 H5（HTML5）游戏服务器。

> 项目主页: [github.com/name5566/leaf](https://github.com/name5566/leaf)

---

## 核心设计理念

- **极简使用体验** — 提供简洁直观的接口，降低学习和开发成本
- **稳定性优先** — 自动恢复运行时错误，避免服务器崩溃
- **多核支持** — 通过模块机制和 goroutine 管理，高效利用多核资源
- **模块化架构** — 每个模块独立 goroutine，通过轻量 RPC 通信

---

## 项目架构总览

```
leaf/
├── leaf.go            # 框架入口，模块注册与生命周期管理
├── version.go         # 版本定义 (v1.1.3)
├── conf/              # 全局配置项
├── module/            # 模块系统 + 骨架 (Skeleton)
├── chanrpc/           # 基于 channel 的 RPC 通信
├── network/           # 网络层 (TCP / WebSocket)
│   ├── json/          # JSON 消息处理器
│   └── protobuf/      # Protobuf 消息处理器
├── gate/              # 网关模块 (客户端接入)
├── cluster/           # 集群模块 (服务器间通信)
├── go/                # goroutine 管理
├── timer/             # 定时器 + Cron 表达式
├── log/               # 分级日志系统
├── console/           # 远程调试控制台
├── db/mongodb/        # MongoDB 数据库封装
├── db/pgsql/          # PostgreSQL 数据库封装
├── recordfile/        # 游戏配置数据 (CSV) 管理
└── util/              # 工具库
```

---

## 启动流程

框架入口位于 `leaf.go` 的 `Run()` 函数：

1. **初始化日志** — 根据 `conf.LogLevel`/`LogPath`/`LogFlag` 创建日志记录器
2. **注册模块** — 按顺序注册传入的 `module.Module` 实例
3. **初始化模块** — 同一 goroutine 中按注册顺序依次调用 `OnInit()`
4. **启动模块** — 每个模块启动独立 goroutine 执行 `Run()`
5. **初始化集群** — 启动 `cluster.Init()`
6. **启动控制台** — 启动 `console.Init()`（TCP 远程管理）
7. **等待关闭信号** — 监听 `os.Interrupt` / `os.Kill`
8. **优雅关闭** — 逆序销毁控制台 → 集群 → 模块

```go
leaf.Run(
    game.Module,
    gate.Module,
    login.Module,
)
```


## 模块系统

### Module 接口

```go
type Module interface {
    OnInit()
    OnDestroy()
    Run(closeSig chan bool)
}
```

- `OnInit()` — 初始化逻辑，在同一 goroutine 中顺序执行
- `Run(closeSig)` — 主循环，收到 `closeSig` 后退出
- `OnDestroy()` — 清理逻辑，逆序执行

### Skeleton (模块骨架)

`Skeleton` 是为模块提供的便捷组合体，集成了四项核心能力：

| 字段 | 用途 |
|------|------|
| `ChanRPCServer` | 模块间 RPC 通信 |
| `g` (Go) | goroutine 管理 + 回调 |
| `dispatcher` (Timer) | 定时器 + Cron 任务 |
| `client` (ChanRPC) | 异步 RPC 调用 |

```go
type Skeleton struct {
    GoLen              int           // Go 通道缓冲长度
    TimerDispatcherLen int           // 定时器通道缓冲长度
    AsynCallLen        int           // 异步调用通道缓冲长度
    ChanRPCServer      *chanrpc.Server // 可外部注入的 RPC 服务
}
```

Skeleton 的主循环使用 `select` 多路复用处理：
- 异步 RPC 返回结果
- 模块内 RPC 调用请求
- 控制台命令请求
- goroutine 回调
- 定时器到期

---

## 模块详解

### 1. chanrpc — 基于 Channel 的 RPC

模块间通信的核心机制，每个 `Server` 运行在一个 goroutine 中，通过 channel 传递调用信息。

**Server 端 (注册函数)**:

```go
s := chanrpc.NewServer(10)    // 参数为 channel 缓冲长度
s.Register("add", func(args []interface{}) interface{} {
    n1 := args[0].(int)
    n2 := args[1].(int)
    return n1 + n2
})
// 在循环中执行
for { s.Exec(<-s.ChanCall) }
```

**Client 端 (调用)**:

| 方法 | 返回值 | 适用函数签名 |
|------|--------|------------|
| `Call0` | `error` | `func([]interface{})` |
| `Call1` | `(interface{}, error)` | `func([]interface{}) interface{}` |
| `CallN` | `([]interface{}, error)` | `func([]interface{}) []interface{}` |
| `AsynCall` | 回调驱动 | 以上三种均可 |
| `Go` | 无返回值 | 异步投递，不等待结果 |

支持的函数签名：
- `func(args []interface{})`
- `func(args []interface{}) interface{}`
- `func(args []interface{}) []interface{}`

### 2. go — goroutine 管理

提供带回调的 goroutine 管理，确保回调在模块主循环中安全执行。

**并发 goroutine**:
```go
d := g.New(10)               // 创建，参数为回调通道缓冲长度
d.Go(func() {
    // 并发执行的任务
}, func() {
    // 完成后在主循环中执行的回调
})
```

**线性上下文 (LinearContext)**:
```go
c := d.NewLinearContext()    // 创建线性上下文
c.Go(func() { /* 任务 1 */ }, nil)
c.Go(func() { /* 任务 2 */ }, nil)  // 任务 2 等待任务 1 完成后才执行
```

LinearContext 保证同一上下文中的多个 goroutine **顺序执行**，适用于需要串行化处理的场景。

### 3. timer — 定时器

**一次性定时器**:
```go
d := timer.NewDispatcher(10)
t := d.AfterFunc(5*time.Second, func() {
    // 5 秒后执行
})
t.Stop()  // 取消定时器
```

**Cron 定时器**:
```go
cronExpr, _ := timer.NewCronExpr("0 30 * * * *")  // 每分钟的 30 秒
c := d.CronFunc(cronExpr, func() {
    // 定时执行
})
c.Stop()
```

Cron 表达式格式 (6 字段): `秒 分 时 日 月 周`

| 字段 | 必填 | 取值范围 |
|------|------|---------|
| 秒 (Seconds) | 否 | 0-59 |
| 分 (Minutes) | 是 | 0-59 |
| 时 (Hours) | 是 | 0-23 |
| 日 (Day of month) | 是 | 1-31 |
| 月 (Month) | 是 | 1-12 |
| 周 (Day of week) | 是 | 0-6 |

支持特殊字符: `*` `,` `-` `/`

### 4. log — 分级日志

| 级别 | 函数 | 说明 |
|------|------|------|
| Debug | `log.Debug()` | 调试信息 |
| Release | `log.Release()` | 正式输出 |
| Error | `log.Error()` | 错误信息 |
| Fatal | `log.Fatal()` | 致命错误，输出后调用 `os.Exit(1)` |

- 支持输出到文件（自动按时间命名）或 stdout
- 可通过 `log.New(level, path, flag)` 创建独立 Logger
- 通过 `log.Export(logger)` 切换全局日志实例
- 日志级别过滤：低于设定级别的日志不会输出

### 5. conf — 全局配置

```go
var (
    LenStackBuf = 4096          // 堆栈缓冲区大小
    LogLevel    string          // 日志级别: debug / release / error / fatal
    LogPath     string          // 日志文件路径（空则输出到 stdout）
    LogFlag     int             // Go log 标准库 flag
    ConsolePort int             // 控制台 TCP 端口
    ConsolePrompt string = "Leaf# "  // 控制台提示符
    ProfilePath string          // pprof 文件输出路径
    ListenAddr  string          // 集群监听地址
    ConnAddrs   []string        // 集群连接地址列表
    PendingWriteNum int         // 网络写入缓冲数
)
```

### 6. console — 远程控制台

通过 TCP 连接远程管理服务器，内置命令：

| 命令 | 功能 |
|------|------|
| `help` | 显示所有可用命令 |
| `cpuprof start` | 启动 CPU 性能分析 |
| `cpuprof stop` | 停止 CPU 性能分析 |
| `prof goroutine` | 导出 goroutine 堆栈信息 |
| `prof heap` | 导出堆内存采样 |
| `prof thread` | 导出 OS 线程创建信息 |
| `prof block` | 导出同步阻塞信息 |
| `quit` | 退出控制台 |

**注册自定义命令**:
```go
skeleton.RegisterCommand("kick", "kick a player", func(args []interface{}) string {
    // 自定义命令逻辑
    return "ok"
})
```

### 7. gate — 网关模块

负责客户端连接管理，同时支持 **TCP** 和 **WebSocket** 协议。

```go
type Gate struct {
    MaxConnNum      int               // 最大连接数
    PendingWriteNum int               // 写入缓冲
    MaxMsgLen       uint32            // 最大消息长度
    Processor       network.Processor // 消息处理器
    AgentChanRPC    *chanrpc.Server   // Agent 事件 RPC
    WSAddr          string            // WebSocket 监听地址
    TCPAddr         string            // TCP 监听地址
    CertFile        string            // TLS 证书文件
    KeyFile         string            // TLS 密钥文件
}
```

**网关 Agent 接口**:
```go
type Agent interface {
    WriteMsg(msg interface{})
    LocalAddr() net.Addr
    RemoteAddr() net.Addr
    Close()
    Destroy()
    UserData() interface{}
    SetUserData(data interface{})
}
```

消息处理流程：
1. 客户端通过 TCP 或 WebSocket 连接
2. 接收到消息后通过 `Processor.Unmarshal()` 反序列化
3. 通过 `Processor.Route()` 路由到对应模块
4. 模块处理完成后通过 `Agent.WriteMsg()` 回发

### 8. network — 网络层

#### 协议支持

- **TCP** — 长度前缀帧 (`|len|data|`)，支持自定义长度字节数 (1/2/4)
- **WebSocket** — 基于 `gorilla/websocket`，支持 TLS/WSS

#### TCP 服务器
```go
server := new(network.TCPServer)
server.Addr = "0.0.0.0:3563"
server.MaxConnNum = 100
server.NewAgent = func(conn *network.TCPConn) network.Agent {
    return &MyAgent{conn: conn}
}
server.Start()
```

#### TCP 客户端 (支持自动重连)
```go
client := new(network.TCPClient)
client.Addr = "127.0.0.1:3563"
client.AutoReconnect = true
client.NewAgent = func(conn *network.TCPConn) network.Agent {
    return &MyAgent{conn: conn}
}
client.Start()
```

#### WebSocket 服务器
```go
wsServer := new(network.WSServer)
wsServer.Addr = "0.0.0.0:3563"
wsServer.NewAgent = func(conn *network.WSConn) network.Agent {
    return &MyAgent{conn: conn}
}
wsServer.Start()
```

#### Processor 接口

```go
type Processor interface {
    Route(msg interface{}, userData interface{}) error
    Unmarshal(data []byte) (interface{}, error)
    Marshal(msg interface{}) ([][]byte, error)
}
```

Leaf 内置两种 Processor 实现：

**JSON 处理器** (`network/json`):
- 消息格式: `{"MsgName": {...}}`
- 消息 ID 为结构体名称
- 支持 `Register()` → `SetRouter()` → 模块自动路由

**Protobuf 处理器** (`network/protobuf`):
- 消息格式: `|uint16 ID|protobuf bytes|`
- ID 为自动分配的 `uint16` 序号
- 支持大端/小端字节序

### 9. cluster — 集群模块

基于 TCP 的服务器间通信：
- 通过 `conf.ListenAddr` 配置监听地址
- 通过 `conf.ConnAddrs` 配置需要连接的服务器地址列表
- 使用 4 字节长度前缀的消息格式，支持最大 `math.MaxUint32` 的消息长度

### 10. db/mongodb — 数据库

基于 `mgo.v2` 驱动的 MongoDB 封装，提供：

**连接池管理**:
```go
c, _ := mongodb.Dial("localhost", 100)  // 100 个 session 的连接池
s := c.Ref()       // 获取一个 session（引用计数 +1）
defer c.UnRef(s)    // 归还 session（引用计数 -1）
```

通过最小堆管理 session 分配，优先使用引用计数最少的 session。

**自增序列**:
```go
c.EnsureCounter("db", "counters", "userid")  // 确保计数器存在
id, _ := c.NextSeq("db", "counters", "userid") // 获取下一个 ID
```

**索引管理**:
```go
c.EnsureIndex("db", "collection", []string{"key"})       // 普通索引
c.EnsureUniqueIndex("db", "collection", []string{"key"})  // 唯一索引
```



### 11. db/pgsql — PostgreSQL 数据库

基于 `database/sql` + `github.com/lib/pq` 驱动的 PostgreSQL 封装。

**连接池管理**:
```go
// DSN 格式: postgres://user:password@host:port/dbname?sslmode=disable
c, _ := pgsql.Dial("postgres://localhost:5432/test?sslmode=disable", 100)
defer c.Close()
```

`DialContext` 包装了 `*sql.DB`，复用 Go 标准库的连接池能力：
- `SetMaxOpenConns(maxOpenConns)`
- `SetMaxIdleConns(maxOpenConns / 4)`
- `SetConnMaxLifetime(maxLifetime)`

**自增序列** (基于 PostgreSQL SEQUENCE):
```go
c.EnsureSequence("user_id_seq")          // 创建序列
id, _ := c.NextSeq("user_id_seq")        // 获取下一个值
c.DropSequence("user_id_seq")            // 删除序列
```

**表和索引管理**:
```go
c.EnsureTable("players", "id BIGSERIAL PRIMARY KEY, name TEXT")
c.EnsureIndex("players", []string{"level"})
c.EnsureUniqueIndex("players", []string{"name"})
c.DropTable("players")
```

**直接执行查询**:
```go
result, _ := c.Exec("INSERT INTO players (name) VALUES ($1)", "leaf")
row := c.QueryRow("SELECT name FROM players WHERE id = $1", 1)
rows, _ := c.Query("SELECT * FROM players")
```

底层 `*sql.DB` 可通过 `c.DB()` 获取，用于更复杂的数据库操作。
### 11. recordfile — 配置数据管理

基于 CSV/TSV 的静态游戏数据管理，支持结构体字段绑定和索引。

- 分隔符: 默认 Tab (`\t`)，可通过 `Comma` 字段修改
- 注释行: 默认 `#`
- 支持字段类型: bool, int 系列, uint 系列, float 系列, string, struct, array, slice, map
- 支持 JSON 格式的复杂类型解析

```go
type Record struct {
    ID    int    "index"     // 索引字段
    Name  string "index"     // 多索引支持
    Value int32
}
rf, _ := recordfile.New(Record{})
rf.Read("data.txt")
r := rf.Index(1).(*Record)           // 通过第一个索引字段查找
r = rf.Indexes(1)["name"].(*Record)  // 通过第二个索引字段查找
```

### 12. util — 工具库

| 模块 | 功能 |
|------|------|
| `Map` | 并发安全的键值映射，支持 RLock/Lock 粒度的 Range 遍历 |
| `DeepCopy` | 基于反射的深度拷贝，支持 `deepcopy:"-"` tag 排除字段 |
| `DeepClone` | 深度克隆并返回新对象 |
| `RandGroup` | 按权重随机选择分组，返回选中下标 |
| `RandInterval` | 在 [b1, b2] 区间内随机取整 |
| `RandIntervalN` | 从区间中随机抽取 N 个不重复整数 |
| `Semaphore` | 基于 channel 的信号量实现 |

---

## 消息处理流程 (完整链路)

```
客户端 ──► Gate (TCP/WS)
              │
              ▼
       Processor.Unmarshal()   ← 解码消息
              │
              ▼
       Processor.Route()       ← 路由到目标模块
              │
              ▼
       模块 Handler 处理        ← 业务逻辑
              │
              ▼
       Agent.WriteMsg()        ← 编码并回发
              │
              ▼
          客户端
```


## 良好的实践建议

- **模块数量不宜过多** — Leaf 不建议设计过多模块，每个模块独立 goroutine 有调度开销
- **优先使用 Skeleton** — Skeleton 集成了 Go / Timer / ChanRPC / Console，简化模块开发
- **善用 LinearContext** — 需要顺序执行 goroutine 时使用线性上下文替代手动同步
- **合理配置缓冲长度** — 根据消息量调整 `GoLen`、`TimerDispatcherLen`、`AsynCallLen`
- **使用 Processor 路由** — 避免在 gate 中处理业务逻辑，通过 Processor 路由到专门模块

---

## 依赖项

| 包 | 用途 |
|----|------|
| `github.com/gorilla/websocket` | WebSocket 协议支持 |
| `github.com/golang/protobuf` | Protocol Buffers 支持 |
| `gopkg.in/mgo.v2` | MongoDB 驱动 |

---

## License

Leaf 基于 Apache License, Version 2.0 开源协议。
