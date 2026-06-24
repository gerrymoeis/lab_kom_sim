package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials      = errors.New("username atau password salah")
	ErrAlreadyLoggedIn         = errors.New("akun sudah login di tempat lain")
	ErrProtectedUser            = errors.New("cannot delete protected user")
	ErrCannotDeleteSuperAdmin   = errors.New("cannot delete super admin")
	ErrCannotDeleteGlobalAdmin  = errors.New("cannot delete global admin")
)

var DefaultPasswordMap = map[string]string{
	"admin": "admin123",
}

var MainAccountPasswordSuffix = "123"

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

// --- Admin user management ---

func (s *GlobalAuthService) ListUsers() ([]models.GlobalUser, error) {
	return s.userRepo.List()
}

func (s *GlobalAuthService) GetUser(id int) (*models.GlobalUser, error) {
	return s.userRepo.GetByID(id)
}

func (s *GlobalAuthService) GetUserByUsername(username string) (*models.GlobalUser, error) {
	return s.userRepo.GetByUsername(username)
}

func (s *GlobalAuthService) CreateUser(username, password, fullName string, isSuperAdmin, isGlobalAdmin bool) (*models.GlobalUser, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("gagal hash password: %w", err)
	}
	_, err = s.userRepo.Create(username, string(hash), fullName, isSuperAdmin, isGlobalAdmin)
	if err != nil {
		return nil, err
	}
	return s.userRepo.GetByUsername(username)
}

func (s *GlobalAuthService) UpdateUser(id int, username, fullName string, isSuperAdmin, isGlobalAdmin bool) error {
	old, err := s.userRepo.GetByID(id)
	if err != nil {
		return err
	}
	if old.Username != username {
		_ = s.userRepo.ClearDefaultPasswordFlag(id)
	}
	return s.userRepo.Update(id, username, fullName, isSuperAdmin, isGlobalAdmin)
}

func (s *GlobalAuthService) DeleteUser(id int) error {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return err
	}
	if user.IsProtected {
		return ErrProtectedUser
	}
	if user.IsSuperAdmin {
		return ErrCannotDeleteSuperAdmin
	}
	if user.IsGlobalAdmin {
		return ErrCannotDeleteGlobalAdmin
	}
	s.userRepo.ClearPermissions(id)
	s.userRepo.ClearSessionToken(id)
	return s.userRepo.Delete(id)
}

func (s *GlobalAuthService) UpdateUserPassword(id int, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("gagal hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(id, string(hash)); err != nil {
		return err
	}
	return s.userRepo.ClearDefaultPasswordFlag(id)
}

func (s *GlobalAuthService) ClearDefaultPasswordFlag(userID int) error {
	return s.userRepo.ClearDefaultPasswordFlag(userID)
}

func (s *GlobalAuthService) GetDefaultPasswordUsers() ([]models.DefaultCredential, error) {
	creds, err := s.userRepo.GetUsersWithDefaultPassword()
	if err != nil {
		return nil, err
	}
	for i := range creds {
		if creds[i].IsSuperAdmin {
			if pw, ok := DefaultPasswordMap[creds[i].Username]; ok {
				creds[i].Password = pw
			}
		} else if creds[i].IsMainAccount {
			creds[i].Password = creds[i].Username + MainAccountPasswordSuffix
		}
	}
	return creds, nil
}

func (s *GlobalAuthService) ListUsersByLab(labURLPath, search, role, sortBy, sortOrder string, page, pageSize int) ([]models.GlobalUser, int, error) {
	return s.userRepo.ListByLabPaginated(labURLPath, search, role, sortBy, sortOrder, page, pageSize)
}

func (s *GlobalAuthService) GetUserByUsernameAndLab(username, labURLPath string) (*models.GlobalUser, error) {
	return s.userRepo.GetByUsernameAndLab(username, labURLPath)
}

func (s *GlobalAuthService) RemoveLabPermission(userID int, labURLPath string) error {
	return s.userRepo.RemovePermission(userID, labURLPath)
}

func (s *GlobalAuthService) GetUsernamesForLab(labURLPath string) ([]string, error) {
	return s.userRepo.GetUsernamesByLab(labURLPath)
}

func (s *GlobalAuthService) SetUserPermissions(userID int, permissions []struct {
	LabURLPath string
	Role       string
}) error {
	s.userRepo.ClearPermissions(userID)
	for _, p := range permissions {
		if err := s.userRepo.AddPermission(userID, p.LabURLPath, p.Role); err != nil {
			return err
		}
	}
	return nil
}
