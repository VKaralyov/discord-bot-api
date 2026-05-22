package discord

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Attachment describes an attachment that can be sent either by URL
// (hosted) or by providing base64 content for upload.
type Attachment struct {
    Filename      string `json:"filename,omitempty"`
    URL           string `json:"url,omitempty"`
    ContentBase64 string `json:"content_base64,omitempty"`
}

// SendMessageBot sends a message to a Discord channel using a bot token via discordgo.
// Supports attachments: if attachment has URL set, it will be added as an embed image; if it has
// ContentBase64 set, it will be uploaded as a file.
func SendMessageBot(token, channelID, content string, attachments []Attachment) error {
    if token == "" || channelID == "" {
        return fmt.Errorf("token and channel_id are required")
    }

    dg, err := discordgo.New("Bot " + token)
    if err != nil {
        return fmt.Errorf("discordgo new: %w", err)
    }

    m := &discordgo.MessageSend{Content: content}

    // Add embeds for URL attachments
    for _, a := range attachments {
        if a.URL != "" {
            emb := &discordgo.MessageEmbed{Image: &discordgo.MessageEmbedImage{URL: a.URL}}
            m.Embeds = append(m.Embeds, emb)
        }
    }
package discord

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "path"
    "strings"
    "time"

    "github.com/bwmarrin/discordgo"
)

// Attachment describes an attachment that can be sent either by URL
// (hosted) or by providing base64 content for upload.
type Attachment struct {
    Filename      string `json:"filename,omitempty"`
    URL           string `json:"url,omitempty"`
    ContentBase64 string `json:"content_base64,omitempty"`
}

var allowedExt = map[string]bool{
    ".png":  true,
    ".jpg":  true,
    ".jpeg": true,
    ".gif":  true,
    ".webp": true,
    ".pdf":  true,
    ".txt":  true,
}

const (
    maxFileSize = 8 << 20 // 8 MiB conservative default
    maxAttempts = 4
    baseDelay   = 500 * time.Millisecond
)

func isTransientStatus(code int) bool {
    if code == http.StatusTooManyRequests {
        return true
    }
    if code >= 500 && code < 600 {
        return true
    }
    return false
}

