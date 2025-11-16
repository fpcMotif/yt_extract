#!/bin/bash

# ytextract 测试示例脚本

echo "==================================="
echo "ytextract 功能测试"
echo "==================================="
echo ""

# 检查是否已构建
if [ ! -f "target/release/ytextract" ]; then
    echo "未找到可执行文件，正在构建..."
    cargo build --release
    if [ $? -ne 0 ]; then
        echo "构建失败！"
        exit 1
    fi
    echo ""
fi

echo "1. 测试帮助信息"
echo "-----------------------------------"
./target/release/ytextract --help
echo ""

echo "==================================="
echo "2. 基本用法示例"
echo "-----------------------------------"
echo "要测试实际的播放列表提取，请运行："
echo ""
echo "  ./target/release/ytextract \"YOUR_PLAYLIST_URL\""
echo ""
echo "例如："
echo "  ./target/release/ytextract \"https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf\""
echo ""
echo "带调试信息："
echo "  ./target/release/ytextract -v \"YOUR_PLAYLIST_URL\""
echo ""
echo "保存到文件："
echo "  ./target/release/ytextract \"YOUR_PLAYLIST_URL\" > urls.txt"
echo ""
echo "==================================="
echo "测试完成！"
echo "==================================="

