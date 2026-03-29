FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o domino-placar .

FROM alpine:latest
LABEL org.opencontainers.image.title="domino-placar" \
      org.opencontainers.image.description="Real-time digital scoreboard for Pontinho" \
      org.opencontainers.image.source="https://github.com/leandrodaf/domino-placar" \
      org.opencontainers.image.licenses="MIT"
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/domino-placar .
COPY --from=builder /app/static/ static/
COPY --from=builder /app/templates/ templates/
COPY --from=builder /app/internal/i18n/locales/ internal/i18n/locales/
EXPOSE 8080
CMD ["./domino-placar"]
