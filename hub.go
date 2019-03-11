package main

import (
	"encoding/json"
	"log"
)

//Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

//MessageData is WS message payload
type MessageData struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Text     string `json:"text"`
}

//Message is WS message wrapper
type Message struct {
	MsgType string      `json:"type"`
	Data    MessageData `json:"data"`
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) broadcastMessage(message *Message) {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Fatal(err)
	}

	h.broadcast <- messageBytes
}

func (h *Hub) broadcastMessageParams(sender string, receiver string, text string) {
	msg := &Message{
		MsgType: "bulk_msg",
		Data: MessageData{
			Sender:   sender,
			Receiver: receiver,
			Text:     text,
		},
	}
	h.broadcastMessage(msg)
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
