package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/bestk/kiro2cc/internal/anthropic"
	"github.com/bestk/kiro2cc/internal/codewhisperer"
	"github.com/bestk/kiro2cc/internal/config"
)

var ModelMap = map[string]string{
	"claude-sonnet-4-20250514":  "CLAUDE_SONNET_4_20250514_V1_0",
	"claude-3-5-haiku-20241022": "CLAUDE_3_7_SONNET_20250219_V1_0",
}

// generateUUID generates a simple UUID v4.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// getMessageContent extracts text content from a message.
func getMessageContent(content any) string {
	switch v := content.(type) {
	case string:
		if len(v) == 0 {
			return "answer for user qeustion"
		}
		return v
	case []interface{}:
		var texts []string
		for _, block := range v {
			if m, ok := block.(map[string]interface{}); ok {
				var cb anthropic.ContentBlock
				if data, err := json.Marshal(m); err == nil {
					if err := json.Unmarshal(data, &cb); err == nil {
						switch cb.Type {
						case "tool_result":
							texts = append(texts, *cb.Content)
						case "text":
							texts = append(texts, *cb.Text)
						}
					}
				}
			}
		}
		if len(texts) == 0 {
			s, err := json.Marshal(content)
			if err != nil {
				return "answer for user qeustion"
			}
			log.Printf("uncatch: %s", string(s))
			return "answer for user qeustion"
		}
		return strings.Join(texts, "\n")
	default:
		s, err := json.Marshal(content)
		if err != nil {
			return "answer for user qeustion"
		}
		log.Printf("uncatch: %s", string(s))
		return "answer for user qeustion"
	}
}

// BuildCodeWhispererRequest builds a CodeWhisperer request from an Anthropic request.
func BuildCodeWhispererRequest(anthropicReq anthropic.Request) codewhisperer.Request {
	cfg, err := config.LoadConfig()
	if err != nil {
		// Fallback to default region if config fails to load
		cfg = &config.Config{Region: "us-east-1"}
		log.Printf("Failed to load config, falling back to default region: %s", cfg.Region)
	}

	cwReq := codewhisperer.Request{
		ProfileArn: fmt.Sprintf("arn:aws:codewhisperer:%s:699475941385:profile/EHGA3GRVQMUK", cfg.Region),
	}
	cwReq.ConversationState.ChatTriggerType = "MANUAL"
	cwReq.ConversationState.ConversationId = generateUUID()
	cwReq.ConversationState.CurrentMessage.UserInputMessage.Content = getMessageContent(anthropicReq.Messages[len(anthropicReq.Messages)-1].Content)
	cwReq.ConversationState.CurrentMessage.UserInputMessage.ModelId = ModelMap[anthropicReq.Model]
	cwReq.ConversationState.CurrentMessage.UserInputMessage.Origin = "AI_EDITOR"

	// Build history messages
	if len(anthropicReq.System) > 0 || len(anthropicReq.Messages) > 1 {
		var history []any

		assistantDefaultMsg := codewhisperer.HistoryAssistantMessage{}
		assistantDefaultMsg.AssistantResponseMessage.Content = getMessageContent("I will follow these instructions")
		assistantDefaultMsg.AssistantResponseMessage.ToolUses = make([]any, 0)

		if len(anthropicReq.System) > 0 {
			for _, sysMsg := range anthropicReq.System {
				userMsg := codewhisperer.HistoryUserMessage{}
				userMsg.UserInputMessage.Content = sysMsg.Text
				userMsg.UserInputMessage.ModelId = ModelMap[anthropicReq.Model]
				userMsg.UserInputMessage.Origin = "AI_EDITOR"
				history = append(history, userMsg)
				history = append(history, assistantDefaultMsg)
			}
		}

		for i := 0; i < len(anthropicReq.Messages)-1; i++ {
			if anthropicReq.Messages[i].Role == "user" {
				userMsg := codewhisperer.HistoryUserMessage{}
				userMsg.UserInputMessage.Content = getMessageContent(anthropicReq.Messages[i].Content)
				userMsg.UserInputMessage.ModelId = ModelMap[anthropicReq.Model]
				userMsg.UserInputMessage.Origin = "AI_EDITOR"
				history = append(history, userMsg)

				if i+1 < len(anthropicReq.Messages)-1 && anthropicReq.Messages[i+1].Role == "assistant" {
					assistantMsg := codewhisperer.HistoryAssistantMessage{}
					assistantMsg.AssistantResponseMessage.Content = getMessageContent(anthropicReq.Messages[i+1].Content)
					assistantMsg.AssistantResponseMessage.ToolUses = make([]any, 0)
					history = append(history, assistantMsg)
					i++ // Skip the assistant message that has been processed
				}
			}
		}
		cwReq.ConversationState.History = history
	}

	return cwReq
}
