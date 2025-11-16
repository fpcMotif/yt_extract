package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"
)

// InnertubeConfig holds YouTube Innertube API configuration
type InnertubeConfig struct {
	APIKey  string
	Context map[string]any
}

// ExtractInnertubeConfig extracts Innertube configuration from HTML
func ExtractInnertubeConfig(html string) (*InnertubeConfig, error) {
	apiKey := extractAPIKey(html)
	if apiKey == "" {
		return nil, fmt.Errorf("无法找到 INNERTUBE_API_KEY")
	}

	context := extractInnertubeContext(html)
	if context == nil {
		return nil, fmt.Errorf("无法找到 INNERTUBE_CONTEXT")
	}

	return &InnertubeConfig{
		APIKey:  apiKey,
		Context: context,
	}, nil
}

// extractAPIKey extracts the API key from HTML
func extractAPIKey(html string) string {
	patterns := []string{
		`"INNERTUBE_API_KEY"\s*:\s*"([^"]+)"`,
		`"innertubeApiKey"\s*:\s*"([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractInnertubeContext extracts the Innertube context from HTML
func extractInnertubeContext(html string) map[string]any {
	// Try to extract full context from ytcfg
	re := regexp.MustCompile(`"INNERTUBE_CONTEXT"\s*:\s*(\{[^}]*?"client"\s*:\s*\{[^}]+\}[^}]*\})`)
	if matches := re.FindStringSubmatch(html); len(matches) > 1 {
		var context map[string]any
		if err := json.Unmarshal([]byte(matches[1]), &context); err == nil {
			return context
		}
	}

	// Fallback: construct minimal context
	clientName := extractClientName(html)
	if clientName == "" {
		clientName = "WEB"
	}

	clientVersion := extractClientVersion(html)
	if clientVersion == "" {
		clientVersion = "2.20231201.00.00"
	}

	return map[string]any{
		"client": map[string]any{
			"clientName":    clientName,
			"clientVersion": clientVersion,
			"hl":            "en",
			"gl":            "US",
		},
	}
}

// extractClientName extracts client name from HTML
func extractClientName(html string) string {
	re := regexp.MustCompile(`"INNERTUBE_CLIENT_NAME"\s*:\s*"([^"]+)"`)
	if matches := re.FindStringSubmatch(html); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractClientVersion extracts client version from HTML
func extractClientVersion(html string) string {
	re := regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION"\s*:\s*"([^"]+)"`)
	if matches := re.FindStringSubmatch(html); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractInitialData extracts ytInitialData from HTML
func ExtractInitialData(html string) (map[string]any, error) {
	patterns := []string{
		`var ytInitialData\s*=\s*(\{.*?\});`,
		`window\["ytInitialData"\]\s*=\s*(\{.*?\});`,
		`ytInitialData\s*=\s*(\{.*?\});`,
	}

	for _, pattern := range patterns {
		// Use a greedy approach to capture the JSON object
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			var data map[string]any
			if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("无法在页面中找到 ytInitialData")
}

// extractVideosFromData recursively extracts video IDs and continuation token
func extractVideosFromData(data any) ([]string, string) {
	videoIDs := []string{}
	continuation := ""

	// Navigate to playlist contents
	if dataMap, ok := data.(map[string]any); ok {
		if contents, ok := navigateJSON(dataMap, "contents", "twoColumnBrowseResultsRenderer", "tabs"); ok {
			extractVideosRecursive(contents, &videoIDs, &continuation)
		} else if contents, ok := dataMap["contents"]; ok {
			extractVideosRecursive(contents, &videoIDs, &continuation)
		} else {
			extractVideosRecursive(data, &videoIDs, &continuation)
		}
	}

	return videoIDs, continuation
}

// navigateJSON navigates through nested JSON structure
func navigateJSON(data map[string]any, keys ...string) (any, bool) {
	current := any(data)
	for _, key := range keys {
		if m, ok := current.(map[string]any); ok {
			if val, exists := m[key]; exists {
				current = val
			} else {
				return nil, false
			}
		} else {
			return nil, false
		}
	}
	return current, true
}

// extractVideosRecursive recursively extracts video IDs and continuation
func extractVideosRecursive(value any, videoIDs *[]string, continuation *string) {
	switch v := value.(type) {
	case map[string]any:
		// Extract videoId
		if videoID, ok := v["videoId"].(string); ok {
			*videoIDs = append(*videoIDs, videoID)
		}

		// Extract continuation token
		if *continuation == "" {
			if token, ok := v["continuation"].(string); ok {
				*continuation = token
			}
		}

		// Recurse through all values
		for _, val := range v {
			extractVideosRecursive(val, videoIDs, continuation)
		}

	case []any:
		for _, item := range v {
			extractVideosRecursive(item, videoIDs, continuation)
		}
	}
}

// FetchPlaylistVideos fetches all videos from a playlist (including pagination)
func FetchPlaylistVideos(ctx context.Context, client *http.Client, config *InnertubeConfig, initialData map[string]any, verbose bool) ([]string, error) {
	allVideos := []string{}

	// Extract from initial data
	initialVideos, continuationToken := extractVideosFromData(initialData)
	allVideos = append(allVideos, initialVideos...)

	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] 初始页面获取了 %d 个视频\n", len(allVideos))
	}

	// Paginate through remaining videos
	page := 1
	for continuationToken != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] 获取第 %d 页...\n", page+1)
		}

		videos, nextToken, err := fetchContinuation(ctx, client, config, continuationToken)
		if err != nil {
			return nil, fmt.Errorf("获取播放列表视频失败: %w", err)
		}

		if len(videos) == 0 {
			break
		}

		allVideos = append(allVideos, videos...)
		continuationToken = nextToken
		page++

		// Small delay to avoid rate limiting
		time.Sleep(200 * time.Millisecond)

		// Safety limit
		if page > 1000 {
			if verbose {
				fmt.Fprintln(os.Stderr, "[WARN] 达到最大分页限制 (1000 页)")
			}
			break
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] 分页完成，总共 %d 页\n", page)
	}

	return allVideos, nil
}

// fetchContinuation fetches the next page using continuation token
func fetchContinuation(ctx context.Context, client *http.Client, config *InnertubeConfig, continuation string) ([]string, string, error) {
	url := fmt.Sprintf("https://www.youtube.com/youtubei/v1/browse?key=%s", config.APIKey)

	payload := map[string]any{
		"context":      config.Context,
		"continuation": continuation,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("编码请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求 browse API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, "", fmt.Errorf("解析 JSON 响应失败: %w", err)
	}

	// Extract videos from response
	videoIDs := []string{}
	nextContinuation := ""

	if actions, ok := data["onResponseReceivedActions"]; ok {
		extractVideosRecursive(actions, &videoIDs, &nextContinuation)
	}

	if continuationContents, ok := data["continuationContents"]; ok {
		extractVideosRecursive(continuationContents, &videoIDs, &nextContinuation)
	}

	return videoIDs, nextContinuation, nil
}
