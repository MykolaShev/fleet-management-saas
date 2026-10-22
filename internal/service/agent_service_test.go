package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type fakeLLMClient struct {
	lastMessages []llm.Message
	chunks       []llm.StreamChunk
}

func (f *fakeLLMClient) StreamChat(_ context.Context, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	f.lastMessages = messages
	ch := make(chan llm.StreamChunk, len(f.chunks))
	for _, c := range f.chunks {
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

func TestAgentService_Chat_IncludesFleetContextInSystemPrompt(t *testing.T) {
	vehicles := &fakeVehicleLister{items: []domain.Vehicle{
		{BaseModel: domain.BaseModel{ID: uuid.New()}, PlateNumber: "AA1234BC", Model: "Transit", Status: domain.VehicleIdle},
	}}
	deliveries := &fakeDeliveryLister{items: []domain.Delivery{
		{BaseModel: domain.BaseModel{ID: uuid.New()}, PickupAddress: "Warehouse", DropoffAddress: "Client St", Status: domain.DeliveryPending},
	}}
	fakeLLM := &fakeLLMClient{chunks: []llm.StreamChunk{{Done: true}}}

	svc := service.NewAgentService(fakeLLM, vehicles, deliveries)

	_, err := svc.Chat(context.Background(), uuid.New(), "how many vehicles are idle?")
	require.NoError(t, err)

	require.Len(t, fakeLLM.lastMessages, 2)
	assert.Equal(t, "system", fakeLLM.lastMessages[0].Role)
	assert.Contains(t, fakeLLM.lastMessages[0].Content, "AA1234BC")
	assert.Contains(t, fakeLLM.lastMessages[0].Content, "Warehouse")
	assert.Equal(t, "user", fakeLLM.lastMessages[1].Role)
	assert.Equal(t, "how many vehicles are idle?", fakeLLM.lastMessages[1].Content)
}

func TestAgentService_Chat_EmptyFleet_StillBuildsValidPrompt(t *testing.T) {
	fakeLLM := &fakeLLMClient{chunks: []llm.StreamChunk{{Done: true}}}
	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{})

	_, err := svc.Chat(context.Background(), uuid.New(), "anything?")

	require.NoError(t, err)
	assert.Contains(t, fakeLLM.lastMessages[0].Content, "(none)")
}

func TestAgentService_Chat_PassesThroughStreamChunks(t *testing.T) {
	fakeLLM := &fakeLLMClient{chunks: []llm.StreamChunk{
		{Content: "part1"},
		{Content: "part2"},
		{Done: true},
	}}
	svc := service.NewAgentService(fakeLLM, &fakeVehicleLister{}, &fakeDeliveryLister{})

	chunks, err := svc.Chat(context.Background(), uuid.New(), "hi")
	require.NoError(t, err)

	var received []llm.StreamChunk
	for c := range chunks {
		received = append(received, c)
	}

	require.Len(t, received, 3)
	assert.Equal(t, "part1", received[0].Content)
	assert.Equal(t, "part2", received[1].Content)
	assert.True(t, received[2].Done)
}
