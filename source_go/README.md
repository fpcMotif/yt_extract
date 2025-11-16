# ytextract - Go Implementation

这是 ytextract 的 Go 语言实现版本，与 Rust 版本功能完全对等。

## 功能特性

- ✅ 无需 API Key，通过解析 YouTube 页面提取视频链接
- ✅ 支持播放列表分页（自动获取所有视频）
- ✅ 并发处理多个播放列表（默认并发数 4）
- ✅ 自动重试机制（失败最多重试 3 次）
- ✅ 完整的错误处理和容错
- ✅ 支持 verbose 调试模式
- ✅ 纯 Go 标准库实现（仅依赖 golang.org/x/sync）

## 构建

```bash
cd source_go
go build -o bin/ytextract ./cmd/ytextract
```

## 使用

### 基本用法

```bash
# 从播放列表提取视频链接
./bin/ytextract "https://www.youtube.com/playlist?list=PLxxxxxx"

# 从含 list= 参数的视频链接提取
./bin/ytextract "https://www.youtube.com/watch?v=VIDEO_ID&list=PLxxxxxx"

# 保存到文件
./bin/ytextract "https://www.youtube.com/playlist?list=PLxxxxxx" > urls.txt
```

### 命令行选项

```bash
# 去重输出
./bin/ytextract --unique "URL"

# 自定义 User-Agent
./bin/ytextract --user-agent "CustomUA/1.0" "URL"

# 设置超时（秒）
./bin/ytextract --timeout 30 "URL"

# 设置并发数
./bin/ytextract --concurrency 8 "URL"

# 显示详细调试信息
./bin/ytextract -v "URL"
./bin/ytextract --verbose "URL"
```

## 输出格式

程序输出为纯文本格式，每行一个视频 URL：

```
https://www.youtube.com/watch?v=VIDEO_ID_1
https://www.youtube.com/watch?v=VIDEO_ID_2
https://www.youtube.com/watch?v=VIDEO_ID_3
...
```

调试信息输出到 STDERR，不影响 STDOUT 的干净输出。

## 与 Rust 版本的对比

### 相同点
- 完全相同的命令行参数和默认值
- 相同的输出格式和错误信息
- 相同的并发策略和重试机制
- 相同的分页处理和去重逻辑

### 实现差异
- Go 版本使用标准库 `net/http`，Rust 版本使用 `reqwest`
- Go 版本使用 `golang.org/x/sync/errgroup` 控制并发，Rust 版本使用 `futures::stream`
- Go 版本使用 `flag` 解析参数，Rust 版本使用 `clap`

### 性能特点
- Go 版本：更快的编译速度，较小的二进制体积（~8-10MB）
- Rust 版本：更小的运行时内存占用
- 两者运行时性能在实际使用中基本相当

## 架构设计

```
source_go/
├── go.mod                          # Go 模块定义
├── cmd/ytextract/main.go           # CLI 入口
├── internal/
│   ├── extract/extract.go          # HTTP 客户端和播放列表发现
│   └── innertube/innertube.go      # YouTube Innertube API 解析
└── bin/                            # 编译输出目录
```

## 技术实现

### HTTP 客户端
- 使用 `net/http.Client` + 自定义 `Transport`
- 支持 gzip/deflate 自动解压
- 配置连接池和超时参数
- 通过自定义 `RoundTripper` 设置 User-Agent

### 播放列表发现
- 使用 `net/url` 解析 URL 参数
- 使用 `regexp` 在 HTML 中匹配播放列表 ID
- 使用 map 去重，slice 保持顺序

### Innertube API
- 正则提取 `INNERTUBE_API_KEY` 和 `INNERTUBE_CONTEXT`
- 递归遍历 JSON 树提取 `videoId` 和 `continuation`
- 分页请求使用 POST 方法调用 `/youtubei/v1/browse`

### 并发控制
- 使用 `golang.org/x/sync/errgroup` 管理 goroutine
- `SetLimit()` 限制并发数（等价于 Rust 的 `buffer_unordered`）
- 每个播放列表独立处理，失败不影响其他

### 错误处理
- 自动重试：最多 3 次，指数退避
- 容错机制：单个播放列表失败返回空列表而不中断
- 错误包装：使用 `fmt.Errorf` 提供上下文信息

## 依赖

- Go 1.21+
- `golang.org/x/sync` - errgroup 并发控制

## 局限性

与 Rust 版本相同：
1. 依赖 YouTube 页面结构，结构变化需要更新代码
2. 无法获取地区限制/年龄限制/私有内容
3. 过高并发可能触发速率限制（建议 ≤ 8）

## 许可证

与主项目保持一致

