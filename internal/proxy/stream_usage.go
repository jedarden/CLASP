package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jedarden/clasp/pkg/models"
)

const requestCostHeader = "X-CLASP-Cost"

// announceRequestCostTrailer declares the cost metadata before a streaming
// response is committed. The value is populated once the provider reports
// final usage, when it is too late to change ordinary response headers.
func announceRequestCostTrailer(w http.ResponseWriter) {
	w.Header().Add("Trailer", requestCostHeader)
}

func setRequestCostHeader(w http.ResponseWriter, tracker *CostTracker, model string, inputTokens, outputTokens int) {
	cost := tracker.CalculateRequestCost(model, inputTokens, outputTokens)
	w.Header().Set(requestCostHeader, fmt.Sprintf("%.6f", cost))
}

type anthropicStreamUsage struct {
	pending      string
	inputTokens  int
	outputTokens int
	hasInput     bool
	hasOutput    bool
}

func (u *anthropicStreamUsage) consume(chunk []byte) {
	u.pending += string(chunk)
	for {
		separator, separatorLength := strings.Index(u.pending, "\n\n"), 2
		if crlfSeparator := strings.Index(u.pending, "\r\n\r\n"); crlfSeparator != -1 && (separator == -1 || crlfSeparator < separator) {
			separator, separatorLength = crlfSeparator, 4
		}
		if separator == -1 {
			return
		}

		event := u.pending[:separator]
		u.pending = u.pending[separator+separatorLength:]
		u.consumeEvent(event)
	}
}

func (u *anthropicStreamUsage) consumeEvent(event string) {
	var data string
	for _, line := range strings.Split(event, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "data: ") {
			data += strings.TrimPrefix(line, "data: ")
		}
	}
	if data == "" {
		return
	}

	var streamEvent struct {
		Type    string `json:"type"`
		Message *struct {
			Usage *models.AnthropicUsage `json:"usage"`
		} `json:"message"`
		Usage *models.AnthropicUsage `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &streamEvent) != nil {
		return
	}

	switch streamEvent.Type {
	case "message_start":
		if streamEvent.Message != nil && streamEvent.Message.Usage != nil {
			u.inputTokens = streamEvent.Message.Usage.InputTokens
			u.hasInput = true
		}
	case "message_delta":
		if streamEvent.Usage != nil {
			u.outputTokens = streamEvent.Usage.OutputTokens
			u.hasOutput = true
		}
	}
}

func (u *anthropicStreamUsage) complete() bool {
	return u.hasInput && u.hasOutput
}
