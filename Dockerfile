FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache dependencies and download modules.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build the binary.
COPY . ./
RUN go build -o regs ./main.go

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/regs ./regs

RUN mkdir -p data

EXPOSE 8080

ENV REGS_DATABASE_DSN=data/regs.db
CMD ["./regs"]
