package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type fakeFeatureFlagRepo struct {
	flags map[string]domain.FeatureFlag // key: tenantID+"|"+flagKey
}

func newFakeFeatureFlagRepo() *fakeFeatureFlagRepo {
	return &fakeFeatureFlagRepo{flags: map[string]domain.FeatureFlag{}}
}

func ffKey(tenantID uuid.UUID, key domain.FeatureFlagKey) string {
	return tenantID.String() + "|" + string(key)
}

func (f *fakeFeatureFlagRepo) Get(_ context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey) (*domain.FeatureFlag, error) {
	flag, ok := f.flags[ffKey(tenantID, key)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &flag, nil
}

func (f *fakeFeatureFlagRepo) List(_ context.Context, tenantID uuid.UUID) ([]domain.FeatureFlag, error) {
	var result []domain.FeatureFlag
	for _, flag := range f.flags {
		if flag.TenantID == tenantID {
			result = append(result, flag)
		}
	}
	return result, nil
}

func (f *fakeFeatureFlagRepo) Set(_ context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey, enabled bool) (*domain.FeatureFlag, error) {
	flag := domain.FeatureFlag{TenantID: tenantID, Key: key, Enabled: enabled}
	f.flags[ffKey(tenantID, key)] = flag
	return &flag, nil
}

func TestFeatureFlagService_List_MergesKnownFlagsWithConfigured(t *testing.T) {
	repo := newFakeFeatureFlagRepo()
	svc := service.NewFeatureFlagService(repo)
	tenantID := uuid.New()

	_, err := svc.Set(context.Background(), tenantID, domain.FeatureAIAgent, true)
	require.NoError(t, err)

	states, err := svc.List(context.Background(), tenantID)
	require.NoError(t, err)

	// The full known catalog comes back, not just the one flag that was
	// explicitly set.
	assert.Len(t, states, len(domain.KnownFeatureFlags))

	var aiAgentEnabled, liveTrackingEnabled bool
	for _, s := range states {
		if s.Key == domain.FeatureAIAgent {
			aiAgentEnabled = s.Enabled
		}
		if s.Key == domain.FeatureLiveTracking {
			liveTrackingEnabled = s.Enabled
		}
	}
	assert.True(t, aiAgentEnabled, "explicitly configured flag should be true")
	assert.False(t, liveTrackingEnabled, "never-configured flag should default to false")
}

func TestFeatureFlagService_IsEnabled_DefaultsFalseWhenUnconfigured(t *testing.T) {
	svc := service.NewFeatureFlagService(newFakeFeatureFlagRepo())

	enabled, err := svc.IsEnabled(context.Background(), uuid.New(), domain.FeatureAIAgent)

	require.NoError(t, err) // not configured is not an error
	assert.False(t, enabled)
}

func TestFeatureFlagService_IsEnabled_ReflectsConfiguredValue(t *testing.T) {
	repo := newFakeFeatureFlagRepo()
	svc := service.NewFeatureFlagService(repo)
	tenantID := uuid.New()

	_, err := svc.Set(context.Background(), tenantID, domain.FeatureLiveTracking, true)
	require.NoError(t, err)

	enabled, err := svc.IsEnabled(context.Background(), tenantID, domain.FeatureLiveTracking)
	require.NoError(t, err)
	assert.True(t, enabled)
}

func TestFeatureFlagService_Set_RejectsUnknownKey(t *testing.T) {
	svc := service.NewFeatureFlagService(newFakeFeatureFlagRepo())

	_, err := svc.Set(context.Background(), uuid.New(), domain.FeatureFlagKey("not_a_real_flag"), true)

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestFeatureFlagService_Set_TogglesValue(t *testing.T) {
	repo := newFakeFeatureFlagRepo()
	svc := service.NewFeatureFlagService(repo)
	tenantID := uuid.New()

	state, err := svc.Set(context.Background(), tenantID, domain.FeatureTelegramNotifications, true)
	require.NoError(t, err)
	assert.True(t, state.Enabled)

	state, err = svc.Set(context.Background(), tenantID, domain.FeatureTelegramNotifications, false)
	require.NoError(t, err)
	assert.False(t, state.Enabled)
}
