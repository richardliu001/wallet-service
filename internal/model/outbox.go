package model

import "time"

type OutboxEvent struct {
	ID          uint64    `gorm:"primaryKey"`
	Aggregate   string    `gorm:"size:64;not null"`
	AggregateID uint64    `gorm:"not null"`
	EventType   string    `gorm:"size:64;not null"`
	Payload     string    `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	Processed   bool      `gorm:"not null;default:false"`
	ProcessedAt *time.Time
}

func (OutboxEvent) TableName() string { return "event_outbox" }
