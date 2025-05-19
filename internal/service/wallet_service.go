package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/richardliu001/wallet-service/internal/model"
	"github.com/richardliu001/wallet-service/internal/repo"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WalletService struct {
	repo repo.RepositoryInterface
	log  *zap.SugaredLogger
}

func NewWalletService(r repo.RepositoryInterface, logger *zap.SugaredLogger) *WalletService {
	return &WalletService{repo: r, log: logger}
}

var ErrInvalidAmount = errors.New("amount must be positive")

// Deposit adds money.
func (s *WalletService) Deposit(ctx context.Context, id uint64, amt decimal.Decimal, key string) (decimal.Decimal, error) {
	if amt.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidAmount
	}
	err := s.repo.DB(ctx).Transaction(func(tx *gorm.DB) error {
		exists, _, err := s.repo.TxExists(ctx, tx, id, key, "DEPOSIT")
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		w, err := s.repo.GetWalletForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		newBal := w.Balance.Add(amt)
		if err := s.repo.UpdateWallet(ctx, tx, id, newBal, w.Version); err != nil {
			return err
		}
		t := &model.Transaction{
			WalletID: id, Type: "DEPOSIT", Amount: amt, BalanceBefore: w.Balance, BalanceAfter: newBal, IdempotencyKey: &key,
		}
		if err := s.repo.CreateTransaction(ctx, tx, t); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]interface{}{"wallet_id": id, "amount": amt, "balance": newBal})
		evt := &model.OutboxEvent{Aggregate: "Wallet", AggregateID: id, EventType: "Deposit", Payload: string(payload)}
		if err := s.repo.CreateOutboxEvent(ctx, tx, evt); err != nil {
			return err
		}
		return s.repo.CacheBalance(ctx, id, newBal)
	})
	if err != nil {
		return decimal.Zero, err
	}
	return s.GetBalance(ctx, id)
}

// Withdraw subtracts.
func (s *WalletService) Withdraw(ctx context.Context, id uint64, amt decimal.Decimal, key string) (decimal.Decimal, error) {
	if amt.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidAmount
	}
	err := s.repo.DB(ctx).Transaction(func(tx *gorm.DB) error {
		exists, _, err := s.repo.TxExists(ctx, tx, id, key, "WITHDRAW")
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		w, err := s.repo.GetWalletForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if w.Balance.LessThan(amt) {
			return repo.ErrInsufficientFunds
		}
		newBal := w.Balance.Sub(amt)
		if err := s.repo.UpdateWallet(ctx, tx, id, newBal, w.Version); err != nil {
			return err
		}
		t := &model.Transaction{
			WalletID: id, Type: "WITHDRAW", Amount: amt, BalanceBefore: w.Balance, BalanceAfter: newBal, IdempotencyKey: &key,
		}
		if err := s.repo.CreateTransaction(ctx, tx, t); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]interface{}{"wallet_id": id, "amount": amt, "balance": newBal})
		evt := &model.OutboxEvent{Aggregate: "Wallet", AggregateID: id, EventType: "Withdraw", Payload: string(payload)}
		if err := s.repo.CreateOutboxEvent(ctx, tx, evt); err != nil {
			return err
		}
		return s.repo.CacheBalance(ctx, id, newBal)
	})
	if err != nil {
		return decimal.Zero, err
	}
	return s.GetBalance(ctx, id)
}

