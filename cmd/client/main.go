package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")

	connectionString := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer connection.Close()
	fmt.Println("The connection was successful")

	channel, err := connection.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer channel.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		panic(err)
	}

	gamestate := gamelogic.NewGameState(username)
	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, 1, handlerPause(gamestate))
	if err != nil {
		log.Fatalf("error: %v", err.Error())
	}
	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", 1, handlerMove(gamestate))
	if err != nil {
		log.Fatalf("error: %v", err.Error())
	}

	for {
		input := gamelogic.GetInput()

		switch input[0] {
		case "spawn":
			gamestate.CommandSpawn(input)
		case "move":
			move, err := gamestate.CommandMove(input)
			if err != nil {
				fmt.Printf("cannot move %v\n", err.Error())
				continue
			}

			pubsub.PublishJson(channel, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, move)
			fmt.Println("Move was succesful!")
		case "status":
			gamestate.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		default:
			fmt.Printf("Command doesn't exist")
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(ps routing.PlayingState) pubsub.AckType {

	return func(ps routing.PlayingState) pubsub.AckType {
		fmt.Println("*******")
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(move gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		fmt.Println("*******")
		defer fmt.Print("> ")
		moveOutcome := gs.HandleMove(move)
		if moveOutcome == gamelogic.MoveOutcomeMakeWar {
			return pubsub.Ack
		}
		return pubsub.NackDiscard

	}
}
