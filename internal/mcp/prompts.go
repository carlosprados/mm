package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Prompts are pre-built templates the AI client can pick. They hydrate the
// prompt with live data (channel messages) so the model receives ready-to-use
// context instead of an empty placeholder.

func (s *Server) registerPrompts() {
	s.srv.AddPrompt(&mcpsdk.Prompt{
		Name:        "summarize_channel",
		Description: "Summarize the most recent activity of a channel: decisions, blockers, pending actions.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "channel", Description: "Channel slug, e.g. dev-backend", Required: true},
			{Name: "limit", Description: "How many recent messages to consider (default 50)"},
		},
	}, s.summarizeChannelPrompt)

	s.srv.AddPrompt(&mcpsdk.Prompt{
		Name:        "draft_reply",
		Description: "Draft a reply to a channel based on its recent context and a stated intent.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "channel", Description: "Channel slug to reply in", Required: true},
			{Name: "intent", Description: "What the reply should accomplish (tone, content, ask)", Required: true},
			{Name: "limit", Description: "How many recent messages to consider as context (default 30)"},
		},
	}, s.draftReplyPrompt)

	s.srv.AddPrompt(&mcpsdk.Prompt{
		Name:        "daily_digest",
		Description: "Produce a digest combining recent activity from several channels.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "channels", Description: "Comma-separated list of channel slugs", Required: true},
			{Name: "limit", Description: "Messages per channel (default 30)"},
		},
	}, s.dailyDigestPrompt)
}

func (s *Server) summarizeChannelPrompt(ctx context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	channel, _ := req.Params.Arguments["channel"]
	if channel == "" {
		return nil, fmt.Errorf("argument 'channel' is required")
	}
	limit := argInt(req.Params.Arguments, "limit", 50)

	msgs, err := s.fetchMessages(ctx, channel, limit)
	if err != nil {
		return nil, err
	}

	body := strings.Builder{}
	fmt.Fprintf(&body, "You are reviewing the last %d messages of #%s. Produce a concise summary highlighting:\n\n", len(msgs), channel)
	body.WriteString("- Decisions made\n- Open questions and blockers\n- Action items with their owner if mentioned\n\n")
	body.WriteString("Messages (oldest → newest):\n")
	for _, m := range msgs {
		fmt.Fprintf(&body, "[%s] %s: %s\n", m.Time, m.From, m.Text)
	}

	return promptResult("Summarize the recent activity of a channel.", body.String()), nil
}

func (s *Server) draftReplyPrompt(ctx context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	channel, _ := req.Params.Arguments["channel"]
	intent, _ := req.Params.Arguments["intent"]
	if channel == "" || intent == "" {
		return nil, fmt.Errorf("arguments 'channel' and 'intent' are required")
	}
	limit := argInt(req.Params.Arguments, "limit", 30)

	msgs, err := s.fetchMessages(ctx, channel, limit)
	if err != nil {
		return nil, err
	}

	body := strings.Builder{}
	fmt.Fprintf(&body, "You are @%s drafting a reply on #%s.\n\n", s.mm.Username, channel)
	fmt.Fprintf(&body, "Intent: %s\n\n", intent)
	body.WriteString("Recent context (oldest → newest):\n")
	for _, m := range msgs {
		fmt.Fprintf(&body, "[%s] %s: %s\n", m.Time, m.From, m.Text)
	}
	body.WriteString("\nWrite a single reply. Match the channel's tone and language. No salutations beyond what the channel uses.")

	return promptResult("Draft a reply to a channel given recent context and an intent.", body.String()), nil
}

func (s *Server) dailyDigestPrompt(ctx context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	raw, _ := req.Params.Arguments["channels"]
	if raw == "" {
		return nil, fmt.Errorf("argument 'channels' is required")
	}
	limit := argInt(req.Params.Arguments, "limit", 30)

	channels := []string{}
	for _, c := range strings.Split(raw, ",") {
		if c = strings.TrimSpace(c); c != "" {
			channels = append(channels, c)
		}
	}

	body := strings.Builder{}
	body.WriteString("Produce a brief digest of activity across the channels below. Group by channel. ")
	body.WriteString("For each one: 2-4 bullet points covering the most relevant news, decisions or blockers. ")
	body.WriteString("Skip channels with no meaningful activity.\n\n")

	for _, name := range channels {
		fmt.Fprintf(&body, "## #%s\n", name)
		msgs, err := s.fetchMessages(ctx, name, limit)
		if err != nil {
			fmt.Fprintf(&body, "(could not read channel: %v)\n\n", err)
			continue
		}
		for _, m := range msgs {
			fmt.Fprintf(&body, "[%s] %s: %s\n", m.Time, m.From, m.Text)
		}
		body.WriteString("\n")
	}

	return promptResult("Daily digest across multiple channels.", body.String()), nil
}

func argInt(args map[string]string, name string, def int) int {
	v, ok := args[name]
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func promptResult(description, text string) *mcpsdk.GetPromptResult {
	return &mcpsdk.GetPromptResult{
		Description: description,
		Messages: []*mcpsdk.PromptMessage{{
			Role:    "user",
			Content: &mcpsdk.TextContent{Text: text},
		}},
	}
}
