package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	ID              uint64          `gorm:"primaryKey"`
	WalletID        uint64          `gorm:"not null"`
	Type            string          `gorm:"size:32;not null"`
	Amount          decimal.Decimal `gorm:"type:numeric(20,8);not null"`
	BalanceBefore   decimal.Decimal `gorm:"type:numeric(20,8);not null"`
	BalanceAfter    decimal.Decimal `gorm:"type:numeric(20,8);not null"`
	RelatedWalletID *uint64
	IdempotencyKey  *string   `gorm:"size:64"`
	CreatedAt       time.Time `gorm:"autoCreateTime"`
}

func (Transaction) TableName() string { return "transaction" }
