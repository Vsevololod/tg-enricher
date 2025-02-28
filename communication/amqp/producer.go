package amqp

import (
	"encoding/json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"log"
	"log/slog"
	"tg-enricher/domain"
	"tg-enricher/lib"
	"tg-enricher/lib/logger/sl"

	"github.com/rabbitmq/amqp091-go"
)

// Producer отвечает за отправку сообщений в RabbitMQ через Exchange
type Producer struct {
	conn       *amqp091.Connection
	channel    *amqp091.Channel
	exchange   string
	routingKey string
	log        *slog.Logger
}

// NewProducer создает нового продюсера и подключается к RabbitMQ
func NewProducer(amqpURL, exchange, routingKey string, log *slog.Logger) (*Producer, error) {
	log.Info("Create Consumer")
	conn, err := amqp091.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Объявляем Exchange (если он не объявлен заранее)
	err = ch.ExchangeDeclare(
		exchange, // Имя Exchange
		"direct", // Тип (direct, topic, fanout, headers)
		true,     // Долговечный (durable)
		false,    // Автоудаляемый (auto-delete)
		false,    // Внутренний
		false,    // Без ожидания подтверждения
		nil,      // Аргументы
	)
	if err != nil {
		conn.Close()
		ch.Close()
		return nil, err
	}

	return &Producer{
		conn:       conn,
		channel:    ch,
		exchange:   exchange,
		routingKey: routingKey,
		log:        log,
	}, nil
}

// StartPublishing читает сообщения из канала и отправляет их в RabbitMQ через Exchange
func (p *Producer) StartPublishing(messageChannel chan domain.OutputMessageWithContext) {
	for msg := range messageChannel {
		err := p.PublishMessage(msg)
		if err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
	}
}

// PublishMessage отправляет сообщение в RabbitMQ через Exchange
func (p *Producer) PublishMessage(msg domain.OutputMessageWithContext) error {
	ctx, span := otel.Tracer("tg-enricher").Start(msg.Context, "PublishMessage")
	defer span.End()

	body, err := json.Marshal(msg.Message)
	if err != nil {
		return err
	}

	headers := lib.MapCarrierToAMQPTable(msg.Context)
	headers["uuid"] = msg.UUID

	err = p.channel.PublishWithContext(
		ctx,
		p.exchange,      // Exchange
		msg.Destination, // Routing Key (для direct или topic exchange)
		false,           // Mandatory
		false,           // Immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     headers,
		},
	)

	if err != nil {
		span.RecordError(err)
		p.log.Error("Ошибка публикации", sl.Err(err))
	} else {
		span.SetAttributes(attribute.String("uuid", msg.UUID))
		p.log.Info("Сообщение отправлено", slog.String("uuid", msg.UUID))
	}
	return err
}

// Close закрывает соединение с RabbitMQ
func (p *Producer) Close() {
	p.channel.Close()
	p.conn.Close()
}
