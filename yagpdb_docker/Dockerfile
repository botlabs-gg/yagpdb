FROM golang:stretch as builder

WORKDIR /appbuild/yagpdb/

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

WORKDIR /appbuild/yagpdb/cmd/yagpdb

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X github.com/jonas747/yagpdb/common.VERSION=$(git describe --tags)" -v

FROM alpine:latest

WORKDIR /app
VOLUME /app/soundboard \
  /app/cert
EXPOSE 80 443

# We need the X.509 certificates for client TLS to work.
# Also install tzdata, needed for Go timezone support, 
# and ffmpeg for soundboard support,
# as the alpine image does not come with them by default. 
RUN apk --no-cache add ca-certificates \
  ffmpeg \
  tzdata

# Handle templates for plugins automatically
COPY --from=builder /appbuild/yagpdb/*/assets/*.html templates/plugins/

COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/templates templates/
COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/posts posts/
COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/static static/

COPY --from=builder /appbuild/yagpdb/cmd/yagpdb .

# add extra flags here when running YAGPDB
# Set "-exthttps=true" if using a TLS-enabled proxy such as
# jrcs/letsencrypt-nginx-proxy-companion
# Set "-https=false" do disable https
ENV extra_flags ""

# `exec` allows us to receive shutdown signals.
CMD exec /app/yagpdb -all -pa $extra_flags
