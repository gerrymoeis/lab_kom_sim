package services

import (
	"errors"

	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrAlreadyLoggedIn    = errors.New("akun sudah login di tempat lain")
)

type AuthService struct {
	userRepo           *repository.UserRepository
	activityLogService *ActivityLogService
}

func NewAuthService(userRepo *repository.UserRepository, activityLogService *ActivityLogService) *AuthService {
	return &AuthService{userRepo: userRepo, activityLogService: activityLogService}
}

func (s *AuthService) Login(username, password, ipAddress, userAgent string) (userID int, fullName, role, token string, isSuperAdmin bool, err error) {
	u, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return 0, "", "", "", false, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		s.activityLogService.LogAuth(0, username, "", "login", false, ipAddress, userAgent, "Invalid password")
		return 0, "", "", "", false, ErrInvalidCredentials
	}

	// Session token management handled by globalAuthService
	s.activityLogService.LogAuth(u.ID, username, u.Role, "login", true, ipAddress, userAgent, "")
	return u.ID, u.FullName, u.Role, "", u.IsSuperAdmin, nil
}

func (s *AuthService) Logout(userID int, username, role, ipAddress, userAgent string) {
	if username != "" {
		s.activityLogService.LogAuth(userID, username, role, "logout", true, ipAddress, userAgent, "")
	}
}
