package queue

import (
	"context"

	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

// Consumer define o contrato para consumir eventos.
type Consumer interface {
	Consume(ctx context.Context, handler func(routingKey string, payload []byte) error) error
	Close()
}

type RabbitMQConsumer struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	queueName  string
	exchange   string
	routingKey string
	logger     *zap.Logger
}

func NewRabbitMQConsumer(url, queueName, exchange, routingKey string, logger *zap.Logger) (*RabbitMQConsumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Declarar exchange
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return nil, err
	}

	// Declarar fila durável
	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	// Criar o BIND (Fila escutando a Exchange na Routing Key)
	if err := ch.QueueBind(q.Name, routingKey, exchange, false, nil); err != nil {
		return nil, err
	}

	return &RabbitMQConsumer{
		conn:       conn,
		channel:    ch,
		queueName:  q.Name,
		exchange:   exchange,
		routingKey: routingKey,
		logger:     logger,
	}, nil
}

func (c *RabbitMQConsumer) Consume(ctx context.Context, handler func(routingKey string, payload []byte) error) error {
	msgs, err := c.channel.Consume(
		c.queueName,
		"",    // consumer tag
		false, // auto-ack (desligado para garantirmos o commit manual em caso de sucesso - At Least Once)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return err
	}

	c.logger.Info("Iniciando consumo de mensagens RabbitMQ", zap.String("queue", c.queueName))

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("RabbitMQ Consumer graceful shutdown")
				return
			case d, ok := <-msgs:
				if !ok {
					c.logger.Warn("RabbitMQ channel closed")
					return
				}
				
				// Processa o handler de negócio
				if err := handler(d.RoutingKey, d.Body); err != nil {
					c.logger.Error("Erro ao processar mensagem", zap.Error(err), zap.String("routing_key", d.RoutingKey))
					d.Nack(false, true) // Nack e Requeue para tentar dnv
				} else {
					d.Ack(false) // Sucesso, remove da fila
				}
			}
		}
	}()

	return nil
}

func (c *RabbitMQConsumer) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
