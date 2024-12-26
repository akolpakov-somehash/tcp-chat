package main

import (
	"context"
	"tcp-chat/internal/server"
)

func main() {
	config := &server.Config{
		Host: "127.0.0.1",
		Port: "8080",
	}
	s := server.NewServer(config)
	context, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Run(context)
}
