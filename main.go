package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"tg-enricher/communication/amqp"
	"tg-enricher/config"
	"tg-enricher/domain"
	"tg-enricher/lib/logger/sl"
	"tg-enricher/service"
	"tg-enricher/storage/postgresql"
	"tg-enricher/tracing"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {

	cfg := config.MustLoad()
	log := setupLogger(cfg.Env)

	shutdown := tracing.InitTracer(&cfg.OtlpConfig)
	defer shutdown()

	inputMessageChannel := make(chan domain.InputMessageWithContext, 100)
	outputMessageChannel := make(chan domain.OutputMessageWithContext, 100)

	consumer := registerConsumer(inputMessageChannel, &cfg.AmqpConf, log)
	producer := registerProducer(outputMessageChannel, &cfg.AmqpConf, log)
	defer consumer.Close()
	defer producer.Close()

	storage, err := postgresql.New(cfg.PgConf.GetDbUri())
	if err != nil {
		log.Error("Cannot init db", sl.Err(err))
	}
	processService := service.NewMessageProcessService(
		inputMessageChannel, outputMessageChannel, storage, storage, log)
	processService.StartProcessing(5)

	health := NewHealth(storage, consumer, log)
	health.Start()

	// Контекст для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done() // Ждем сигнала завершения

	log.Info("Shutdown signal received. Closing services...")

	// Закрываем consumer
	consumer.Close()
	log.Info("Consumer stopped.")

	// Закрываем процесс обработки сообщений
	processService.StopProcessing()
	log.Info("Message processing stopped.")

	health.StopProcessing()
	log.Info("Stop /health endpoint.")

	log.Info("Shutdown complete.")

}

func registerConsumer(inputMessageChannel chan domain.InputMessageWithContext, cfg *config.AmqpConfig, log *slog.Logger) *amqp.Consumer {
	consumer, err := amqp.NewConsumer(cfg.GetAmqpUri(), cfg.QueueName, log)
	if err != nil {
		log.Error("Ошибка создания потребителя:", sl.Err(err))
	}

	go consumer.StartListening(inputMessageChannel)
	return consumer
}

func registerProducer(outputMessageChannel chan domain.OutputMessageWithContext, cfg *config.AmqpConfig, log *slog.Logger) *amqp.Producer {
	producer, err := amqp.NewProducer(cfg.GetAmqpUri(), cfg.ExchangeName, cfg.RoutingKey, log)
	if err != nil {
		log.Error("Ошибка создания Producer:", sl.Err(err))
	}
	go producer.StartPublishing(outputMessageChannel)
	return producer
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
