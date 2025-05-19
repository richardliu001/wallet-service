package main

import (
	"context"
	"fmt"
	"time"

	"github.com/richardliu001/wallet-service/internal/config"
	"github.com/richardliu001/wallet-service/internal/logger"
	"github.com/richardliu001/wallet-service/internal/repo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
)

func main() {
	cfg, err := config.Load("internal/config/config.yaml")
	if err != nil {
		panic(fmt.Errorf("load config: %w", err))
	}

	log, err := logger.NewLogger()
	if err != nil {
		panic(fmt.Errorf("init logger: %w", err))
	}
	defer log.Sync()

	gdb, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{PrepareStmt: true})
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	kw := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Topic:    cfg.Kafka.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	repo := repo.NewRepository(gdb, rdb, kw, log)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Info("wallet-poller started")
	for range ticker.C {
		ctx := context.Background()
		events, err := repo.PollOutbox(ctx, 100)
		if err != nil {
			log.Errorf("poll outbox: %v", err)
			continue
		}
		for _, evt := range events {
			if err := repo.PublishEvent(ctx, evt); err != nil {
				log.Errorf("publish id=%d: %v", evt.ID, err)
				continue
			}
			if err := repo.MarkOutboxProcessed(ctx, evt.ID); err != nil {
				log.Errorf("mark processed id=%d: %v", evt.ID, err)
			} else {
				log.Infof("event %d sent", evt.ID)
			}
		}
	}
}
