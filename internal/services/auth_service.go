package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrAlreadyLoggedIn    = errors.New("akun sudah login di tempat lain")
)

type AuthService struct {
	userRepo         *repository.UserRepository
	activityLogService *ActivityLogService
}

func NewAuthService(userRepo *repository.UserRepository, activityLogService *ActivityLogService) *AuthService {
	return &AuthService{userRepo: userRepo, activityLogService: activityLogService}
}

func (s *AuthService) Login(username, password, ipAddress, userAgent string) (userID int, fullName, role, token string, err error) {
	u, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return 0, "", "", "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		s.activityLogService.LogAuth(0, username, "", "login", false, ipAddress, userAgent, "Invalid password")
		return 0, "", "", "", ErrInvalidCredentials
	}

	existingToken, _ := s.userRepo.GetSessionToken(u.ID)
	if existingToken != "" {
		s.userRepo.ClearSessionToken(u.ID)
		s.activityLogService.LogAuth(u.ID, username, u.Role, "login_force", true, ipAddress, userAgent, "Previous session terminated for re-login")
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return 0, "", "", "", fmt.Errorf("failed to generate session token: %w", err)
	}
	token = hex.EncodeToString(b)
	s.userRepo.UpdateSessionToken(u.ID, token)

	s.activityLogService.LogAuth(u.ID, username, u.Role, "login", true, ipAddress, userAgent, "")
	return u.ID, u.FullName, u.Role, token, nil
}

func (s *AuthService) Logout(userID int, username, role, ipAddress, userAgent string) {
	s.userRepo.ClearSessionToken(userID)
	if username != "" {
		s.activityLogService.LogAuth(userID, username, role, "logout", true, ipAddress, userAgent, "")
	}
}
