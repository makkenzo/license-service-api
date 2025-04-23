package memstorage

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/domain/user"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"golang.org/x/crypto/bcrypt"
)

type UserRepositoryMock struct {
	mu    sync.RWMutex
	users map[string]*user.User
}

func NewUserRepositoryMock() *UserRepositoryMock {
	repo := &UserRepositoryMock{
		users: make(map[string]*user.User),
	}

	adminPassword := "adminpassword"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)

	adminUser := &user.User{
		ID:           uuid.New(),
		Username:     "admin",
		PasswordHash: string(hashedPassword),
		Role:         "admin",
	}
	repo.users[strings.ToLower(adminUser.Username)] = adminUser

	return repo
}

func (r *UserRepositoryMock) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.users[strings.ToLower(username)]
	if !ok {
		return nil, ierr.ErrUserNotFound
	}

	userCopy := *u
	return &userCopy, nil
}

func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
