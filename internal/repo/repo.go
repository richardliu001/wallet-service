package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/richardliu001/wallet-service/internal/model"
	"github.com/segmentio/kafka-go"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrInsufficientFunds indicates insufficient balance.
var ErrInsufficientFunds = errors.New("insufficient funds")

// RepositoryInterface declares all repo operations.
type RepositoryInterface interface {
	DB(ctx context.Context) *gorm.DB
	GetWalletForUpdate(ctx context.Context, tx *gorm.DB, walletID uint64) (*model.Wallet, error)
	CreateWallet(ctx context.Context, tx *gorm.DB, w *model.Wallet) error
	UpdateWallet(ctx context.Context, tx *gorm.DB, walletID uint64, newBalance decimal.Decimal, oldVersion uint64) error
	CreateTransaction(ctx context.Context, tx *gorm.DB, t *model.Transaction) error
	TxExists(ctx context.Context, tx *gorm.DB, walletID uint64, idemKey, txType string) (bool, *model.Transaction, error)
	CreateOutboxEvent(ctx context.Context, tx *gorm.DB, evt *model.OutboxEvent) error
	PollOutbox(ctx context.Context, limit int) ([]model.OutboxEvent, error)
	MarkOutboxProcessed(ctx context.Context, id uint64) error
	PublishEvent(ctx context.Context, evt model.OutboxEvent) error
	CacheBalance(ctx context.Context, walletID uint64, bal decimal.Decimal) error
	GetCachedBalance(ctx context.Context, walletID uint64) (decimal.Decimal, error)
}

// Repository implements RepositoryInterface.
type Repository struct {
	db     *gorm.DB
	rdb    *redis.Client
	writer *kafka.Writer
	log    *zap.SugaredLogger
}

// NewRepository returns Repository instance.
func NewRepository(db *gorm.DB, rdb *redis.Client, w *kafka.Writer, logger *zap.SugaredLogger) *Repository {
	return &Repository{db: db, rdb: rdb, writer: w, log: logger}
}

// DB returns *gorm.DB with ctx.
func (r *Repository) DB(ctx context.Context) *gorm.DB { return r.db.WithContext(ctx) }

// GetWalletForUpdate locks a wallet row.
func (r *Repository) GetWalletForUpdate(ctx context.Context, tx *gorm.DB, walletID uint64) (*model.Wallet, error) {
	var w model.Wallet
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", walletID).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

// CreateWallet inserts a new wallet row.
func (r *Repository) CreateWallet(ctx context.Context, tx *gorm.DB, w *model.Wallet) error {
	return tx.WithContext(ctx).Create(w).Error
}

// UpdateWallet updates balance using optimistic locking.
func (r *Repository) UpdateWallet(ctx context.Context, tx *gorm.DB, walletID uint64, newBalance decimal.Decimal, oldVersion uint64) error {
	res := tx.WithContext(ctx).
		Model(&model.Wallet{}).
		Where("id = ? AND version = ?", walletID, oldVersion).
		Updates(map[string]interface{}{
			"balance":    newBalance,
			"version":    oldVersion + 1,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("optimistic lock conflict")
	}
	return nil
}

// CreateTransaction inserts a transaction record.
func (r *Repository) CreateTransaction(ctx context.Context, tx *gorm.DB, t *model.Transaction) error {
	return tx.WithContext(ctx).Create(t).Error
}

// TxExists checks if a transaction with same idempotency key already exists.
func (r *Repository) TxExists(ctx context.Context, tx *gorm.DB, walletID uint64, idemKey, txType string) (bool, *model.Transaction, error) {
	if idemKey == "" {
		return false, nil, nil
	}
	var t model.Transaction
	err := tx.WithContext(ctx).
		Where("wallet_id=? AND idempotency_key=? AND type=?", walletID, idemKey, txType).
		First(&t).Error
	if err == nil {
		return true, &t, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	return false, nil, err
}

// CreateOutboxEvent inserts an outbox event.
func (r *Repository) CreateOutboxEvent(ctx context.Context, tx *gorm.DB, evt *model.OutboxEvent) error {
	return tx.WithContext(ctx).Create(evt).Error
}

// PollOutbox fetches unprocessed events.
func (r *Repository) PollOutbox(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	var evts []model.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("processed=false").
		Order("created_at").
		Limit(limit).
		Find(&evts).Error
	return evts, err
}

// MarkOutboxProcessed marks event as processed.
func (r *Repository) MarkOutboxProcessed(ctx context.Context, id uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.OutboxEvent{}).
		Where("id=?", id).
		Updates(map[string]interface{}{"processed": true, "processed_at": &now}).Error
}

// PublishEvent writes event to Kafka.
func (r *Repository) PublishEvent(ctx context.Context, evt model.OutboxEvent) error {
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", evt.ID)),
		Value: []byte(evt.Payload),
		Time:  time.Now(),
	}
	return r.writer.WriteMessages(ctx, msg)
}

// CacheBalance caches balance in Redis.
func (r *Repository) CacheBalance(ctx context.Context, walletID uint64, bal decimal.Decimal) error {
	return r.rdb.Set(ctx, fmt.Sprintf("balance:%d", walletID), bal.String(), 5*time.Minute).Err()
}

// GetCachedBalance retrieves balance from Redis.
func (r *Repository) GetCachedBalance(ctx context.Context, walletID uint64) (decimal.Decimal, error) {
	str, err := r.rdb.Get(ctx, fmt.Sprintf("balance:%d", walletID)).Result()
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(str)
}
