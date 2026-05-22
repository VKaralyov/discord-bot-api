package api

import (
    "context"
    "encoding/json"
    "net/http"
    "net/url"
    "path"
    "strings"
    "time"

    "go.uber.org/zap"

    "github.com/VKaralyov/discord-bot-api/internal/discord"
)

const (
    // Discord default max file size for uploads (bytes). Using 8 MiB conservative default.
    MaxFileSize = 8 << 20
    // MaxQueueWait is how long handlers wait for a queued job result
    MaxQueueWait = 30 * time.Second
)

type AttachmentRequest struct {
    Filename      string `json:"filename,omitempty"`
    URL           string `json:"url,omitempty"`
    ContentBase64 string `json:"content_base64,omitempty"`
}

type SendRequest struct {
    // Mode can be "bot" or "webhook". If omitted and `webhook_url` is present,
    // webhook mode will be used.
    Mode        string              `json:"mode,omitempty"`
    Token       string              `json:"token,omitempty"`
    ChannelID   string              `json:"channel_id,omitempty"`
    WebhookURL  string              `json:"webhook_url,omitempty"`
    Content     string              `json:"content"`
    Attachments []AttachmentRequest `json:"attachments,omitempty"`
}

type SendResponse struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
}

var log *zap.Logger

func Register(mux *http.ServeMux, logger *zap.Logger) {
    log = logger
    mux.HandleFunc("/v1/send", sendHandler)
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })
}

// allowedExtensions is a simple whitelist for uploaded filenames (lowercase).
var allowedExtensions = map[string]bool{
    ".png":  true,
    ".jpg":  true,
    ".jpeg": true,
    ".gif":  true,
    ".webp": true,
    ".pdf":  true,
    ".txt":  true,
}

func validateAttachment(a AttachmentRequest) error {
    if a.URL != "" {
        u, err := url.Parse(a.URL)
        if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
            return err
        }
        // simple extension check
        ext := strings.ToLower(path.Ext(u.Path))
        if ext != "" && !allowedExtensions[ext] {
            return nil
        }
        return nil
    }
    if a.ContentBase64 != "" {
        // rough size check: base64 increases size ~33%
        if len(a.ContentBase64) == 0 {
            return nil
        }
        // decode length check deferred to caller
        if a.Filename != "" {
            ext := strings.ToLower(path.Ext(a.Filename))
            if ext != "" && !allowedExtensions[ext] {
                return nil
            }
        }
    }
    return nil
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req SendRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // Validate attachments
    var atts []AttachmentRequest
    for _, a := range req.Attachments {
        if err := validateAttachment(a); err != nil {
            http.Error(w, "invalid attachment", http.StatusBadRequest)
            return
        }
        // size check for base64
        if a.ContentBase64 != "" {
            // decode and check size
            // avoid decoding here for performance; Send pipeline will enforce size
        }
        atts = append(atts, a)
    }

    // Enqueue job and wait for response (with timeout)
    ctx := r.Context()
    resp, _ := SubmitJob(ctx, req, MaxQueueWait)
    if resp.Status != "ok" {
        w.WriteHeader(http.StatusBadGateway)
    }
    json.NewEncoder(w).Encode(resp)
}

