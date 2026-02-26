# ZeroBounce Go SDK – test image (Go 1.21)
FROM golang:1.21-alpine

WORKDIR /app

COPY . .

RUN go mod download

# -short skips integration tests that call the real API
CMD ["go", "test", "./...", "-cover", "-short", "-v"]
