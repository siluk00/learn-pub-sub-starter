package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AckType string

const (
	Ack         = "Ack"
	NackRequeue = "NackRequeue"
	NackDiscard = "NackDiscard"
)

func PublishJson[T any](ch *amqp.Channel, exchange, key string, val T) error {
	encoding, err := json.Marshal(val)

	if err != nil {
		return err
	}

	if err := ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "appliction/json",
		Body:        encoding,
	}); err != nil {
		return err
	}

	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, simpleQueueType int) (*amqp.Channel, amqp.Queue, error) {
	channel, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	queue, err := channel.QueueDeclare(queueName, simpleQueueType == 0,
		simpleQueueType == 1, simpleQueueType == 1, false, amqp.Table{
			"x-dead-letter-exchange": "peril_dlx",
		})
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	if err = channel.QueueBind(queue.Name, key, exchange, false, nil); err != nil {
		return nil, amqp.Queue{}, err
	}

	return channel, queue, nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, simpleQueueType int, handler func(T) AckType) error {
	channel, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	deliveryChan, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer channel.Close()
		for message := range deliveryChan {
			var theType T
			err := json.Unmarshal(message.Body, &theType)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			ack := handler(theType)
			switch ack {
			case Ack:
				err = message.Ack(false)
				log.Println("Ack")
			case NackRequeue:
				err = message.Nack(false, true)
				log.Println("Nack requeue")
			case NackDiscard:
				err = message.Nack(false, false)
				log.Println("Nack Discard")
			}

			if err != nil {
				fmt.Println(err.Error())
			}

		}
	}()
	return nil
}
