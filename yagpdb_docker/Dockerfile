FROM golang:stretch as builder

WORKDIR /appbuild/yagpdb
COPY go.mod go.sum .
RUN go mod download
COPY . .

WORKDIR /bin/yagpdb
COPY cmd/yagpdb/posts posts/
COPY cmd/yagpdb/static static/
COPY cmd/yagpdb/templates templates/
# Handle templates for plugins automatically
COPY */assets/*.html templates/plugins/

WORKDIR /appbuild/yagpdb/cmd/yagpdb
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X github.com/jonas747/yagpdb/common.VERSION=$(git describe --tags)" -v -o /bin/yagpdb



FROM alpine:latest

WORKDIR /app
VOLUME \
  /app/soundboard \
  /app/cert
EXPOSE 80 443

# We need the X.509 certificates for client TLS to work.
# Also install tzdata, needed for Go timezone support,
# and ffmpeg for soundboard support,
# as the alpine image does not come with them by default.
RUN apk --no-cache add ca-certificates ffmpeg tzdata

COPY --from=builder /bin/yagpdb .

ENTRYPOINT ["/app/yagpdb"]
CMD ["-all", "-pa", "-exthttps=false", "-https=true"]
