.PHONY: build clean test install help

# 默认目标
all: build

# 构建
build:
	@echo "构建 ytextract..."
	@go build -o bin/ytextract ./cmd/ytextract
	@echo "✅ 构建完成: bin/ytextract"

# 构建优化版本
build-optimized:
	@echo "构建优化版本..."
	@go build -ldflags="-s -w" -o bin/ytextract ./cmd/ytextract
	@echo "✅ 优化构建完成: bin/ytextract"

# 清理
clean:
	@echo "清理构建产物..."
	@rm -rf bin/
	@rm -f /tmp/go_output.txt /tmp/rust_output.txt
	@echo "✅ 清理完成"

# 测试
test:
	@echo "运行测试..."
	@go test -v ./...

# 运行对比测试
compare:
	@echo "运行 Go vs Rust 对比测试..."
	@./test_comparison.sh

# 安装到系统
install: build
	@echo "安装到 /usr/local/bin..."
	@sudo cp bin/ytextract /usr/local/bin/
	@echo "✅ 安装完成"

# 格式化代码
fmt:
	@echo "格式化代码..."
	@go fmt ./...
	@echo "✅ 格式化完成"

# 代码检查
lint:
	@echo "运行 golint..."
	@go vet ./...
	@echo "✅ 检查完成"

# 更新依赖
deps:
	@echo "更新依赖..."
	@go mod tidy
	@go mod download
	@echo "✅ 依赖更新完成"

# 显示帮助
help:
	@echo "ytextract Go 实现 - Makefile 帮助"
	@echo ""
	@echo "可用目标："
	@echo "  make build           - 构建可执行文件"
	@echo "  make build-optimized - 构建优化版本（更小体积）"
	@echo "  make clean           - 清理构建产物"
	@echo "  make test            - 运行测试"
	@echo "  make compare         - 对比 Go 和 Rust 版本"
	@echo "  make install         - 安装到系统（需要 sudo）"
	@echo "  make fmt             - 格式化代码"
	@echo "  make lint            - 代码检查"
	@echo "  make deps            - 更新依赖"
	@echo "  make help            - 显示此帮助信息"
	@echo ""
	@echo "示例："
	@echo "  make build"
	@echo "  ./bin/ytextract --help"
	@echo "  TEST_URL='...' make compare"

