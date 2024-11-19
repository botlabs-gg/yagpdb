FROM docker.io/golang:1.23.3 AS builder

WORKDIR /appbuild/yagpdb
COPY go.mod go.sum ./
RUN go mod download

COPY . .

WORKDIR /appbuild/yagpdb/cmd/yagpdb
RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags "-X github.com/botlabs-gg/yagpdb/v2/common.VERSION=$(git describe --tags)"



FROM docker.io/alpine:latest

WORKDIR /app
VOLUME ["/app/soundboard", "/app/cert"]
EXPOSE 80 443

# Dependencies: ca-certificates for client TLS, tzdata for timezone and ffmpeg for soundboard support
RUN apk --no-cache add ca-certificates ffmpeg tzdata

COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/yagpdb yagpdb

ENTRYPOINT ["/app/yagpdb"]
CMD ["-all", "-pa", "-exthttps=false", "-https=true"]
