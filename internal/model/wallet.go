package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Wallet struct {
	ID        uint64          `gorm:"primaryKey;column:id"`
	Balance   decimal.Decimal `gorm:"type:numeric(20,8);not null;default:'0'"`
	Version   uint64          `gorm:"not null;default:0"`
	UpdatedAt time.Time       `gorm:"autoUpdateTime"`
}

func (Wallet) TableName() string { return "wallet" }
