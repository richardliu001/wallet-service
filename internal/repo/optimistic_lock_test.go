package repo

import (
	"context"
	"sync"
	"testing"

	"github.com/richardliu001/wallet-service/internal/logger"
	"github.com/richardliu001/wallet-service/internal/model"
	"github.com/segmentio/kafka-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOptimisticLock_ConcurrentUpdate(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&model.Wallet{})

	// seed wallet
	db.Create(&model.Wallet{ID: 1, Balance: decimal.NewFromInt(100)})

	repo := NewRepository(db, nil, &kafka.Writer{}, must(logger.NewLogger()))

	wg := sync.WaitGroup{}
	success := 0

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = db.Transaction(func(tx *gorm.DB) error {
				w, err := repo.GetWalletForUpdate(context.Background(), tx, 1)
				if err != nil {
					return err
				}
				return repo.UpdateWallet(context.Background(), tx, 1,
					w.Balance.Add(decimal.NewFromInt(10)), w.Version)
			})
		}()
	}
	wg.Wait()

	var final model.Wallet
	_ = db.First(&final, 1).Error

	if final.Balance.Equal(decimal.NewFromInt(110)) {
		success = 1
	}
	assert.Equal(t, 1, success, "only one goroutine should succeed with optimistic lock")
}

func must(l *zap.SugaredLogger, err error) *zap.SugaredLogger {
	if err != nil {
		panic(err)
	}
	return l
}
