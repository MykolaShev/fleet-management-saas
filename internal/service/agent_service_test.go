package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

// fakeLLMClient implements llm.Client. chatResponse/chatErr drive the
// non-streaming "decide" call; streamChunks drive the follow-up streamed
// call made only when a tool was requested.
type fakeLLMClient struct {
	chatResponse *llm.ChatResponse
	chatErr      error
	streamChunks []llm.StreamChunk
	streamErr    error

	lastChatMessages   []llm.Message
	lastStreamMessages []llm.Message
}

func (f *fakeLLMClient) Chat(_ context.Context, messages []llm.Message, _ []llm.Tool) (*llm.ChatResponse, error) {
	f.lastChatMessages = messages
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	return f.chatResponse, nil
}

func (f *fakeLLMClient) StreamChat(_ context.Context, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	f.lastStreamMessages = messages
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan llm.StreamChunk, len(f.streamChunks))
	for _, c := range f.streamChunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

type fakeVehicleLister struct {
	items []domain.Vehicle
}

func (f *fakeVehicleLister) List(_ context.Context, _ uuid.UUID, _, _ int) (*service.ListResult, error) {
	return &service.ListResult{Items: f.items, TotalItems: int64(len(f.items))}, nil
}

type fakeDeliveryLister struct {
	items []domain.Delivery
}

func (f *fakeDeliveryLister) List(_ context.Context, _ uuid.UUID, _, _ int) (*service.DeliveryListResult, error) {
	return &service.DeliveryListResult{Items: f.items, TotalItems: int64(len(f.items))}, nil
}

// fakeToolProvider implements the (unexported) toolProvider interface by
// structural typing — it never needs to name that interface.
type fakeToolProvider struct {
	defs              []mcp.ToolDefinition
	callResult        string
	callErr           error
	calledName        string
	calledArgs        json.RawMessage
	definitionsCalled bool
}

func (f *fakeToolProvider) Definitions() []mcp.ToolDefinition {
	f.definitionsCalled = true
	return f.defs
}

func (f *fakeToolProvider) Call(_ context.Context, _ uuid.UUID, name string, arguments json.RawMessage) (string, error) {
	f.calledName = name
	f.calledArgs = arguments
	if f.callErr != nil {
		return "", f.callErr
	}
	return f.callResult, nil
}

func TestAgentService_Chat_IncludesFleetContextInDecideCall(t *testing.T) {
	vehicles := &fakeVehicleLister{items: []domain.Vehicle{
		{BaseModel: domain.BaseModel{ID: uuid.New()}, PlateNumber: "AA1234BC", Model: "Transit", Status: domain.VehicleIdle},
	}}
	deliveries := &fakeDeliveryLister{items: []domain.Delivery{
		{BaseModel: domain.BaseModel{ID: uuid.New()}, PickupAddress: "Warehouse", DropoffAddress: "Client St", Status: domain.DeliveryPending},
	}}
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{Content: "all good"}}

	svc := service.NewAgentService(fakeLLM, vehicles, deliveries, &fakeToolProvider{})

	_, err := svc.Chat(context.Background(), uuid.New(), "how many vehicles are idle?")
	require.NoError(t, err)

	require.Len(t, fakeLLM.lastChatMessages, 2)
	assert.Equal(t, "system", fakeLLM.lastChatMessages[0].Role)
	assert.Contains(t, fakeLLM.lastChatMessages[0].Content, "AA1234BC")
	assert.Contains(t, fakeLLM.lastChatMessages[0].Content, "Warehouse")
	assert.Equal(t, "user", fakeLLM.lastChatMessages[1].Role)
	assert.Equal(t, "how many vehicles are idle?", fakeLLM.lastChatMessages[1].Content)
}

func TestAgentService_Chat_EmptyFleet_StillBuildsValidPrompt(t *testing.T) {
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{Content: "ok"}}
	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{}, &fakeToolProvider{})

	_, err := svc.Chat(context.Background(), uuid.New(), "anything?")

	require.NoError(t, err)
	assert.Contains(t, fakeLLM.lastChatMessages[0].Content, "(none)")
}

