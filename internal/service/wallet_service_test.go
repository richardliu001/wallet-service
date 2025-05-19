package service

import (
	"context"
	"github.com/go-redis/redismock/v8"
	"testing"
	"time"

	"github.com/richardliu001/wallet-service/internal/logger"
	"github.com/richardliu001/wallet-service/internal/model"
	"github.com/richardliu001/wallet-service/internal/repo"
	"github.com/segmentio/kafka-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) (*WalletService, context.Context) {
	// SQLite in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&model.Wallet{}, &model.Transaction{}, &model.OutboxEvent{}))

	// Redis mock
	rdb, mock := redismock.NewClientMock()
	mock.ExpectGet("balance:1").RedisNil()
	mock.ExpectGet("balance:2").RedisNil()
	mock.ExpectSet("balance:1", "100", 0).SetVal("OK")
	mock.ExpectSet("balance:1", "70", 0).SetVal("OK")
	mock.ExpectSet("balance:2", "50", 0).SetVal("OK")
	mock.ExpectSet("balance:2", "80", 0).SetVal("OK")

	writer := &kafka.Writer{} // not used here
	log, _ := logger.NewLogger()
	repository := repo.NewRepository(db, rdb, writer, log)
	svc := NewWalletService(repository, log)

	return svc, context.Background()
}

func TestWalletService_FullFlow(t *testing.T) {
	svc, ctx := newTestService(t)

	// bootstrap two wallets
	err := svc.Repo().DB(ctx).Create(&model.Wallet{ID: 1, Balance: decimal.Zero}).Error
	assert.NoError(t, err)
	err = svc.Repo().DB(ctx).Create(&model.Wallet{ID: 2, Balance: decimal.Zero}).Error
	assert.NoError(t, err)

	// deposit
	bal, err := svc.Deposit(ctx, 1, decimal.NewFromInt(100), "init1")
	assert.NoError(t, err)
	assert.Equal(t, "100", bal.StringFixed(0))

	// withdraw too much (should fail)
	_, err = svc.Withdraw(ctx, 1, decimal.NewFromInt(130), "w1")
	assert.ErrorIs(t, err, repo.ErrInsufficientFunds)

	// transfer 30
	fromBal, toBal, err := svc.Transfer(ctx, 1, 2, decimal.NewFromInt(30), "tx1")
	assert.NoError(t, err)
	assert.Equal(t, "70", fromBal.StringFixed(0))
	assert.Equal(t, "30", toBal.StringFixed(0))

	// idempotent transfer (same key)
	fromBal2, toBal2, err := svc.Transfer(ctx, 1, 2, decimal.NewFromInt(30), "tx1")
	assert.NoError(t, err)
	assert.Equal(t, fromBal, fromBal2)
	assert.Equal(t, toBal, toBal2)

	// balance endpoint logic
	b1, _ := svc.GetBalance(ctx, 1)
	b2, _ := svc.GetBalance(ctx, 2)
	assert.Equal(t, "70", b1.StringFixed(0))
	assert.Equal(t, "30", b2.StringFixed(0))

	// history
	hist, err := svc.GetHistory(ctx, 1, 10, time.Now().Add(-time.Hour))
	assert.NoError(t, err)
	assert.Len(t, hist, 2) // deposit + transfer_out
}
