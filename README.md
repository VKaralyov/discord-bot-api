# discord-bot-api

Lightweight Go HTTP API to send messages through the Discord API on behalf of bots.

Quickstart

1. Build locally:

```bash
make build
```

2. Run server (development):

```bash
make run
```

API

POST /v1/send

Request body (JSON):

```json
{
  "mode": "bot",
  "token": "BOT_TOKEN",
  "channel_id": "123456789012345678",
  "content": "Hello from API"
}
```

Example curl (use environment variable for token):

```bash
curl -X POST http://localhost:8080/v1/send \
  -H "Content-Type: application/json" \
  -d '{"mode":"bot","token":"'$DISCORD_TOKEN'","channel_id":"123456789012345678","content":"Hello from API"}'
```

Webhook example:

```bash
curl -X POST http://localhost:8080/v1/send \
  -H "Content-Type: application/json" \
  -d '{"mode":"webhook","webhook_url":"https://discord.com/api/webhooks/ID/TOKEN","content":"Hello via webhook"}'
```

Attachment examples:

- Using a remote image URL (embed):

```bash
curl -X POST http://localhost:8080/v1/send \
  -H "Content-Type: application/json" \
  -d '{"mode":"bot","token":"'$DISCORD_TOKEN'","channel_id":"123","content":"Here is an image","attachments":[{"url":"https://example.com/image.jpg"}]}'
```

- Uploading a file (base64-encoded):

```bash
BASE64=$(base64 -w0 ./myfile.png)
curl -X POST http://localhost:8080/v1/send \
  -H "Content-Type: application/json" \
  -d '{"mode":"bot","token":"'$DISCORD_TOKEN'","channel_id":"123","content":"File attached","attachments":[{"filename":"myfile.png","content_base64":"'$BASE64'"}]}'
```

OpenAPI

The API surface is described in `api/openapi.yaml`.

Docker

Build and run locally:

```bash
make docker-build
make docker-run
```

- This project is licensed under Apache-2.0 (see `LICENSE`).
- Do NOT commit real bot tokens—use environment variables or a secrets manager.
- This is an integration helper: you must follow Discord Developer Terms of Service.

Rate limits and queueing

- The server applies an internal rate limiter (configurable via `--rate`) and a bounded queue (configurable via `--queue`) to protect downstream Discord APIs.
- If the queue is full the server returns a busy error; tune `--workers` and `--queue` for your workload.

Attachment format and size limits

- Attachments may be provided as a remote `url` (will be added as an embed) or as `content_base64` with a `filename` to upload.
- To limit payload size, uploads use a conservative default maximum file size of 8 MiB. Adjust this in code if needed.
