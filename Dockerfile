
# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" \
    -o /mcqueen ./cmd/bot

# Runtime stage
FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /mcqueen /app/mcqueen
COPY prompts/ /app/prompts/

ENTRYPOINT ["/app/mcqueen"]
