# ytextract

一个 Rust CLI 工具，从 YouTube 播放列表中提取所有视频链接。

## 使用方法

```bash
# 构建
cargo build --release

# 运行
./target/release/ytextract "https://www.youtube.com/playlist?list=PLxxxx" > urls.txt

# 或者直接用 cargo run
cargo run -- "https://www.youtube.com/watch?v=VIDEO&list=PLxxxx"
```

## 功能

- 支持任意含 `list=` 参数的 YouTube 链接
- 支持播放列表页面
- 自动发现页面中的所有播放列表并提取全部视频
- 输出干净的 URL 格式：`https://www.youtube.com/watch?v=VIDEO_ID`
- 无需 API Key

## 选项

- `--unique`: 对输出去重
- `--user-agent <UA>`: 自定义 User-Agent
- `--timeout <SECS>`: 请求超时（默认 20 秒）
- `--concurrency <N>`: 并发数（默认 4）
- `--verbose`: 显示调试信息到 STDERR

