package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/richardliu001/wallet-service/internal/config"
	"github.com/richardliu001/wallet-service/internal/logger"
	"github.com/richardliu001/wallet-service/internal/model"
	"github.com/richardliu001/wallet-service/internal/repo"
	"github.com/richardliu001/wallet-service/internal/service"
	httptransport "github.com/richardliu001/wallet-service/internal/transport/http"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. load config
	cfg, err := config.Load("internal/config/config.yaml")
	if err != nil {
		panic(fmt.Errorf("load config: %w", err))
	}

	// 2. init logger
	log, err := logger.NewLogger()
	if err != nil {
		panic(fmt.Errorf("init logger: %w", err))
	}
	defer log.Sync()

	// 3. postgres
	gdb, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{PrepareStmt: true})
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	if err := gdb.AutoMigrate(&model.Wallet{}, &model.Transaction{}, &model.OutboxEvent{}); err != nil {
		log.Fatalf("auto-migrate: %v", err)
	}

	// 4. redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	// 5. kafka writer
	kw := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Topic:    cfg.Kafka.Topic,
		Balancer: &kafka.LeastBytes{},
	}

	// 6. repo & service
	repository := repo.NewRepository(gdb, rdb, kw, log)
	svc := service.NewWalletService(repository, log)

	// 7. gin router
	router := httptransport.NewRouter(svc, cfg.RateLimit, log)

	// 8. serve
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Infof("wallet-server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
