package services

import (
	"errors"
	"fmt"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrSelfDelete       = errors.New("tidak dapat menghapus akun sendiri")
	ErrProtectedDelete  = errors.New("tidak dapat menghapus akun admin utama")
	ErrDeleteNotAllowed = errors.New("hanya akun utama yang dapat menghapus user lain")
	ErrUserNotFound     = errors.New("user tidak ditemukan")
	ErrUsernameTaken    = errors.New("username sudah digunakan")
	ErrPasswordMismatch = errors.New("password baru dan konfirmasi tidak cocok")
	ErrWrongPassword    = errors.New("password lama salah")
	ErrProtectedUpdate  = errors.New("tidak dapat mengubah role user ini")
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

func (s *UserService) ListPaginated(search, role, sortBy, sortOrder string, page, pageSize int) ([]models.User, int, error) {
	return s.userRepo.ListPaginated(search, role, sortBy, sortOrder, page, pageSize)
}

func (s *UserService) GetByID(id int) (*models.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) GetByUsername(username string) (*models.User, error) {
	return s.userRepo.GetByUsername(username)
}

func (s *UserService) CreateUser(actorID int, actorUsername, actorRole, username, password, fullName, role, ipAddress, userAgent string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "user", 0,
			map[string]any{"username": username}, ipAddress, userAgent, err.Error())
		return err
	}
	result, err := s.userRepo.Create(username, string(hash), fullName, role)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "user", 0,
			map[string]any{"username": username, "full_name": fullName, "role": role}, ipAddress, userAgent, err.Error())
		return err
	}
	id, _ := result.LastInsertId()
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "user", int(id), map[string]any{"username": username, "full_name": fullName, "role": role}, ipAddress, userAgent)
	return nil
}

func (s *UserService) DeleteUser(actorID int, targetID int, actorUsername, actorRole string, actorIsSuperAdmin bool, ipAddress, userAgent string) error {
	if actorID == targetID {
		return ErrSelfDelete
	}
	u, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return ErrUserNotFound
	}
	if u.IsProtected {
		return ErrProtectedDelete
	}
	if !actorIsSuperAdmin {
		return ErrDeleteNotAllowed
	}
	if err := s.userRepo.Delete(targetID); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", targetID,
			map[string]any{"deleted_username": u.Username}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", targetID, map[string]any{"deleted_username": u.Username}, ipAddress, userAgent)
	return nil
}

func (s *UserService) UpdateUser(actorID int, targetID int, actorUsername, actorRole, ipAddress, userAgent, username, fullName, role, newPassword string) error {
	username = SanitizeText(username)
	fullName = ToTitleCaseWithAbbr(fullName)

	target, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return ErrUserNotFound
	}

	if target.IsProtected && username != target.Username {
		return ErrProtectedUpdate
	}

	if target.IsProtected && role != target.Role {
		return ErrProtectedUpdate
	}

	exists, _ := s.userRepo.ExistsUsername(username, targetID)
	if exists {
		return ErrUsernameTaken
	}

	oldVals := map[string]any{}
	newVals := map[string]any{}
	if target.Username != username { oldVals["username"] = target.Username; newVals["username"] = username }
	if target.FullName != fullName { oldVals["full_name"] = target.FullName; newVals["full_name"] = fullName }
	if target.Role != role { oldVals["role"] = target.Role; newVals["role"] = role }
	if newPassword != "" { oldVals["password_changed"] = false; newVals["password_changed"] = true }

	if err := s.userRepo.UpdateUser(targetID, username, fullName, role); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "user", targetID,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}

	if newPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "user", targetID,
				oldVals, nil, ipAddress, userAgent, err.Error())
			return err
		}
		if err := s.userRepo.UpdatePassword(targetID, string(hash)); err != nil {
			s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "user", targetID,
				oldVals, nil, ipAddress, userAgent, err.Error())
			return err
		}
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "user", targetID,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *UserService) UpdateProfile(userID int, username, fullName, actorUsername, actorRole, ipAddress, userAgent string) (string, string, error) {
	username = SanitizeText(username)
	fullName = ToTitleCaseWithAbbr(fullName)

	exists, _ := s.userRepo.ExistsUsername(username, userID)
	if exists {
		return "", "", ErrUsernameTaken
	}

	user, _ := s.userRepo.GetByID(userID)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if user != nil {
		if user.Username != username { oldVals["username"] = user.Username; newVals["username"] = username }
		if user.FullName != fullName { oldVals["full_name"] = user.FullName; newVals["full_name"] = fullName }
	}

	if err := s.userRepo.UpdateProfile(userID, username, fullName); err != nil {
		return "", "", err
	}
	s.activityLogService.LogUpdate(userID, actorUsername, actorRole, "user", userID,
		oldVals, newVals, ipAddress, userAgent)
	return username, fullName, nil
}

func (s *UserService) ChangePassword(userID int, oldPassword, newPassword, confirmPassword, actorUsername, actorRole, ipAddress, userAgent string) error {
	if newPassword != confirmPassword {
		s.activityLogService.LogAction(userID, actorUsername, actorRole, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ipAddress, userAgent, ErrPasswordMismatch.Error())
		return ErrPasswordMismatch
	}
	hash, err := s.userRepo.GetPasswordHash(userID)
	if err != nil {
		return ErrUserNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) != nil {
		s.activityLogService.LogAction(userID, actorUsername, actorRole, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ipAddress, userAgent, ErrWrongPassword.Error())
		return ErrWrongPassword
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		s.activityLogService.LogAction(userID, actorUsername, actorRole, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ipAddress, userAgent, err.Error())
		return err
	}
	if err := s.userRepo.UpdatePassword(userID, string(newHash)); err != nil {
		s.activityLogService.LogAction(userID, actorUsername, actorRole, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogAction(userID, actorUsername, actorRole, "update", "user", userID,
		map[string]any{"password_changed": true}, map[string]any{"password_changed": true},
		ipAddress, userAgent)
	return nil
}

func (s *UserService) BatchDeleteUser(actorID int, targetUsernames []string, actorUsername, actorRole string, actorIsSuperAdmin bool, ipAddress, userAgent string) error {
	items := make([]map[string]string, 0, len(targetUsernames))
	for _, username := range targetUsernames {
		if actorUsername == username {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(targetUsernames), "items": items},
				ipAddress, userAgent, ErrSelfDelete.Error())
			return ErrSelfDelete
		}
		target, err := s.GetByUsername(username)
		if err != nil {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(targetUsernames), "items": items},
				ipAddress, userAgent, "user "+username+" not found")
			return fmt.Errorf("user %s tidak ditemukan", username)
		}
		if target.IsProtected {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(targetUsernames), "items": items},
				ipAddress, userAgent, ErrProtectedDelete.Error())
			return ErrProtectedDelete
		}
		if !actorIsSuperAdmin {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(targetUsernames), "items": items},
				ipAddress, userAgent, ErrDeleteNotAllowed.Error())
			return ErrDeleteNotAllowed
		}
		info := map[string]string{"username": target.Username, "full_name": target.FullName}
		if err := s.userRepo.Delete(target.ID); err != nil {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(targetUsernames), "items": items},
				ipAddress, userAgent, err.Error())
			return err
		}
		items = append(items, info)
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "user", 0,
		map[string]any{"action": "batch_delete", "count": len(targetUsernames), "items": items},
		ipAddress, userAgent)
	return nil
}
