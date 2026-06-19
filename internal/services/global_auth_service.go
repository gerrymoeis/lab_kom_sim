package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type GlobalAuthService struct {
	userRepo *repository.GlobalUserRepository
}

func NewGlobalAuthService(userRepo *repository.GlobalUserRepository) *GlobalAuthService {
	return &GlobalAuthService{userRepo: userRepo}
}

func (s *GlobalAuthService) Login(username, password string) (*models.GlobalUser, string, error) {
	u, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	existingToken, _ := s.userRepo.GetSessionToken(u.ID)
	if existingToken != "" {
		return nil, "", ErrAlreadyLoggedIn
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, "", fmt.Errorf("gagal generate session token: %w", err)
	}
	token := hex.EncodeToString(b)
	s.userRepo.UpdateSessionToken(u.ID, token)

	return u, token, nil
}

func (s *GlobalAuthService) Logout(userID int) {
	s.userRepo.ClearSessionToken(userID)
}

func (s *GlobalAuthService) GetPermissions(userID int) ([]models.LabPermission, error) {
	return s.userRepo.GetPermissions(userID)
}

func (s *GlobalAuthService) GetLabsForUser(userID int, isSuperAdmin bool, allLabs []config.LabConfig) []string {
	if isSuperAdmin {
		paths := make([]string, len(allLabs))
		for i, lab := range allLabs {
			paths[i] = lab.URLPath
		}
		return paths
	}

	perms, err := s.userRepo.GetPermissions(userID)
	if err != nil {
		return nil
	}

	paths := make([]string, 0, len(perms))
	for _, p := range perms {
		paths = append(paths, p.LabURLPath)
	}
	return paths
}
