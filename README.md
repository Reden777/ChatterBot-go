# ChatterBot Go

ChatterBot Go is a small, language-independent conversational engine. It learns
which statements follow other statements, finds the closest known prompt, and
chooses a response weighted by how often that response has been observed.

This first Go version intentionally focuses on classic ChatterBot behavior.
Storage and logic adapter systems can be added later; the built-in JSON storage
is enough to run a persistent chatbot today.

## Run the terminal chatbot

Go 1.23 or newer is required.

```sh
go run ./cmd/chatterbot
```

Knowledge is saved to `chatterbot.json`. Use `-db` to choose another file and
`-name` to change the bot's name:

```sh
go run ./cmd/chatterbot -name Alice -db data/alice.json
```

The bot learns consecutive messages during the conversation. Type `/help` to
see the available commands, including explicit training:

```text
/teach hello | Hi there!
```

Type `/quit` or press Ctrl-D to exit.

## Use as a library

```go
package main

import (
	"fmt"
	"log"

	chatterbot "github.com/Reden777/ChatterBot-go"
)

func main() {
	bot, err := chatterbot.New(
		"Ron Obvious",
		chatterbot.WithStorage("knowledge.json"),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = bot.Train([]string{
		"Good morning! How are you doing?",
		"I am doing very well, thank you for asking.",
		"You're welcome.",
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err := bot.GetResponse("Good morning, how are you doing?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Text, response.Confidence)
}
```

`Train` accepts an ordered conversation and learns every adjacent pair.
`Learn` adds a single prompt-response pair. `GetResponse` performs fuzzy
matching and also learns from the current live conversation. All methods on a
bot are safe to call concurrently.

## Test

```sh
go test ./...
```

ChatterBot is licensed under the BSD 3-clause license.
