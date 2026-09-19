package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	chatterbot "github.com/Reden777/ChatterBot-go"
)

func main() {
	name := flag.String("name", "ChatterBot", "the chatbot's name")
	database := flag.String("db", "chatterbot.json", "path to the knowledge file")
	flag.Parse()
	bot, err := chatterbot.New(
		*name,
		chatterbot.WithStorage(*database),
		chatterbot.WithMathematicalEvaluation(),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s is ready. Type /help for commands.\n", bot.Name())
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "/quit" || input == "/exit" {
			break
		}
		if input == "/help" {
			fmt.Println("/teach PROMPT | RESPONSE  teach a response")
			fmt.Println("/reset                    start a new conversation")
			fmt.Println("/quit                     exit")
			continue
		}
		if input == "/reset" {
			bot.ResetConversation()
			fmt.Println("Conversation reset.")
			continue
		}
		if strings.HasPrefix(input, "/teach ") {
			lesson := strings.TrimSpace(strings.TrimPrefix(input, "/teach "))
			parts := strings.SplitN(lesson, "|", 2)
			if len(parts) != 2 {
				fmt.Println("Usage: /teach PROMPT | RESPONSE")
				continue
			}
			if err := bot.Learn(parts[0], parts[1]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			fmt.Println("Learned.")
			continue
		}
		response, err := bot.GetResponse(input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		fmt.Printf("%s: %s\n", bot.Name(), response.Text)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
