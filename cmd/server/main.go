package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	gamelogic.PrintServerHelp()

	connectionString := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer connection.Close()
	fmt.Println("The connection was successful")

	connectionChannel, err := connection.Channel()
	if err != nil {
		panic(err)
	}
	defer connectionChannel.Close()

	pubsub.DeclareAndBind(connection, routing.ExchangePerilTopic, "game_logs", "game_logs.*", 0)

	for {
		input := gamelogic.GetInput()
		if input[0] == "pause" {
			fmt.Printf("Sending a pause message\n")
			encodedState := routing.PlayingState{
				IsPaused: true,
			}

			pubsub.PublishJson(connectionChannel, routing.ExchangePerilDirect, routing.PauseKey, &encodedState)

		} else if input[0] == "resume" {
			fmt.Printf("Sending a resume message\n")
			encodedState, err := json.Marshal(&routing.PlayingState{
				IsPaused: false,
			})
			if err != nil {
				panic(err)
			}
			pubsub.PublishJson(connectionChannel, routing.ExchangePerilDirect, routing.PauseKey, encodedState)
		} else if input[0] == "quit" {
			fmt.Println("Program shutting down")
			os.Exit(0)
		} else {
			fmt.Println("I don't understand what you mean")
		}
	}

}
