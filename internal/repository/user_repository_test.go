package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

// setupUserTestDB mirrors setupTestDB but migrates User instead of Vehicle.
// Kept separate rather than migrating every model in one shared helper, so
// each test file's dependencies stay obvious at a glance.
func setupUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupTestDB(t) // from vehicle_repository_test.go: fresh in-memory SQLite
	require.NoError(t, db.AutoMigrate(&domain.User{}))
	return db
}

func TestUserRepository_CreateAndGet(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	u := &domain.User{TenantID: tenantID, Email: "manager@acme.test", Name: "Ada", Role: domain.RoleManager}

	require.NoError(t, repo.Create(ctx, u))
	assert.NotEqual(t, uuid.Nil, u.ID)

	got, err := repo.GetByID(ctx, tenantID, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "manager@acme.test", got.Email)
	assert.Equal(t, domain.RoleManager, got.Role)
}

func TestUserRepository_Create_DuplicateEmailInSameTenant(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	require.NoError(t, repo.Create(ctx, &domain.User{TenantID: tenantID, Email: "dup@acme.test", Role: domain.RoleDriver}))

	err := repo.Create(ctx, &domain.User{TenantID: tenantID, Email: "dup@acme.test", Role: domain.RoleDriver})
	assert.ErrorIs(t, err, repository.ErrConflict)
}

func TestUserRepository_Create_SameEmailAcrossDifferentTenants(t *testing.T) {
	// The uniqueness constraint is (tenant_id, email), not email alone, so
	// the same email must be allowed to exist in two different tenants.
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	createTestTenant(t, db, tenantA)
	createTestTenant(t, db, tenantB)

	require.NoError(t, repo.Create(ctx, &domain.User{TenantID: tenantA, Email: "shared@acme.test", Role: domain.RoleDriver}))
	err := repo.Create(ctx, &domain.User{TenantID: tenantB, Email: "shared@acme.test", Role: domain.RoleDriver})
	assert.NoError(t, err)
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestUserRepository_List_IsScopedToTenant(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	createTestTenant(t, db, tenantA)
	createTestTenant(t, db, tenantB)

	require.NoError(t, repo.Create(ctx, &domain.User{TenantID: tenantA, Email: "a1@acme.test", Role: domain.RoleDriver}))
	require.NoError(t, repo.Create(ctx, &domain.User{TenantID: tenantA, Email: "a2@acme.test", Role: domain.RoleManager}))
	require.NoError(t, repo.Create(ctx, &domain.User{TenantID: tenantB, Email: "b1@acme.test", Role: domain.RoleDriver}))

	items, total, err := repo.List(ctx, tenantA, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

func TestUserRepository_Update_PersistsChange(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	u := &domain.User{TenantID: tenantID, Email: "old@acme.test", Name: "Old Name", Role: domain.RoleDriver}
	require.NoError(t, repo.Create(ctx, u))

	u.Name = "New Name"
	u.Role = domain.RoleManager
	require.NoError(t, repo.Update(ctx, tenantID, u))

	got, err := repo.GetByID(ctx, tenantID, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", got.Name)
	assert.Equal(t, domain.RoleManager, got.Role)
}

func TestUserRepository_Update_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	err := repo.Update(ctx, uuid.New(), &domain.User{BaseModel: domain.BaseModel{ID: uuid.New()}})
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	u := &domain.User{TenantID: tenantID, Email: "del@acme.test", Role: domain.RoleDriver}
	require.NoError(t, repo.Create(ctx, u))

	require.NoError(t, repo.Delete(ctx, tenantID, u.ID))

	_, err := repo.GetByID(ctx, tenantID, u.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