// Transfer money between wallets.
func (s *WalletService) Transfer(ctx context.Context, fromID, toID uint64, amt decimal.Decimal, key string) (decimal.Decimal, decimal.Decimal, error) {
	if amt.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, ErrInvalidAmount
	}
	if fromID == toID {
		return decimal.Zero, decimal.Zero, errors.New("cannot transfer to self")
	}
	var fromBal, toBal decimal.Decimal
	err := s.repo.DB(ctx).Transaction(func(tx *gorm.DB) error {
		exists, txOut, err := s.repo.TxExists(ctx, tx, fromID, key, "TRANSFER_OUT")
		if err != nil {
			return err
		}
		if exists {
			fromBal = txOut.BalanceAfter
			var txIn model.Transaction
			tx.WithContext(ctx).Where("wallet_id=? AND idempotency_key=? AND type=?",
				toID, key, "TRANSFER_IN").First(&txIn)
			toBal = txIn.BalanceAfter
			return nil
		}
		// lock in ID order
		firstID, secondID := fromID, toID
		if secondID < firstID {
			firstID, secondID = secondID, firstID
		}
		w1, err := s.repo.GetWalletForUpdate(ctx, tx, firstID)
		if err != nil {
			return err
		}
		w2, err := s.repo.GetWalletForUpdate(ctx, tx, secondID)
		if err != nil {
			return err
		}
		var wFrom, wTo *model.Wallet
		if firstID == fromID {
			wFrom, wTo = w1, w2
		} else {
			wFrom, wTo = w2, w1
		}
		if wFrom.Balance.LessThan(amt) {
			return repo.ErrInsufficientFunds
		}
		newFrom := wFrom.Balance.Sub(amt)
		newTo := wTo.Balance.Add(amt)
		if err := s.repo.UpdateWallet(ctx, tx, fromID, newFrom, wFrom.Version); err != nil {
			return err
		}
		if err := s.repo.UpdateWallet(ctx, tx, toID, newTo, wTo.Version); err != nil {
			return err
		}
		txOut = &model.Transaction{
			WalletID: fromID, Type: "TRANSFER_OUT", Amount: amt, BalanceBefore: wFrom.Balance, BalanceAfter: newFrom,
			RelatedWalletID: &toID, IdempotencyKey: &key,
		}
		txIn := &model.Transaction{
			WalletID: toID, Type: "TRANSFER_IN", Amount: amt, BalanceBefore: wTo.Balance, BalanceAfter: newTo,
			RelatedWalletID: &fromID, IdempotencyKey: &key,
		}
		if err := s.repo.CreateTransaction(ctx, tx, txOut); err != nil {
			return err
		}
		if err := s.repo.CreateTransaction(ctx, tx, txIn); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]interface{}{"from": fromID, "to": toID, "amount": amt, "balance": newFrom})
		evt := &model.OutboxEvent{Aggregate: "Wallet", AggregateID: fromID, EventType: "Transfer", Payload: string(payload)}
		if err := s.repo.CreateOutboxEvent(ctx, tx, evt); err != nil {
			return err
		}
		if err := s.repo.CacheBalance(ctx, fromID, newFrom); err != nil {
			s.log.Warn(err)
		}
		if err := s.repo.CacheBalance(ctx, toID, newTo); err != nil {
			s.log.Warn(err)
		}
		fromBal, newFrom = newFrom, newFrom
		toBal = newTo
		return nil
	})
	return fromBal, toBal, err
}

// GetBalance returns current balance.
func (s *WalletService) GetBalance(ctx context.Context, walletID uint64) (decimal.Decimal, error) {
	bal, err := s.repo.GetCachedBalance(ctx, walletID)
	if err == nil {
		return bal, nil
	}
	var w model.Wallet
	if err := s.repo.DB(ctx).Where("id=?", walletID).First(&w).Error; err != nil {
		return decimal.Zero, err
	}
	_ = s.repo.CacheBalance(ctx, walletID, w.Balance)
	return w.Balance, nil
}

// GetHistory returns transactions since ...
func (s *WalletService) GetHistory(ctx context.Context, walletID uint64, limit int, since time.Time) ([]model.Transaction, error) {
	var txs []model.Transaction
	err := s.repo.DB(ctx).Where("wallet_id=? AND created_at>=?", walletID, since).
		Order("created_at asc").Limit(limit).Find(&txs).Error
	return txs, err
}
