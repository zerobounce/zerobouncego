# ZeroBounce Go SDK – test image (Go 1.27)
# The base is pinned by digest (the multi-arch index of golang:1.27-alpine); Dependabot moves it.
FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280

WORKDIR /app

COPY . .

RUN go mod download

# The tests run as a non-root user whose HOME is /, so Go's default build cache
# (~/.cache/go-build) is not writable. Keep the build cache in /tmp; the module cache
# (/go/pkg/mod, filled above) is only read at test time.
ENV GOCACHE=/tmp/go-build

# -short skips integration tests that call the real API

USER 65532:65532

CMD ["go", "test", "./...", "-cover", "-short", "-v"]
