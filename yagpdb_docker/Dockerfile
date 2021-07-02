FROM golang:stretch as builder

WORKDIR /appbuild/yagpdb
COPY go.mod go.sum ./
RUN go mod download

# Handle templates for plugins automatically
COPY */assets/*.html cmd/yagpdb/templates/plugins/
COPY . .

WORKDIR /appbuild/yagpdb/cmd/yagpdb
RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags "-X github.com/jonas747/yagpdb/common.VERSION=$(git describe --tags)"



FROM alpine:latest

WORKDIR /app
VOLUME ["/app/soundboard", "/app/cert"]
EXPOSE 80 443

# Dependencies: ca-certificates for client TLS, tzdata for timezone and ffmpeg for soundboard support
RUN apk --no-cache add ca-certificates ffmpeg tzdata

COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/static static/
COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/templates templates/
COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/yagpdb yagpdb

ENTRYPOINT ["/app/yagpdb"]
CMD ["-all", "-pa", "-exthttps=false", "-https=true"]
