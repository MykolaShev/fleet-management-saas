package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

type featureFlagRepository interface {
	Get(ctx context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey) (*domain.FeatureFlag, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]domain.FeatureFlag, error)
	Set(ctx context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey, enabled bool) (*domain.FeatureFlag, error)
}

type FeatureFlagService struct {
	repo featureFlagRepository
}

func NewFeatureFlagService(repo featureFlagRepository) *FeatureFlagService {
	return &FeatureFlagService{repo: repo}
}

// FlagState is what callers see: one entry per known flag, merged with
// whatever this tenant has explicitly configured. The catalog is always
// complete — a flag the tenant never touched still shows up, as disabled.
type FlagState struct {
	Key     domain.FeatureFlagKey `json:"key"`
	Enabled bool                  `json:"enabled"`
}

func (s *FeatureFlagService) List(ctx context.Context, tenantID uuid.UUID) ([]FlagState, error) {
	configured, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	enabledByKey := make(map[domain.FeatureFlagKey]bool, len(configured))
	for _, f := range configured {
		enabledByKey[f.Key] = f.Enabled
	}

	result := make([]FlagState, 0, len(domain.KnownFeatureFlags))
	for _, key := range domain.KnownFeatureFlags {
		result = append(result, FlagState{Key: key, Enabled: enabledByKey[key]})
	}
	return result, nil
}

// IsEnabled is the check other parts of the system (AI agent, live
// tracking, Telegram notifications — once those stages exist) will call
// before gating access to a feature. There's deliberately no error for
// "not configured" — that's an expected, normal state, not a failure, so
// it just resolves to false.
func (s *FeatureFlagService) IsEnabled(ctx context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey) (bool, error) {
	f, err := s.repo.Get(ctx, tenantID, key)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return f.Enabled, nil
}

func (s *FeatureFlagService) Set(ctx context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey, enabled bool) (*FlagState, error) {
	if !domain.IsKnownFeatureFlag(key) {
		return nil, fmt.Errorf("%w: unknown feature flag %q", ErrValidation, key)
	}

	f, err := s.repo.Set(ctx, tenantID, key, enabled)
	if err != nil {
		return nil, err
	}

	return &FlagState{Key: f.Key, Enabled: f.Enabled}, nil
}
