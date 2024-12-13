# 使用官方 Golang 镜像作为基础镜像
FROM golang:1.22-alpine

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod tidy

# 复制源代码到容器中
COPY . .

# 编译 Go 应用
RUN GOOS=linux GOARCH=amd64 go build -o main .

# 暴露应用的端口
EXPOSE 8082

# 设置容器启动时执行的命令
CMD ["./main"]
