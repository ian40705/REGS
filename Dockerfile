FROM golang:1.26-alpine AS builder

WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
# 快取相依套件
COPY go.mod go.sum ./
RUN go mod download

# 複製原始碼並編譯
COPY . ./
# 💡 修正 1：加上 CGO_ENABLED=0，確保編譯出的靜態二進位檔在乾淨的 Alpine 能完美執行
RUN CGO_ENABLED=0 GOOS=linux go build -o regs ./main.go

FROM alpine:latest
WORKDIR /app

# 💡 修正 2：在最終階段安裝 docker-cli！這步是 DinD 能通的絕對關鍵
RUN apk update && apk add --no-cache docker-cli tzdata

# 從編譯階段複製執行檔
COPY --from=builder /app/regs ./regs

# 💡 修正 3：將 114-framework 測資目錄複製進去，確保 API 容器內路徑完整
COPY --from=builder /app/114-framework ./114-framework

RUN mkdir -p data

EXPOSE 8080

ENV REGS_DATABASE_DSN=data/regs.db
CMD ["./regs"]