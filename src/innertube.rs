use anyhow::{Context, Result, anyhow};
use regex::Regex;
use reqwest::Client;
use serde_json::{json, Value};

/// Innertube 配置
#[derive(Debug, Clone)]
pub struct InnertubeConfig {
    pub api_key: String,
    pub context: Value,
}

/// 从 HTML 中提取 ytcfg 配置
pub fn extract_innertube_config(html: &str) -> Result<InnertubeConfig> {
    // 提取 API Key
    let api_key = extract_api_key(html)
        .context("无法找到 INNERTUBE_API_KEY")?;
    
    // 提取 Context
    let context = extract_innertube_context(html)
        .context("无法找到 INNERTUBE_CONTEXT")?;
    
    Ok(InnertubeConfig { api_key, context })
}

/// 提取 API Key
fn extract_api_key(html: &str) -> Option<String> {
    let patterns = [
        r#""INNERTUBE_API_KEY"\s*:\s*"([^"]+)"#,
        r#""innertubeApiKey"\s*:\s*"([^"]+)"#,
    ];
    
    for pattern in &patterns {
        let re = Regex::new(pattern).ok()?;
        if let Some(cap) = re.captures(html) {
            if let Some(key) = cap.get(1) {
                return Some(key.as_str().to_string());
            }
        }
    }
    
    None
}

