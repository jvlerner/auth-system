package queue

import (
	"context"

	"github.com/streadway/amqp"
)

// Publisher define a interface genérica para enviar mensagens a uma fila/exchange.
// Respeita Inversão de Dependência para o Bounded Context não conhecer amqp.
type Publisher interface {
	Publish(ctx context.Context, exchange, routingKey string, payload []byte) error
}

// RabbitMQPublisher é a implementação concreta usando streadway/amqp.
type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitMQPublisher(url string) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &RabbitMQPublisher{
		conn:    conn,
		channel: ch,
	}, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, exchange, routingKey string, payload []byte) error {
	// Cria a exchange caso não exista internamente (Topic, Durável)
	err := p.channel.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return err
	}

	return p.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         payload,
			DeliveryMode: amqp.Persistent, // Garante que a mensagem sobreviva a reboots do RabbitMQ
		})
}

func (p *RabbitMQPublisher) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
