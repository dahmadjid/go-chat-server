package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"nhooyr.io/websocket"
)

type Message struct {
	ID  primitive.ObjectID `bson:"_id,omitempty"`
	Msg []byte             `bson:"msg"`
}

type ChatApp struct {
	redis_client *redis.Client
	mongo_client *mongo.Client
	chats        map[string]chan []byte
}

func NewChatApp() ChatApp {
	var chats = make(map[string]chan []byte)

	redis_client := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	mongo_client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://db:27017"))
	if err != nil {
		fmt.Printf("MONGODB: err %v", err)
	}

	app := ChatApp{redis_client, mongo_client, chats}

	return app

}

func (app *ChatApp) mongo_sub(chat_id string) {
	pubsub := app.redis_client.Subscribe(context.Background(), chat_id)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {

		res, err := app.mongo_client.Database("chat").Collection(chat_id).InsertOne(context.Background(), Message{Msg: []byte(msg.Payload)})
		if err != nil {
			fmt.Printf("err: %v\n", err)
			return
		}
		fmt.Print("Inserted one chat message with id: ")
		fmt.Println(res.InsertedID)
	}

}

func (app *ChatApp) redis_brodcaster(chat_id string) {
	ctx := context.Background()

	for msg := range app.chats[chat_id] {
		app.redis_client.Publish(ctx, chat_id, msg)
	}
}

func (app *ChatApp) sending_loop(ctx context.Context, c *websocket.Conn, chat_id string) {
	for {
		_, msg, err := c.Read(ctx)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			return
		}
		app.chats[chat_id] <- msg
	}
}

func (app *ChatApp) chat_handler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "Error from Server")
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}

	chat_id := chi.URLParam(r, "chat_id")

	// goroutine that listens for broadcast messages from redis
	go app.sending_loop(context.Background(), c, chat_id)

	// create a room if it doesnt exists. kind of room
	if _, ok := app.chats[chat_id]; !ok {
		chat_channel := make(chan []byte)
		go app.redis_brodcaster(chat_id)
		go app.mongo_sub(chat_id)
		app.chats[chat_id] = chat_channel
	}

	pubsub := app.redis_client.Subscribe(context.Background(), chat_id)
	defer pubsub.Close()

	cursor, err := app.mongo_client.Database("chat").Collection(chat_id).Find(context.Background(), bson.D{{}})
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	var message_history []Message
	if err := cursor.All(context.Background(), &message_history); err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	for _, m := range message_history {
		c.Write(r.Context(), websocket.MessageBinary, []byte(m.Msg))
	}
	ch := pubsub.Channel()
	for msg := range ch {
		c.Write(r.Context(), websocket.MessageBinary, []byte(msg.Payload))
	}

}

func main() {

	app := chi.NewRouter()
	chat_app := NewChatApp()
	app.Get("/{chat_id}", chat_app.chat_handler)

	fmt.Println("Server Started on port 3000")
	http.ListenAndServe(":3000", app)
}
