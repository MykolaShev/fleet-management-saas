package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
)

// vehicleLister and deliveryLister are narrow read-only views AgentService
// needs to build context for the LLM — satisfied directly by
// *VehicleService and *DeliveryService via their existing List methods,
// no new repository code needed.
type vehicleLister interface {
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) (*ListResult, error)
}

type deliveryLister interface {
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) (*DeliveryListResult, error)
}

// toolProvider is the subset of *mcp.Server AgentService needs: the tool
// definitions to offer the model, and a way to invoke one. Declared here
// (not using *mcp.Server directly) so tests can supply a fake without
// spinning up a real mcp.Server — same narrow-interface pattern as
// vehicleLister/deliveryLister above.
type toolProvider interface {
	Definitions() []mcp.ToolDefinition
	Call(ctx context.Context, tenantID uuid.UUID, name string, arguments json.RawMessage) (string, error)
}

type AgentService struct {
	llm        llm.Client
	vehicles   vehicleLister
	deliveries deliveryLister
	tools      toolProvider
}

func NewAgentService(llmClient llm.Client, vehicles vehicleLister, deliveries deliveryLister, tools toolProvider) *AgentService {
	return &AgentService{llm: llmClient, vehicles: vehicles, deliveries: deliveries, tools: tools}
}

// maxContextItems caps how many vehicles/deliveries get stuffed into the
// prompt. Fine for a course-sized fleet; a real system with thousands of
// records would need an actual retrieval step instead of "dump
// everything" — flagged here rather than silently breaking at scale.
const maxContextItems = 50

const systemPromptTemplate = `You are a logistics assistant for a fleet management company. Answer the manager's question using the fleet data below and, when it genuinely helps, the tools available to you. Be concise. If neither the data nor a tool answers the question, say so honestly instead of guessing.

When a tool takes a vehicle_id or delivery_id, use the matching "id" field from the data below (a UUID) — never the plate number or any other human-readable label, even if the user's question only mentions the plate number.

Current vehicles:
%s

Current deliveries:
%s`

// Chat builds a snapshot of the tenant's current fleet state, feeds it to
// the LLM alongside the tools the fleet MCP server (internal/mcp) exposes,
// and streams the response back.
//
// This is a deliberate two-step, partially-streamed flow, not a full
// multi-round tool-calling loop: (1) a non-streaming "decide" call with
// tools attached — if no tool is requested, the complete answer is
// relayed as a single SSE chunk (singleChunkStream) rather than wasting a
// second API call; (2) if a tool *is* requested, call it via the MCP
// server (tenant-scoped), emit ToolCall/ToolResult events so a caller can
// see this happen even from the raw SSE stream, then make a second,
// genuinely streamed call (without tools) for the final natural-language
// answer. One round only — justified because each of this project's three
// tools is a single atomic lookup/action, not a task needing iterative
// tool use.
func (s *AgentService) Chat(ctx context.Context, tenantID uuid.UUID, userMessage string) (<-chan llm.StreamChunk, error) {
	vehicles, err := s.vehicles.List(ctx, tenantID, 1, maxContextItems)
	if err != nil {
		return nil, fmt.Errorf("load vehicle context: %w", err)
	}
	deliveries, err := s.deliveries.List(ctx, tenantID, 1, maxContextItems)
	if err != nil {
		return nil, fmt.Errorf("load delivery context: %w", err)
	}

	systemPrompt := fmt.Sprintf(systemPromptTemplate, formatVehicles(vehicles.Items), formatDeliveries(deliveries.Items))

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	decision, err := s.llm.Chat(ctx, messages, toLLMTools(s.tools.Definitions()))
	if err != nil {
		return nil, fmt.Errorf("agent decide step: %w", err)
	}

	if len(decision.ToolCalls) == 0 {
		return singleChunkStream(decision.Content), nil
	}

	// Only the first requested call is acted on — see the doc comment
	// above for why a single round is enough here.
	call := decision.ToolCalls[0]

	out := make(chan llm.StreamChunk, 2)
	go func() {
		defer close(out)

		out <- llm.StreamChunk{ToolCall: &llm.ToolCallEvent{Name: call.Function.Name, Arguments: call.Function.Arguments}}

		result, err := s.tools.Call(ctx, tenantID, call.Function.Name, json.RawMessage(call.Function.Arguments))
		if err != nil {
			out <- llm.StreamChunk{Err: fmt.Errorf("tool %s failed: %w", call.Function.Name, err)}
			return
		}

		out <- llm.StreamChunk{ToolResult: &llm.ToolResultEvent{Name: call.Function.Name, Result: result}}

		followUp := make([]llm.Message, 0, len(messages)+2)
		followUp = append(followUp, messages...)
		followUp = append(followUp,
			llm.Message{Role: "assistant", ToolCalls: decision.ToolCalls},
			llm.Message{Role: "tool", ToolCallID: call.ID, Content: result},
		)

		stream, err := s.llm.StreamChat(ctx, followUp)
		if err != nil {
			out <- llm.StreamChunk{Err: fmt.Errorf("agent final answer: %w", err)}
			return
		}
		for chunk := range stream {
			out <- chunk
		}
	}()

	return out, nil
}

// singleChunkStream wraps a complete answer (no tool call needed) in a
// closed channel with exactly one content chunk plus Done — so the
// handler's SSE loop, which only knows how to drain a channel, doesn't
// need a separate code path for "the decide call already had the full
// answer".
func singleChunkStream(content string) <-chan llm.StreamChunk {
	ch := make(chan llm.StreamChunk, 2)
	ch <- llm.StreamChunk{Content: content}
	ch <- llm.StreamChunk{Done: true}
	close(ch)
	return ch
}

// toLLMTools converts the MCP server's tool definitions into the
// OpenAI-compatible shape Chat needs. The two types are deliberately kept
// separate (mcp.ToolDefinition vs llm.Tool) so neither package depends on
// the other's wire format.
func toLLMTools(defs []mcp.ToolDefinition) []llm.Tool {
	tools := make([]llm.Tool, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  d.InputSchema,
			},
		})
	}
	return tools
}

func formatVehicles(vehicles []domain.Vehicle) string {
	if len(vehicles) == 0 {
		return "(none)"
	}
	// id is listed explicitly and first (not just plate_number) because
	// get_vehicle_location's vehicle_id argument is this UUID, not the
	// plate number — without it in context the model has no way to know
	// the UUID for "AA1234BC" and ends up passing the plate number
	// itself as vehicle_id, which fails UUID parsing in the tool.
	var b strings.Builder
	for _, v := range vehicles {
		fmt.Fprintf(&b, "- id=%s, plate_number=%s, model=%s, status=%s\n", v.ID, v.PlateNumber, v.Model, v.Status)
	}
	return b.String()
}

func formatDeliveries(deliveries []domain.Delivery) string {
	if len(deliveries) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, d := range deliveries {
		fmt.Fprintf(&b, "- delivery %s: %s -> %s, status=%s\n", d.ID, d.PickupAddress, d.DropoffAddress, d.Status)
	}
	return b.String()
}