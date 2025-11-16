# ytextract 使用指南

## 快速开始

### 构建项目

```bash
cargo build --release
```

可执行文件位于 `target/release/ytextract`

### 基本用法

```bash
# 从播放列表提取视频链接
./target/release/ytextract "https://www.youtube.com/playlist?list=PLxxxxxx"

# 从含 list= 参数的视频链接提取
./target/release/ytextract "https://www.youtube.com/watch?v=VIDEO_ID&list=PLxxxxxx"

# 保存到文件
./target/release/ytextract "https://www.youtube.com/playlist?list=PLxxxxxx" > urls.txt

# 使用 cargo run（开发时）
cargo run -- "https://www.youtube.com/playlist?list=PLxxxxxx"
```

## 命令行选项

### --unique
对输出进行去重（默认已经在播放列表内去重）

```bash
./target/release/ytextract --unique "URL"
```

### --user-agent
自定义 User-Agent（用于绕过某些限制）

```bash
./target/release/ytextract --user-agent "CustomUA/1.0" "URL"
```

### --timeout
设置请求超时时间（秒），默认 20 秒

```bash
./target/release/ytextract --timeout 30 "URL"
```

### --concurrency
设置并发抓取数，默认 4

```bash
./target/release/ytextract --concurrency 8 "URL"
```

### --verbose / -v
显示详细调试信息到 STDERR

```bash
./target/release/ytextract -v "URL"
```

## 输出格式

程序输出为纯文本格式，每行一个视频 URL：

```
https://www.youtube.com/watch?v=VIDEO_ID_1
https://www.youtube.com/watch?v=VIDEO_ID_2
https://www.youtube.com/watch?v=VIDEO_ID_3
...
```

## 实际应用示例

### 1. 下载播放列表中的所有视频

结合 `yt-dlp` 使用：

```bash
./target/release/ytextract "PLAYLIST_URL" | xargs -I {} yt-dlp {}
```

或者：

```bash
./target/release/ytextract "PLAYLIST_URL" > urls.txt
yt-dlp -a urls.txt
```

### 2. 统计视频数量

```bash
./target/release/ytextract "PLAYLIST_URL" | wc -l
```

### 3. 去重并排序

```bash
./target/release/ytextract "PLAYLIST_URL" | sort -u
```

### 4. 提取视频 ID

```bash
./target/release/ytextract "PLAYLIST_URL" | sed 's/.*v=//'
```

### 5. 处理多个播放列表

```bash
# 如果输入页面包含多个播放列表，程序会自动发现并提取所有播放列表中的视频
./target/release/ytextract "PAGE_WITH_MULTIPLE_PLAYLISTS" --verbose
```

## 注意事项

1. **无需 API Key**：本工具通过解析 YouTube 页面获取数据，无需 API Key
2. **速率限制**：程序内置了合理的延迟和重试机制，避免触发 YouTube 的速率限制
3. **私有/受限内容**：无法访问的视频（年龄限制、地区限制、私有等）会被跳过
4. **网络问题**：如遇网络错误，程序会自动重试最多 3 次
5. **大型播放列表**：对于超大播放列表（数千个视频），处理可能需要几分钟时间

## 常见问题

### Q: 为什么某些视频没有被提取？
A: 可能原因：
- 视频已被删除或设为私有
- 地区限制
- 年龄限制
- YouTube 页面结构变化

使用 `--verbose` 查看详细日志以诊断问题。

### Q: 如何加快提取速度？
A: 
- 增加并发数：`--concurrency 8`
- 确保网络连接良好
- 注意：过高的并发可能触发速率限制

### Q: 输出中有重复的 URL 怎么办？
A: 使用 `--unique` 选项或用 `sort -u` 后处理

### Q: 支持哪些类型的 YouTube 链接？
A: 
- 播放列表：`https://www.youtube.com/playlist?list=PLxxxx`
- 视频（含播放列表参数）：`https://www.youtube.com/watch?v=xxx&list=PLxxxx`
- 任意含 `list=` 参数的页面

## 故障排查

### 编译错误
确保已安装最新的 Rust 工具链：

```bash
rustup update
```

### 运行时错误
1. 检查 URL 是否正确
2. 使用 `--verbose` 查看详细错误信息
3. 检查网络连接
4. 尝试增加 `--timeout` 值

### 提取结果为空
1. 确认播放列表是公开的
2. 检查 URL 是否包含 `list=` 参数
3. 使用 `--verbose` 查看调试信息

