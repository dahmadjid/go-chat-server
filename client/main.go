package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"nhooyr.io/websocket"
)

func main() {
	chat_id := os.Args[1]
	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	c, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://localhost:3000/%s", chat_id), nil)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "Error from client")
	go func(ctx context.Context, c *websocket.Conn) {
		for {
			_, msg, err := c.Read(ctx)
			if err != nil {
				fmt.Printf("err: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(string(msg))
		}
	}(ctx, c)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("err: %v\n", err)
			return
		}
		c.Write(ctx, websocket.MessageBinary, []byte(line))
	}
}