/// 提取 Innertube Context
fn extract_innertube_context(html: &str) -> Option<Value> {
    // 尝试从 ytcfg 中提取完整的 context
    let re = Regex::new(r#""INNERTUBE_CONTEXT"\s*:\s*(\{[^}]*?"client"\s*:\s*\{[^}]+\}[^}]*\})"#).ok()?;
    if let Some(cap) = re.captures(html) {
        if let Some(ctx) = cap.get(1) {
            if let Ok(context) = serde_json::from_str::<Value>(ctx.as_str()) {
                return Some(context);
            }
        }
    }
    
    // 备用方案：构造最小 context
    let client_name = extract_client_name(html).unwrap_or_else(|| "WEB".to_string());
    let client_version = extract_client_version(html).unwrap_or_else(|| "2.20231201.00.00".to_string());
    
    Some(json!({
        "client": {
            "clientName": client_name,
            "clientVersion": client_version,
            "hl": "en",
            "gl": "US",
        }
    }))
}

/// 提取客户端名称
fn extract_client_name(html: &str) -> Option<String> {
    let re = Regex::new(r#""INNERTUBE_CLIENT_NAME"\s*:\s*"([^"]+)"#).ok()?;
    re.captures(html)?
        .get(1)
        .map(|m| m.as_str().to_string())
}

/// 提取客户端版本
fn extract_client_version(html: &str) -> Option<String> {
    let re = Regex::new(r#""INNERTUBE_CLIENT_VERSION"\s*:\s*"([^"]+)"#).ok()?;
    re.captures(html)?
        .get(1)
        .map(|m| m.as_str().to_string())
}

/// 从 HTML 中提取 ytInitialData
pub fn extract_initial_data(html: &str) -> Result<Value> {
    // 尝试多种模式
    let patterns = [
        r#"var ytInitialData\s*=\s*(\{.*?\});"#,
        r#"window\["ytInitialData"\]\s*=\s*(\{.*?\});"#,
        r#"ytInitialData\s*=\s*(\{.*?\});"#,
    ];
    
    for pattern in &patterns {
        let re = Regex::new(pattern).ok();
        if let Some(re) = re {
            if let Some(cap) = re.captures(html) {
                if let Some(data) = cap.get(1) {
                    match serde_json::from_str::<Value>(data.as_str()) {
                        Ok(json) => return Ok(json),
                        Err(_) => continue,
                    }
                }
            }
        }
    }
    
    Err(anyhow!("无法在页面中找到 ytInitialData"))
}

/// 从 ytInitialData 中提取视频 ID 和 continuation token
fn extract_videos_from_data(data: &Value) -> (Vec<String>, Option<String>) {
    let mut video_ids = Vec::new();
    let mut continuation = None;
    
    // 导航到播放列表内容
    if let Some(contents) = data
        .pointer("/contents/twoColumnBrowseResultsRenderer/tabs")
        .or_else(|| data.pointer("/contents"))
    {
        extract_videos_recursive(contents, &mut video_ids, &mut continuation);
    }
    
    (video_ids, continuation)
}

/// 递归提取视频 ID
fn extract_videos_recursive(value: &Value, video_ids: &mut Vec<String>, continuation: &mut Option<String>) {
    match value {
        Value::Object(map) => {
            // 提取 videoId
            if let Some(Value::String(video_id)) = map.get("videoId") {
                video_ids.push(video_id.clone());
            }
            
            // 提取 continuation token
            if continuation.is_none() {
                if let Some(Value::String(token)) = map.get("continuation") {
                    *continuation = Some(token.clone());
                }
            }
            
            // 递归处理所有子对象
            for (_, v) in map {
                extract_videos_recursive(v, video_ids, continuation);
            }
        }
        Value::Array(arr) => {
            for item in arr {
                extract_videos_recursive(item, video_ids, continuation);
            }
        }
        _ => {}
    }
}

/// 获取播放列表的所有视频（包括分页）
pub async fn fetch_playlist_videos(
    client: &Client,
    config: &InnertubeConfig,
    initial_data: &Value,
    verbose: bool,
) -> Result<Vec<String>> {
    let mut all_videos = Vec::new();
    
    // 从初始数据中提取
    let (initial_videos, mut continuation_token) = extract_videos_from_data(initial_data);
    all_videos.extend(initial_videos);
    
    if verbose {
        eprintln!("[DEBUG] 初始页面获取了 {} 个视频", all_videos.len());
    }
    
    // 分页获取剩余视频
    let mut page = 1;
    while let Some(token) = continuation_token {
        if verbose {
            eprintln!("[DEBUG] 获取第 {} 页...", page + 1);
        }
        
        let (videos, next_token) = fetch_continuation(client, config, &token).await?;
        
        if videos.is_empty() {
            break;
        }
        
        all_videos.extend(videos);
        continuation_token = next_token;
        page += 1;
        
        // 添加小延迟避免请求过快
        tokio::time::sleep(tokio::time::Duration::from_millis(200)).await;
        
        // 安全上限
        if page > 1000 {
            eprintln!("[WARN] 达到最大分页限制 (1000 页)");
            break;
        }
    }
    
    if verbose {
        eprintln!("[DEBUG] 分页完成，总共 {} 页", page);
    }
    
    Ok(all_videos)
}

/// 使用 continuation token 获取下一页
async fn fetch_continuation(
    client: &Client,
    config: &InnertubeConfig,
    continuation: &str,
) -> Result<(Vec<String>, Option<String>)> {
    let url = format!(
        "https://www.youtube.com/youtubei/v1/browse?key={}",
        config.api_key
    );
    
    let payload = json!({
        "context": config.context,
        "continuation": continuation,
    });
    
    let response = client
        .post(&url)
        .json(&payload)
        .send()
        .await
        .context("请求 browse API 失败")?;
    
    let data: Value = response.json().await.context("解析 JSON 响应失败")?;
    
    // 从响应中提取视频
    let mut video_ids = Vec::new();
    let mut next_continuation = None;
    
    if let Some(actions) = data.get("onResponseReceivedActions") {
        extract_videos_recursive(actions, &mut video_ids, &mut next_continuation);
    }
    
    // 也尝试从 continuationContents 中提取
    if let Some(continuation_contents) = data.get("continuationContents") {
        extract_videos_recursive(continuation_contents, &mut video_ids, &mut next_continuation);
    }
    
    Ok((video_ids, next_continuation))
}