func TestAgentService_Chat_OffersRegisteredToolsToTheModel(t *testing.T) {
	tools := &fakeToolProvider{defs: []mcp.ToolDefinition{
		{Name: "get_vehicle_location", Description: "..."},
	}}
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{Content: "ok"}}
	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{}, tools)

	_, err := svc.Chat(context.Background(), uuid.New(), "hi")

	require.NoError(t, err)
	assert.True(t, tools.definitionsCalled, "AgentService.Chat must offer the registered tools to the model's decide call")
}

func TestAgentService_Chat_NoToolCall_RelaysDecisionAsSingleChunk(t *testing.T) {
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{Content: "3 vehicles are idle."}}
	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{}, &fakeToolProvider{})

	chunks, err := svc.Chat(context.Background(), uuid.New(), "how many vehicles are idle?")
	require.NoError(t, err)

	var received []llm.StreamChunk
	for c := range chunks {
		received = append(received, c)
	}

	require.Len(t, received, 2)
	assert.Equal(t, "3 vehicles are idle.", received[0].Content)
	assert.True(t, received[1].Done)
}

func TestAgentService_Chat_ToolCallRequested_EmitsToolEventsThenStreamsFinalAnswer(t *testing.T) {
	toolCall := llm.ToolCall{
		ID:       "call_1",
		Type:     "function",
		Function: llm.ToolCallFunction{Name: "get_vehicle_location", Arguments: `{"vehicle_id":"abc"}`},
	}
	fakeLLM := &fakeLLMClient{
		chatResponse: &llm.ChatResponse{ToolCalls: []llm.ToolCall{toolCall}},
		streamChunks: []llm.StreamChunk{{Content: "It's "}, {Content: "at the depot."}, {Done: true}},
	}
	tools := &fakeToolProvider{callResult: "Vehicle AA1234BC was last seen at lat=50, lng=30."}

	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{}, tools)

	chunks, err := svc.Chat(context.Background(), uuid.New(), "where is AA1234BC?")
	require.NoError(t, err)

	var received []llm.StreamChunk
	for c := range chunks {
		received = append(received, c)
	}

	require.Len(t, received, 5) // tool_call, tool_result, 2 content chunks, done
	require.NotNil(t, received[0].ToolCall)
	assert.Equal(t, "get_vehicle_location", received[0].ToolCall.Name)
	assert.Equal(t, `{"vehicle_id":"abc"}`, received[0].ToolCall.Arguments)

	require.NotNil(t, received[1].ToolResult)
	assert.Contains(t, received[1].ToolResult.Result, "lat=50")

	assert.Equal(t, "It's ", received[2].Content)
	assert.Equal(t, "at the depot.", received[3].Content)
	assert.True(t, received[4].Done)

	assert.Equal(t, "get_vehicle_location", tools.calledName)
	assert.JSONEq(t, `{"vehicle_id":"abc"}`, string(tools.calledArgs))

	// The follow-up StreamChat call must carry the tool result back as a
	// "tool"-role message so the model can produce a grounded final
	// answer, not a fresh guess.
	last := fakeLLM.lastStreamMessages
	require.NotEmpty(t, last)
	toolMsg := last[len(last)-1]
	assert.Equal(t, "tool", toolMsg.Role)
	assert.Equal(t, "call_1", toolMsg.ToolCallID)
	assert.Contains(t, toolMsg.Content, "lat=50")
}

func TestAgentService_Chat_ToolCallFails_EmitsToolCallThenError(t *testing.T) {
	toolCall := llm.ToolCall{ID: "call_1", Function: llm.ToolCallFunction{Name: "get_vehicle_location", Arguments: "{}"}}
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{ToolCalls: []llm.ToolCall{toolCall}}}
	tools := &fakeToolProvider{callErr: errors.New("vehicle not found")}

	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{}, tools)

	chunks, err := svc.Chat(context.Background(), uuid.New(), "where is X?")
	require.NoError(t, err)

	var received []llm.StreamChunk
	for c := range chunks {
		received = append(received, c)
	}

	require.Len(t, received, 2)
	require.NotNil(t, received[0].ToolCall)
	require.Error(t, received[1].Err)
}

func TestAgentService_Chat_DecideStepFails_ReturnsError(t *testing.T) {
	fakeLLM := &fakeLLMClient{chatErr: errors.New("groq unavailable")}
	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{}, &fakeToolProvider{})

	_, err := svc.Chat(context.Background(), uuid.New(), "hi")

	assert.Error(t, err)
}