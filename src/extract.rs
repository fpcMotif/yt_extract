use anyhow::{Context, Result, anyhow};
use indexmap::IndexSet;
use regex::Regex;
use reqwest::Client;
use std::time::Duration;
use url::Url;
use futures::stream::{self, StreamExt};

use crate::innertube::{extract_innertube_config, extract_initial_data, fetch_playlist_videos};

const DEFAULT_USER_AGENT: &str = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";

/// 构建 HTTP 客户端
pub fn build_client(timeout_secs: u64, user_agent: Option<&str>) -> Result<Client> {
    let ua = user_agent.unwrap_or(DEFAULT_USER_AGENT);
    
    Client::builder()
        .timeout(Duration::from_secs(timeout_secs))
        .user_agent(ua)
        .gzip(true)
        .brotli(true)
        .cookie_store(true)
        .build()
        .context("无法构建 HTTP 客户端")
}

/// 从 URL 中提取 list 参数
fn extract_list_from_url(url_str: &str) -> Option<String> {
    Url::parse(url_str)
        .ok()?
        .query_pairs()
        .find(|(key, _)| key == "list")
        .map(|(_, value)| value.to_string())
}

/// 从 HTML 中查找所有播放列表 ID
fn find_playlist_ids_in_html(html: &str) -> Vec<String> {
    let mut ids = IndexSet::new();
    
    // 从 URL 参数中查找 list=...
    let re_url = Regex::new(r#"[?&]list=([\w-]+)"#).unwrap();
    for cap in re_url.captures_iter(html) {
        if let Some(id) = cap.get(1) {
            let list_id = id.as_str().to_string();
            // 过滤掉一些特殊列表
            if !list_id.starts_with("UU") && list_id.len() > 10 {
                ids.insert(list_id);
            }
        }
    }
    
    // 从 JSON 中查找 playlistId 字段
    let re_playlist_id = Regex::new(r#""playlistId"\s*:\s*"([\w-]+)"#).unwrap();
    for cap in re_playlist_id.captures_iter(html) {
        if let Some(id) = cap.get(1) {
            let list_id = id.as_str().to_string();
            if !list_id.starts_with("UU") && list_id.len() > 10 {
                ids.insert(list_id);
            }
        }
    }
    
    ids.into_iter().collect()
}

/// 从页面发现所有播放列表
async fn discover_playlists(
    client: &Client,
    url: &str,
    verbose: bool,
) -> Result<Vec<String>> {
    if verbose {
        eprintln!("[DEBUG] 抓取页面: {}", url);
    }
    
    let response = client
        .get(url)
        .send()
        .await
        .context("请求页面失败")?;
    
    let html = response.text().await.context("读取页面内容失败")?;
    
    // 从 URL 中提取 list 参数
    let mut playlist_ids = IndexSet::new();
    if let Some(list_id) = extract_list_from_url(url) {
        if verbose {
            eprintln!("[DEBUG] 从 URL 发现播放列表: {}", list_id);
        }
        playlist_ids.insert(list_id);
    }
    
    // 从 HTML 中查找更多播放列表
    let found_ids = find_playlist_ids_in_html(&html);
    if verbose {
        eprintln!("[DEBUG] 从 HTML 发现 {} 个播放列表", found_ids.len());
    }
    
    for id in found_ids {
        playlist_ids.insert(id);
    }
    
    if playlist_ids.is_empty() {
        return Err(anyhow!("未在页面中发现任何播放列表"));
    }
    
    Ok(playlist_ids.into_iter().collect())
}

/// 抓取单个播放列表的所有视频 ID
async fn extract_playlist(
    client: &Client,
    playlist_id: &str,
    verbose: bool,
) -> Result<Vec<String>> {
    if verbose {
        eprintln!("[DEBUG] 处理播放列表: {}", playlist_id);
    }
    
    let playlist_url = format!("https://www.youtube.com/playlist?list={}", playlist_id);
    
    // 重试机制
    let mut retries = 0;
    let max_retries = 3;
    
    loop {
        match try_extract_playlist(client, &playlist_url, verbose).await {
            Ok(videos) => {
                if verbose {
                    eprintln!("[DEBUG] 从播放列表 {} 提取了 {} 个视频", playlist_id, videos.len());
                }
                return Ok(videos);
            }
            Err(e) => {
                retries += 1;
                if retries >= max_retries {
                    eprintln!("[ERROR] 播放列表 {} 提取失败: {}", playlist_id, e);
                    return Ok(Vec::new()); // 失败时返回空列表而不是中断
                }
                if verbose {
                    eprintln!("[WARN] 播放列表 {} 提取失败，重试 {}/{}...", playlist_id, retries, max_retries);
                }
                tokio::time::sleep(Duration::from_secs(2u64.pow(retries as u32))).await;
            }
        }
    }
}

/// 尝试提取播放列表（内部函数）
async fn try_extract_playlist(
    client: &Client,
    playlist_url: &str,
    verbose: bool,
) -> Result<Vec<String>> {
    // 获取播放列表页面
    let response = client
        .get(playlist_url)
        .send()
        .await
        .context("请求播放列表页面失败")?;
    
    let html = response.text().await.context("读取页面内容失败")?;
    
    // 提取 innertube 配置
    let config = extract_innertube_config(&html)
        .context("无法提取 Innertube 配置")?;
    
    // 提取初始数据
    let initial_data = extract_initial_data(&html)
        .context("无法提取 ytInitialData")?;
    
    // 获取所有视频（包括分页）
    let video_ids = fetch_playlist_videos(
        client,
        &config,
        &initial_data,
        verbose,
    )
    .await
    .context("获取播放列表视频失败")?;
    
    Ok(video_ids)
}

/// 提取所有视频 ID（主入口）
pub async fn extract_all_videos(
    client: &Client,
    url: &str,
    concurrency: usize,
    verbose: bool,
) -> Result<Vec<String>> {
    // 发现所有播放列表
    let playlist_ids = discover_playlists(client, url, verbose).await?;
    
    if verbose {
        eprintln!("[INFO] 共发现 {} 个播放列表", playlist_ids.len());
    }
    
    // 并发抓取所有播放列表
    let results: Vec<Vec<String>> = stream::iter(playlist_ids)
        .map(|playlist_id| {
            let client = client.clone();
            async move {
                extract_playlist(&client, &playlist_id, verbose).await
                    .unwrap_or_else(|_| Vec::new())
            }
        })
        .buffer_unordered(concurrency)
        .collect()
        .await;
    
    // 合并所有视频 ID（保持顺序，按播放列表内去重）
    let mut all_video_ids = Vec::new();
    for videos in results {
        let mut seen = IndexSet::new();
        for video_id in videos {
            if seen.insert(video_id.clone()) {
                all_video_ids.push(video_id);
            }
        }
    }
    
    if verbose {
        eprintln!("[INFO] 总共提取了 {} 个视频", all_video_ids.len());
    }
    
    Ok(all_video_ids)
}

