package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
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

type AgentService struct {
	llm        llm.Client
	vehicles   vehicleLister
	deliveries deliveryLister
}

func NewAgentService(llmClient llm.Client, vehicles vehicleLister, deliveries deliveryLister) *AgentService {
	return &AgentService{llm: llmClient, vehicles: vehicles, deliveries: deliveries}
}

// maxContextItems caps how many vehicles/deliveries get stuffed into the
// prompt. Fine for a course-sized fleet; a real system with thousands of
// records would need an actual retrieval step instead of "dump
// everything" — flagged here rather than silently breaking at scale.
const maxContextItems = 50

const systemPromptTemplate = `You are a logistics assistant for a fleet management company. Answer the manager's question using ONLY the fleet data below. Be concise. If the data doesn't answer the question, say so honestly instead of guessing.

Current vehicles:
%s

Current deliveries:
%s`

// Chat builds a snapshot of the tenant's current fleet state and feeds it
// to the LLM as context — rather than giving the model live tools to call
// itself, which is C22's job via MCP — then streams the response back.
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

	return s.llm.StreamChat(ctx, messages)
}

func formatVehicles(vehicles []domain.Vehicle) string {
	if len(vehicles) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, v := range vehicles {
		fmt.Fprintf(&b, "- %s (%s): status=%s\n", v.PlateNumber, v.Model, v.Status)
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
