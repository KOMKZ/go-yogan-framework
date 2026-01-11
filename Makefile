.PHONY: test lint fmt tidy clean

# 运行所有测试
test:
	go test -v -cover ./...

# 运行测试并生成覆盖率报告
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# 代码格式化
fmt:
	go fmt ./...

# 静态检查
vet:
	go vet ./...

# 代码检查（需要安装 golangci-lint）
lint:
	golangci-lint run

# 整理依赖
tidy:
	go mod tidy

# 清理临时文件
clean:
	rm -f coverage.out coverage.html
	rm -rf */logs
	find . -name "*.out" -delete
