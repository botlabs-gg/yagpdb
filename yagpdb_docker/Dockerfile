FROM golang:stretch as builder

# Uncomment during development
COPY . /appbuild/yagpdb

WORKDIR /appbuild/yagpdb/cmd/yagpdb

RUN CGO_ENABLED=0 GOOS=linux go build -v

FROM alpine:latest

WORKDIR /app
VOLUME /app/soundboard \
	/app/cert
EXPOSE 80 443

# We need the X.509 certificates for client TLS to work.
RUN apk --no-cache add ca-certificates

# Add tzdata for time#LoadLocation
RUN apk add --no-cache tzdata

# Add ffmpeg for soundboard support
RUN apk --no-cache add ffmpeg

# Handle templates for plugins automatically
COPY --from=builder /appbuild/yagpdb/*/assets/*.html templates/plugins/

COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/templates templates/
COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/posts posts/
COPY --from=builder /appbuild/yagpdb/cmd/yagpdb/static static/

COPY --from=builder /appbuild/yagpdb/cmd/yagpdb .

# `exec` allows us to receive shutdown signals.
# CMD exec /app/yagpdb -all -pa $extra_flags
CMD ["exec", "/app/yagpdb", "-all"]