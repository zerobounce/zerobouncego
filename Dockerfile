# ZeroBounce Go SDK – test image (Go 1.27)
# The base is pinned by digest (the multi-arch index of golang:1.27-alpine); Dependabot moves it.
FROM golang:1.27-alpine@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523

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
