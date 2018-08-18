FROM golang:stretch as builder

WORKDIR $GOPATH/src

RUN git clone -b dgofork https://github.com/jonas747/dshardmanager github.com/jonas747/dshardmanager \
  && git clone -b dgofork https://github.com/jonas747/dcmd github.com/jonas747/dcmd \
  && git clone -b v2.7.4 https://github.com/volatiletech/sqlboiler github.com/volatiletech/sqlboiler

RUN go get -d -v \
  github.com/jonas747/yagpdb/cmd/yagpdb

# Uncomment during development
# COPY . github.com/jonas747/yagpdb

# Disable CGO_ENABLED to force a totally static compile.
RUN CGO_ENABLED=0 GOOS=linux go install -v \
  github.com/jonas747/yagpdb/cmd/yagpdb


FROM alpine:latest

WORKDIR /app
VOLUME /app/soundboard \
  /app/cert
EXPOSE 80 443

# We need the X.509 certificates for client TLS to work.
RUN apk --no-cache add ca-certificates

# Add ffmpeg for soundboard support
RUN apk --no-cache add ffmpeg

# Handle templates for plugins automatically
COPY --from=builder /go/src/github.com/jonas747/yagpdb/*/assets/*.html templates/plugins/

COPY --from=builder /go/src/github.com/jonas747/yagpdb/cmd/yagpdb/templates templates/
COPY --from=builder /go/src/github.com/jonas747/yagpdb/cmd/yagpdb/posts posts/
COPY --from=builder /go/src/github.com/jonas747/yagpdb/cmd/yagpdb/static static/

COPY --from=builder /go/bin/yagpdb .

# Set to "-https=false" if using a TLS-enabled proxy such as
# jrcs/letsencrypt-nginx-proxy-companion
ENV external_tls ""

# `exec` allows us to receive shutdown signals.
CMD exec /app/yagpdb -bot -web -pa $external_tls

