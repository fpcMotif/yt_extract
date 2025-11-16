mod extract;
mod innertube;

use anyhow::Result;
use clap::Parser;
use std::collections::HashSet;

#[derive(Parser, Debug)]
#[command(name = "ytextract")]
#[command(about = "从 YouTube 播放列表提取视频链接", long_about = None)]
struct Args {
    /// YouTube URL（播放列表或任意含 list= 的页面）
    url: String,

    /// 对输出去重
    #[arg(long)]
    unique: bool,

    /// 自定义 User-Agent
    #[arg(long)]
    user_agent: Option<String>,

    /// 请求超时（秒）
    #[arg(long, default_value = "20")]
    timeout: u64,

    /// 并发抓取数
    #[arg(long, default_value = "4")]
    concurrency: usize,

    /// 显示调试信息到 STDERR
    #[arg(short, long)]
    verbose: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    // 构建 HTTP 客户端
    let client = extract::build_client(
        args.timeout,
        args.user_agent.as_deref(),
    )?;

    // 输出所有视频链接
    let video_ids = extract::extract_all_videos(
        &client,
        &args.url,
        args.concurrency,
        args.verbose,
    )
    .await?;

    // 去重（如果需要）
    let output_ids: Vec<String> = if args.unique {
        let mut seen = HashSet::new();
        video_ids
            .into_iter()
            .filter(|id| seen.insert(id.clone()))
            .collect()
    } else {
        video_ids
    };

    // 逐行输出干净 URL 到 STDOUT
    for video_id in output_ids {
        println!("https://www.youtube.com/watch?v={}", video_id);
    }

    Ok(())
}

