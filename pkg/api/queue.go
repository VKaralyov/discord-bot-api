package api

import (
    "context"
    "time"

    "go.uber.org/zap"
    "golang.org/x/time/rate"

    "github.com/VKaralyov/discord-bot-api/internal/discord"
)

// Job represents a request to send a message and a channel to receive the response.
type Job struct {
    Ctx  context.Context
    Req  SendRequest
    Resp chan SendResponse
}

var (
    jobQueue chan Job
    limiter  *rate.Limiter
    logger   *zap.Logger
)

// Init initializes the package-level queue and starts worker goroutines.
func Init(queueSize, workers int, lim *rate.Limiter, l *zap.Logger) {
    jobQueue = make(chan Job, queueSize)
    limiter = lim
    logger = l

    for i := 0; i < workers; i++ {
        go worker(i)
    }
}

func SubmitJob(ctx context.Context, req SendRequest, timeout time.Duration) (SendResponse, error) {
    if jobQueue == nil {
        return SendResponse{Status: "error", Message: "server not initialized"}, nil
    }

    respCh := make(chan SendResponse, 1)
    job := Job{Ctx: ctx, Req: req, Resp: respCh}

    select {
    case jobQueue <- job:
        // enqueued
    case <-time.After(timeout):
        return SendResponse{Status: "error", Message: "server busy"}, nil
    }

    select {
    case res := <-respCh:
        return res, nil
    case <-time.After(timeout):
        return SendResponse{Status: "error", Message: "timeout waiting for result"}, nil
    }
}

func worker(id int) {
    for j := range jobQueue {
        ctx := j.Ctx

        // Respect global limiter if present
        if limiter != nil {
            if err := limiter.Wait(ctx); err != nil {
                j.Resp <- SendResponse{Status: "error", Message: "rate limit wait canceled"}
                continue
            }
        }

        // Create a context with timeout for the external call
        callCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
        defer cancel()

        // Map attachments
        var atts []discord.Attachment
        for _, a := range j.Req.Attachments {
            atts = append(atts, discord.Attachment{Filename: a.Filename, URL: a.URL, ContentBase64: a.ContentBase64})
        }

        var res SendResponse
        if j.Req.Mode == "webhook" || (j.Req.Mode == "" && j.Req.WebhookURL != "") {
            err := discord.SendWebhook(callCtx, j.Req.WebhookURL, j.Req.Content, atts)
            if err != nil {
                logger.Warn("webhook send failed", zap.Error(err))
                res = SendResponse{Status: "error", Message: err.Error()}
            } else {
                res = SendResponse{Status: "ok"}
            }
        } else {
            err := discord.SendMessageBot(callCtx, j.Req.Token, j.Req.ChannelID, j.Req.Content, atts)
            if err != nil {
                logger.Warn("bot send failed", zap.Error(err))
                res = SendResponse{Status: "error", Message: err.Error()}
            } else {
                res = SendResponse{Status: "ok"}
            }
        }

        // Non-blocking send to response channel
        select {
        case j.Resp <- res:
        default:
        }
    }
}
