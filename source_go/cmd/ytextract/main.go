package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"ytextract/internal/extract"
)

func main() {
	// Define flags
	var (
		unique      bool
		userAgent   string
		timeout     int
		concurrency int
		verbose     bool
	)

	flag.BoolVar(&unique, "unique", false, "对输出去重")
	flag.StringVar(&userAgent, "user-agent", "", "自定义 User-Agent")
	flag.IntVar(&timeout, "timeout", 20, "请求超时（秒）")
	flag.IntVar(&concurrency, "concurrency", 4, "并发抓取数")
	flag.BoolVar(&verbose, "verbose", false, "显示调试信息到 STDERR")
	flag.BoolVar(&verbose, "v", false, "显示调试信息到 STDERR（简写）")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] URL\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "从 YouTube 播放列表提取视频链接\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Get URL argument
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "错误: 缺少 URL 参数\n\n")
		flag.Usage()
		os.Exit(1)
	}

	url := flag.Arg(0)

	// Build HTTP client
	client, err := extract.BuildClient(timeout, userAgent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法构建 HTTP 客户端: %v\n", err)
		os.Exit(1)
	}

	// Extract all videos
	ctx := context.Background()
	videoIDs, err := extract.ExtractAllVideos(ctx, client, url, concurrency, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	// Deduplicate if requested
	var outputIDs []string
	if unique {
		seen := make(map[string]struct{})
		for _, id := range videoIDs {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				outputIDs = append(outputIDs, id)
			}
		}
	} else {
		outputIDs = videoIDs
	}

	// Print URLs to STDOUT
	for _, videoID := range outputIDs {
		fmt.Printf("https://www.youtube.com/watch?v=%s\n", videoID)
	}
}
