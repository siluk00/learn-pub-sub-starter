package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

	//Handles server pause
	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username,
		routing.PauseKey,
		1,
		handlerPause(gamestate),
	)
	if err != nil {
		log.Fatalf("error: %v", err.Error())
	}

	//Handles move
	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+username,
		routing.ArmyMovesPrefix+".*",
		1,
		func(move *gamelogic.ArmyMove) pubsub.AckType {
			fmt.Println("*******")
			defer fmt.Print("> ")
			moveOutcome := gamestate.HandleMove(*move)
			if moveOutcome == gamelogic.MoveOutcomeMakeWar {
				recognition := gamelogic.RecognitionOfWar{
					Attacker: gamestate.Player,
					Defender: move.Player,
				}
				err = pubsub.PublishJson(
					channel,
					routing.ExchangePerilTopic,
					routing.WarRecognitionsPrefix+"."+username,
					&recognition,
				)
				if err != nil {
					log.Printf("Error publishing Move")
					return pubsub.NackRequeue
				}

				return pubsub.Ack
			}
			return pubsub.NackDiscard
		},
	)
	if err != nil {
		log.Fatalf("error: %v", err.Error())
	}

	//Handles Recognition of war
	err = pubsub.SubscribeJSON(connection,
		routing.ExchangePerilTopic,
		"war",
		routing.WarRecognitionsPrefix+".*",
		0,
		func(rw *gamelogic.RecognitionOfWar) pubsub.AckType {
			outcome, winner, loser := gamestate.HandleWar(*rw)
			gamelog := routing.GameLog{
				CurrentTime: time.Now(),
				Username:    username,
			}
			switch outcome {
			case gamelogic.WarOutcomeNotInvolved:
				return pubsub.NackRequeue
			case gamelogic.WarOutcomeNoUnits:
				return pubsub.NackDiscard
			case gamelogic.WarOutcomeOpponentWon:
				fallthrough
			case gamelogic.WarOutcomeYouWon:
				gamelog.Message = winner + " won a war against " + loser
				pubsub.PublishGob(channel,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+username,
					&gamelog,
				)
				if err != nil {
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			case gamelogic.WarOutcomeDraw:
				gamelog.Message = "A war between " + winner + " and " + loser + " resulted in a draw"
				err = pubsub.PublishGob(channel,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+username,
					&gamelog,
				)
				if err != nil {
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			default:
				fmt.Print("error in war")
				return pubsub.NackDiscard
			}
		},
	)
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

			pubsub.PublishJson(
				channel,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+username,
				&move,
			)
			fmt.Println("Move was succesful!")
		case "status":
			gamestate.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(input) < 2 {
				fmt.Println("Ensure one more argument")
				break
			}
			inputInteger, err := strconv.Atoi(input[1])
			if err != nil {
				fmt.Println("The second argument should be a number")
				break
			}
			for i := 0; i < inputInteger; i++ {
				maliciousLog := gamelogic.GetMaliciousLog()
				pubsub.PublishJson(channel,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+username,
					&struct {
						message string
					}{
						message: maliciousLog,
					},
				)
			}
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
