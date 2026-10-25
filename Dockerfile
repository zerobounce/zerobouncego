# ZeroBounce Go SDK – test image (Go 1.27)
FROM golang:1.27-alpine

WORKDIR /app

COPY . .

RUN go mod download

# -short skips integration tests that call the real API

USER 65532:65532

CMD ["go", "test", "./...", "-cover", "-short", "-v"]
