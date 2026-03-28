FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o domino-placar .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/domino-placar .
COPY --from=builder /app/static/ static/
COPY --from=builder /app/templates/ templates/
EXPOSE 8080
CMD ["./domino-placar"]
