package extract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"ytextract/internal/innertube"

	"golang.org/x/sync/errgroup"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// BuildClient constructs an HTTP client with the specified configuration
func BuildClient(timeoutSeconds int, userAgent string) (*http.Client, error) {
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false, // Enable gzip/deflate
	}

	client := &http.Client{
		Transport: &userAgentTransport{
			Transport: transport,
			UserAgent: userAgent,
		},
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	return client, nil
}

// userAgentTransport wraps http.RoundTripper to set User-Agent
type userAgentTransport struct {
	Transport http.RoundTripper
	UserAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.UserAgent)
	return t.Transport.RoundTrip(req)
}

// extractListFromURL extracts the list parameter from a URL
func extractListFromURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	return u.Query().Get("list")
}

// findPlaylistIDsInHTML finds all playlist IDs in HTML
func findPlaylistIDsInHTML(html string) []string {
	seen := make(map[string]struct{})
	var ids []string

	// Find list=... in URL parameters
	reURL := regexp.MustCompile(`[?&]list=([\w-]+)`)
	for _, match := range reURL.FindAllStringSubmatch(html, -1) {
		if len(match) > 1 {
			listID := match[1]
			// Filter out special lists
			if !startsWithUU(listID) && len(listID) > 10 {
				if _, exists := seen[listID]; !exists {
					seen[listID] = struct{}{}
					ids = append(ids, listID)
				}
			}
		}
	}

	// Find "playlistId": "..." in JSON
	rePlaylistID := regexp.MustCompile(`"playlistId"\s*:\s*"([\w-]+)"`)
	for _, match := range rePlaylistID.FindAllStringSubmatch(html, -1) {
		if len(match) > 1 {
			listID := match[1]
			if !startsWithUU(listID) && len(listID) > 10 {
				if _, exists := seen[listID]; !exists {
					seen[listID] = struct{}{}
					ids = append(ids, listID)
				}
			}
		}
	}

	return ids
}

// startsWithUU checks if a string starts with "UU"
func startsWithUU(s string) bool {
	return len(s) >= 2 && s[0] == 'U' && s[1] == 'U'
}

// discoverPlaylists discovers all playlists from a page
func discoverPlaylists(ctx context.Context, client *http.Client, urlStr string, verbose bool) ([]string, error) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] 抓取页面: %s\n", urlStr)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求页面失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取页面内容失败: %w", err)
	}
	html := string(body)

	// Extract list parameter from URL
	seen := make(map[string]struct{})
	var playlistIDs []string

	if listID := extractListFromURL(urlStr); listID != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] 从 URL 发现播放列表: %s\n", listID)
		}
		seen[listID] = struct{}{}
		playlistIDs = append(playlistIDs, listID)
	}

	// Find more playlists in HTML
	foundIDs := findPlaylistIDsInHTML(html)
	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] 从 HTML 发现 %d 个播放列表\n", len(foundIDs))
	}

	for _, id := range foundIDs {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			playlistIDs = append(playlistIDs, id)
		}
	}

	if len(playlistIDs) == 0 {
		return nil, fmt.Errorf("未在页面中发现任何播放列表")
	}

	return playlistIDs, nil
}

// extractPlaylist extracts all video IDs from a single playlist
func extractPlaylist(ctx context.Context, client *http.Client, playlistID string, verbose bool) ([]string, error) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] 处理播放列表: %s\n", playlistID)
	}

	playlistURL := fmt.Sprintf("https://www.youtube.com/playlist?list=%s", playlistID)

	// Retry logic
	maxRetries := 3
	for retries := 0; retries < maxRetries; retries++ {
		videos, err := tryExtractPlaylist(ctx, client, playlistURL, verbose)
		if err == nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[DEBUG] 从播放列表 %s 提取了 %d 个视频\n", playlistID, len(videos))
			}
			return videos, nil
		}

		if retries < maxRetries-1 {
			if verbose {
				fmt.Fprintf(os.Stderr, "[WARN] 播放列表 %s 提取失败，重试 %d/%d...\n", playlistID, retries+1, maxRetries)
			}
			time.Sleep(time.Second * time.Duration(1<<uint(retries)))
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] 播放列表 %s 提取失败: %v\n", playlistID, err)
			return []string{}, nil // Return empty list instead of failing
		}
	}

	return []string{}, nil
}

// tryExtractPlaylist attempts to extract playlist (internal function)
func tryExtractPlaylist(ctx context.Context, client *http.Client, playlistURL string, verbose bool) ([]string, error) {
	// Get playlist page
	req, err := http.NewRequestWithContext(ctx, "GET", playlistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求播放列表页面失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取页面内容失败: %w", err)
	}
	html := string(body)

	// Extract innertube config
	config, err := innertube.ExtractInnertubeConfig(html)
	if err != nil {
		return nil, fmt.Errorf("无法提取 Innertube 配置: %w", err)
	}

	// Extract initial data
	initialData, err := innertube.ExtractInitialData(html)
	if err != nil {
		return nil, fmt.Errorf("无法提取 ytInitialData: %w", err)
	}

	// Get all videos (including pagination)
	videoIDs, err := innertube.FetchPlaylistVideos(ctx, client, config, initialData, verbose)
	if err != nil {
		return nil, fmt.Errorf("获取播放列表视频失败: %w", err)
	}

	return videoIDs, nil
}

// ExtractAllVideos extracts all video IDs from the given URL (main entry point)
func ExtractAllVideos(ctx context.Context, client *http.Client, urlStr string, concurrency int, verbose bool) ([]string, error) {
	// Discover all playlists
	playlistIDs, err := discoverPlaylists(ctx, client, urlStr, verbose)
	if err != nil {
		return nil, err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[INFO] 共发现 %d 个播放列表\n", len(playlistIDs))
	}

	// Concurrently fetch all playlists
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	results := make([][]string, len(playlistIDs))

	for i, playlistID := range playlistIDs {
		i, playlistID := i, playlistID // Capture loop variables
		g.Go(func() error {
			videos, err := extractPlaylist(ctx, client, playlistID, verbose)
			if err != nil {
				// Don't fail the entire operation
				results[i] = []string{}
				return nil
			}
			results[i] = videos
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Merge all video IDs (dedupe within each playlist, preserve order)
	var allVideoIDs []string
	for _, videos := range results {
		seen := make(map[string]struct{})
		for _, videoID := range videos {
			if _, exists := seen[videoID]; !exists {
				seen[videoID] = struct{}{}
				allVideoIDs = append(allVideoIDs, videoID)
			}
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[INFO] 总共提取了 %d 个视频\n", len(allVideoIDs))
	}

	return allVideoIDs, nil
}
