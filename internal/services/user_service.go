package services

import (
	"errors"
	"strings"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrSelfDelete      = errors.New("tidak dapat menghapus akun sendiri")
	ErrProtectedDelete = errors.New("tidak dapat menghapus akun admin utama")
	ErrUserNotFound    = errors.New("user tidak ditemukan")
	ErrUsernameTaken   = errors.New("username sudah digunakan")
	ErrPasswordMismatch = errors.New("password baru dan konfirmasi tidak cocok")
	ErrWrongPassword   = errors.New("password lama salah")
)

type UserService struct {
	userRepo           *repository.UserRepository
	activityLogService *ActivityLogService
}

func NewUserService(userRepo *repository.UserRepository, activityLogService *ActivityLogService) *UserService {
	return &UserService{userRepo: userRepo, activityLogService: activityLogService}
}

func (s *UserService) List() ([]models.User, error) {
	return s.userRepo.List()
}

func (s *UserService) GetByID(id int) (*models.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) CreateUser(actorID int, actorUsername, actorRole, username, password, fullName, role, ipAddress, userAgent string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := s.userRepo.Create(username, string(hash), fullName, role); err != nil {
		return err
	}
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "user", 0, map[string]any{"username": username, "full_name": fullName, "role": role}, ipAddress, userAgent)
	return nil
}

func (s *UserService) DeleteUser(actorID int, targetID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if actorID == targetID {
		return ErrSelfDelete
	}
	u, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return ErrUserNotFound
	}
	if u.Username == "admin" || u.Username == "rekan" {
		return ErrProtectedDelete
	}
	if err := s.userRepo.Delete(targetID); err != nil {
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", targetID, map[string]any{"deleted_username": u.Username}, ipAddress, userAgent)
	return nil
}

func (s *UserService) UpdateProfile(userID int, username, fullName, actorUsername, actorRole, ipAddress, userAgent string) (string, string, error) {
	username = strings.TrimSpace(username)
	fullName = strings.TrimSpace(fullName)

	exists, _ := s.userRepo.ExistsUsername(username, userID)
	if exists {
		return "", "", ErrUsernameTaken
	}
	if err := s.userRepo.UpdateProfile(userID, username, fullName); err != nil {
		return "", "", err
	}
	s.activityLogService.LogUpdate(userID, actorUsername, actorRole, "user", userID,
		map[string]any{"id": userID},
		map[string]any{"username": username, "full_name": fullName}, ipAddress, userAgent)
	return username, fullName, nil
}

func (s *UserService) ChangePassword(userID int, oldPassword, newPassword, confirmPassword string) error {
	if newPassword != confirmPassword {
		return ErrPasswordMismatch
	}
	hash, err := s.userRepo.GetPasswordHash(userID)
	if err != nil {
		return ErrUserNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) != nil {
		return ErrWrongPassword
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.userRepo.UpdatePassword(userID, string(newHash))
}
