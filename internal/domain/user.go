package domain

import "time"

// User is a shop owner, identified and authenticated by their Telegram account.
// Each User row is the single "shop" it owns (FR-023).
type User struct {
	ID               int64     `gorm:"column:id;primaryKey"`
	TelegramID       int64     `gorm:"column:telegram_id;uniqueIndex;not null"`
	TelegramUsername *string   `gorm:"column:telegram_username"`
	DisplayName      *string   `gorm:"column:display_name"`
	CreatedAt        time.Time `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (User) TableName() string { return "users" }
