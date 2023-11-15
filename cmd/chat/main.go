package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sno6/chat/internal/openai"
)

func main() {
	var (
		stream = flag.Bool("stream", false, "Stream the response to stdout.")
	)
	flag.Parse()

	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		fmt.Println("$OPENAI_API_KEY should be set.")
		os.Exit(1)
	}

	var prompt string
	if len(flag.Args()) > 0 {
		prompt = flag.Args()[0]
	}

	service := openai.NewService(key, openai.GPT4)

	if *stream {
		streamChat(service, prompt)
	} else {
		syncChat(service, prompt)
	}
}

func streamChat(service openai.Service, prompt string) {
	stream, err := service.ChatAsync(prompt)
	if err != nil {
		fmt.Printf("chat: something went wrong: %v\n", err)
		os.Exit(1)
	}

	for !stream.Done() {
		token := stream.Next()
		fmt.Print(token)
	}
}

func syncChat(service openai.Service, prompt string) {
	response, err := service.ChatSync(prompt)
	if err != nil {
		fmt.Printf("chat: something went wrong: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(response)
}
