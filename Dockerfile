FROM golang:1.20-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN apk add --no-cache git
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -o /out/discord-bot-api ./cmd/server

FROM alpine:3.18 AS runtime
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /out/discord-bot-api /usr/local/bin/discord-bot-api
RUN chown app:app /usr/local/bin/discord-bot-api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/discord-bot-api"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD [ "sh", "-c", "wget -qO- http://localhost:8080/healthz || exit 1" ]
