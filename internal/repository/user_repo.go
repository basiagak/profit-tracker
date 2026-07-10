// Package repository holds GORM-backed data access, one file per aggregate.
// Every method that reads/writes shop-scoped data takes/filters by userID.
package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/internal/domain"
)

// UserRepository persists and looks up shop-owner accounts.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository builds a UserRepository over the given GORM handle.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindOrCreateByTelegramID looks up the user owning telegramID, creating one
// on first contact (FR-022: any Telegram account issuing any command is
// auto-provisioned a shop). username/displayName are refreshed on every call
// since Telegram usernames/names are mutable.
func (r *UserRepository) FindOrCreateByTelegramID(telegramID int64, username, displayName *string) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("telegram_id = ?", telegramID).First(&user).Error
	if err == nil {
		user.TelegramUsername = username
		user.DisplayName = displayName
		if err := r.db.Save(&user).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user = domain.User{
		TelegramID:       telegramID,
		TelegramUsername: username,
		DisplayName:      displayName,
	}
	if err := r.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID looks up a user by internal ID.
func (r *UserRepository) FindByID(id int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
