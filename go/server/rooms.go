package server

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Room struct {
	Host      *websocket.Conn
	Clients   map[*websocket.Conn]bool
	Private   map[*websocket.Conn]bool // Salas privadas
	Broadcast chan []byte              // Canal para mensagens
}

var (
	rooms = make(map[string]*Room)
	mutex = sync.Mutex{}
)

func JoinRoom(roomID string, conn *websocket.Conn, isHost bool) {
	mutex.Lock()
	defer mutex.Unlock()

	room, exists := rooms[roomID]
	if !exists {
		room = &Room{
			Clients:   make(map[*websocket.Conn]bool),
			Private:   make(map[*websocket.Conn]bool),
			Broadcast: make(chan []byte),
		}
		rooms[roomID] = room
	}

	if isHost {
		room.Host = conn
	} else {
		room.Clients[conn] = true
	}

	go room.Start()
}

func (r *Room) Start() {
	for {
		select {
		case message := <-r.Broadcast:
			for client := range r.Clients {
				client.WriteMessage(websocket.TextMessage, message)
			}
		}
	}
}

func RequestPrivateRoom(roomID string, client *websocket.Conn) {
	mutex.Lock()
	defer mutex.Unlock()

	room := rooms[roomID]
	if room == nil {
		return
	}

	delete(room.Clients, client) // Remove da sala pública
	room.Private[client] = true  // Adiciona à sala privada
	room.Host.WriteMessage(websocket.TextMessage, []byte("Sala privada iniciada"))
}
