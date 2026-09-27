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

type fakeUserRepo struct {
	users map[uuid.UUID]domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[uuid.UUID]domain.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	u.ID = uuid.New()
	f.users[u.ID] = *u
	return nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok || u.TenantID != tenantID {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}

func (f *fakeUserRepo) List(_ context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.User, int64, error) {
	var all []domain.User
	for _, u := range f.users {
		if u.TenantID == tenantID {
			all = append(all, u)
		}
	}
	total := int64(len(all))

	start := (page - 1) * pageSize
	if start > len(all) {
		start = len(all)
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}

	return all[start:end], total, nil
}

func (f *fakeUserRepo) Update(_ context.Context, tenantID uuid.UUID, u *domain.User) error {
	existing, ok := f.users[u.ID]
	if !ok || existing.TenantID != tenantID {
		return repository.ErrNotFound
	}
	f.users[u.ID] = *u
	return nil
}

func (f *fakeUserRepo) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	existing, ok := f.users[id]
	if !ok || existing.TenantID != tenantID {
		return repository.ErrNotFound
	}
	delete(f.users, id)
	return nil
}

func TestUserService_Create_RejectsEmptyEmail(t *testing.T) {
	svc := service.NewUserService(newFakeUserRepo())

	_, err := svc.Create(context.Background(), service.CreateUserInput{
		TenantID: uuid.New(),
		Role:     domain.RoleDriver,
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestUserService_Create_RejectsInvalidRole(t *testing.T) {
	svc := service.NewUserService(newFakeUserRepo())

	_, err := svc.Create(context.Background(), service.CreateUserInput{
		TenantID: uuid.New(),
		Email:    "a@b.test",
		Role:     "owner", // not a recognized role
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestUserService_Create_Success(t *testing.T) {
	svc := service.NewUserService(newFakeUserRepo())

	u, err := svc.Create(context.Background(), service.CreateUserInput{
		TenantID: uuid.New(),
		Email:    "driver@acme.test",
		Name:     "Dmytro",
		Role:     domain.RoleDriver,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.RoleDriver, u.Role)
	assert.NotEqual(t, uuid.Nil, u.ID)
}

func TestUserService_Update_RejectsInvalidRole(t *testing.T) {
	repo := newFakeUserRepo()
	svc := service.NewUserService(repo)
	tenantID := uuid.New()

	u, err := svc.Create(context.Background(), service.CreateUserInput{
		TenantID: tenantID,
		Email:    "a@b.test",
		Role:     domain.RoleDriver,
	})
	require.NoError(t, err)

	badRole := domain.UserRole("owner")
	_, err = svc.Update(context.Background(), tenantID, u.ID, service.UpdateUserInput{
		Role: &badRole,
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestUserService_Update_PartialFields(t *testing.T) {
	repo := newFakeUserRepo()
	svc := service.NewUserService(repo)
	tenantID := uuid.New()

	u, err := svc.Create(context.Background(), service.CreateUserInput{
		TenantID: tenantID,
		Email:    "a@b.test",
		Name:     "Old Name",
		Role:     domain.RoleDriver,
	})
	require.NoError(t, err)

	newName := "New Name"
	updated, err := svc.Update(context.Background(), tenantID, u.ID, service.UpdateUserInput{
		Name: &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, domain.RoleDriver, updated.Role) // untouched field preserved
}

func TestUserService_List_ComputesTotalPages(t *testing.T) {
	repo := newFakeUserRepo()
	svc := service.NewUserService(repo)
	tenantID := uuid.New()

	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), service.CreateUserInput{
			TenantID: tenantID,
			Email:    "user@acme.test",
			Role:     domain.RoleDriver,
		})
		require.NoError(t, err)
	}

	result, err := svc.List(context.Background(), tenantID, 1, 2)

	require.NoError(t, err)
	assert.Equal(t, int64(3), result.TotalItems)
	assert.Equal(t, 2, result.TotalPages) // 3 items / page size 2 -> 2 pages
}

func TestUserService_Delete_NotFound(t *testing.T) {
	svc := service.NewUserService(newFakeUserRepo())

	err := svc.Delete(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, repository.ErrNotFound)
}