// SendMessageBot sends a message using discordgo with retries and timeout via context.
func SendMessageBot(ctx context.Context, token, channelID, content string, attachments []Attachment) error {
    if token == "" || channelID == "" {
        return fmt.Errorf("token and channel_id are required")
    }

    // Validate attachments: size & extension
    var files []*discordgo.File
    var embeds []*discordgo.MessageEmbed
    for _, a := range attachments {
        if a.URL != "" {
            embeds = append(embeds, &discordgo.MessageEmbed{Image: &discordgo.MessageEmbedImage{URL: a.URL}})
            continue
        }
        if a.ContentBase64 != "" {
            data, err := base64.StdEncoding.DecodeString(a.ContentBase64)
            if err != nil {
                return fmt.Errorf("decode attachment: %w", err)
            }
            if len(data) > maxFileSize {
                return fmt.Errorf("attachment %s exceeds max size %d", a.Filename, maxFileSize)
            }
            if a.Filename != "" {
                ext := strings.ToLower(path.Ext(a.Filename))
                if ext != "" && !allowedExt[ext] {
                    return fmt.Errorf("attachment %s has disallowed extension", a.Filename)
                }
            }
            fname := a.Filename
            if fname == "" {
                fname = "file"
            }
            files = append(files, &discordgo.File{Name: fname, Reader: bytes.NewReader(data)})
        }
    }

    // Prepare message
    m := &discordgo.MessageSend{Content: content, Embeds: embeds, Files: files}

    // Attempt with retries and exponential backoff
    var lastErr error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        dg, err := discordgo.New("Bot " + token)
        if err != nil {
            return fmt.Errorf("discordgo new: %w", err)
        }
        // set a reasonable client timeout derived from context deadline
        dg.Client = &http.Client{Timeout: 20 * time.Second}

        _, err = dg.ChannelMessageSendComplex(channelID, m)
        _ = dg.Close()
        if err == nil {
            return nil
        }

        lastErr = err
        // If error is a discordgo.RESTError, inspect status code
        var restErr *discordgo.RESTError
        if errors.As(err, &restErr) {
            if !isTransientStatus(restErr.Response.StatusCode) {
                return err
            }
        }

        // Backoff with jitter
        sleep := time.Duration(attempt) * baseDelay
        select {
        case <-time.After(sleep):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return fmt.Errorf("send message failed after retries: %w", lastErr)
}

// SendWebhook sends to a webhook URL. Supports embeds (URL) and multipart uploads for base64 attachments.
func SendWebhook(ctx context.Context, webhookURL, content string, attachments []Attachment) error {
    if webhookURL == "" {
        return fmt.Errorf("webhook_url is required")
    }

    // Validate attachments and check if we need multipart
    hasUpload := false
    var embeds []map[string]interface{}
    for _, a := range attachments {
        if a.URL != "" {
            embeds = append(embeds, map[string]interface{}{"image": map[string]string{"url": a.URL}})
            continue
        }
        if a.ContentBase64 != "" {
            data, err := base64.StdEncoding.DecodeString(a.ContentBase64)
            if err != nil {
                return fmt.Errorf("decode attachment: %w", err)
            }
            if len(data) > maxFileSize {
                return fmt.Errorf("attachment %s exceeds max size %d", a.Filename, maxFileSize)
            }
            if a.Filename != "" {
                ext := strings.ToLower(path.Ext(a.Filename))
                if ext != "" && !allowedExt[ext] {
                    return fmt.Errorf("attachment %s has disallowed extension", a.Filename)
                }
            }
            hasUpload = true
        }
    }

    var lastErr error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if !hasUpload {
            payload := map[string]interface{}{"content": content}
            if len(embeds) > 0 {
                payload["embeds"] = embeds
            }
            b, _ := json.Marshal(payload)
            req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(b))
            if err != nil {
                return err
            }
            req.Header.Set("Content-Type", "application/json")
            client := &http.Client{Timeout: 15 * time.Second}
            resp, err := client.Do(req)
            if err == nil {
                resp.Body.Close()
                if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                    return nil
                }
                if !isTransientStatus(resp.StatusCode) {
                    return fmt.Errorf("webhook returned status %d", resp.StatusCode)
                }
                lastErr = fmt.Errorf("webhook status %d", resp.StatusCode)
            } else {
                lastErr = err
            }
        } else {
            // multipart upload
            var b bytes.Buffer
            mw := multipart.NewWriter(&b)
            pj := map[string]interface{}{"content": content}
            if len(embeds) > 0 {
                pj["embeds"] = embeds
            }

            fileIndex := 0
            for _, a := range attachments {
                if a.ContentBase64 == "" {
                    continue
                }
                data, _ := base64.StdEncoding.DecodeString(a.ContentBase64)
                fname := a.Filename
                if fname == "" {
                    fname = fmt.Sprintf("file%d", fileIndex)
                }
                fw, err := mw.CreateFormFile(fmt.Sprintf("files[%d]", fileIndex), fname)
                if err != nil {
                    return err
                }
                if _, err := io.Copy(fw, bytes.NewReader(data)); err != nil {
                    return err
                }
                fileIndex++
            }

            pjb, _ := json.Marshal(pj)
            _ = mw.WriteField("payload_json", string(pjb))
            _ = mw.Close()

            req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, &b)
            if err != nil {
                return err
            }
            req.Header.Set("Content-Type", mw.FormDataContentType())
            client := &http.Client{Timeout: 30 * time.Second}
            resp, err := client.Do(req)
            if err == nil {
                resp.Body.Close()
                if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                    return nil
                }
                if !isTransientStatus(resp.StatusCode) {
                    return fmt.Errorf("webhook returned status %d", resp.StatusCode)
                }
                lastErr = fmt.Errorf("webhook status %d", resp.StatusCode)
            } else {
                lastErr = err
            }
        }

        // backoff
        sleep := time.Duration(attempt) * baseDelay
        select {
        case <-time.After(sleep):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return fmt.Errorf("webhook send failed after retries: %w", lastErr)
}
