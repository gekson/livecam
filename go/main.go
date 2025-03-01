package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin:     func(r *http.Request) bool { return true },
}

type Room struct {
    HostConn *websocket.Conn
    Clients  map[*websocket.Conn]bool
    mu       sync.Mutex
}

var (
    rooms = make(map[string]*Room)
    mutex = sync.Mutex{}
)

func main() {
    http.HandleFunc("/ws", handleWebSocket)
    log.Println("Servidor rodando na porta 8090...")
    log.Fatal(http.ListenAndServe(":8090", nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("Erro ao fazer upgrade para WebSocket:", err)
        return
    }
    defer conn.Close()

    log.Println("Conexão WebSocket estabelecida")
    _, msg, err := conn.ReadMessage()
    if err != nil {
        log.Println("Erro ao ler mensagem inicial:", err)
        return
    }
    log.Println("Mensagem recebida:", string(msg))

    parts := strings.Split(string(msg), ":")
    if len(parts) != 3 || parts[0] != "JOIN" {
        log.Println("Mensagem inicial inválida:", string(msg))
        return
    }

    roomID := parts[1]
    role := parts[2]
    log.Println("RoomID:", roomID, "Role:", role)

    if role == "host" {
        handleHost(conn, roomID)
    } else if role == "client" {
        handleClient(conn, roomID)
    }
}

func handleHost(wsConn *websocket.Conn, roomID string) {
    mutex.Lock()
    room, exists := rooms[roomID]
    if !exists {
        room = &Room{
            HostConn: wsConn,
            Clients:  make(map[*websocket.Conn]bool),
        }
        rooms[roomID] = room
    } else {
        room.HostConn = wsConn
    }
    mutex.Unlock()

    for {
        _, msg, err := wsConn.ReadMessage()
        if err != nil {
            log.Println("Host desconectado:", err)
            mutex.Lock()
            delete(rooms, roomID)
            mutex.Unlock()
            return
        }
        log.Println("Mensagem do host:", string(msg))

        var data map[string]interface{}
        if err := json.Unmarshal(msg, &data); err != nil {
            log.Println("Erro ao decodificar mensagem JSON (ignorando):", err)
            continue
        }

        room.mu.Lock()
        if data["type"] == "chat" {
            roomId := data["roomId"].(string)
            targetRoom, exists := rooms[roomId]
            if exists {
                broadcastChat(targetRoom, msg, nil)
            }
        } else if data["type"] == "accept-private" {
            clientId := data["clientId"].(string)
            privateRoomID := roomID + "-private-" + clientId
            privateRoom := &Room{
                HostConn: wsConn,
                Clients:  make(map[*websocket.Conn]bool),
            }
            mutex.Lock()
            rooms[privateRoomID] = privateRoom
            mutex.Unlock()

            for clientConn := range room.Clients {
                clientConn.WriteJSON(map[string]interface{}{
                    "type":     "join-private",
                    "roomId":   privateRoomID,
                    "clientId": clientId,
                })
            }
        } else if data["type"] == "reject-private" {
            clientId := data["clientId"].(string)
            for clientConn := range room.Clients {
                clientConn.WriteJSON(map[string]interface{}{
                    "type":     "private-rejected",
                    "clientId": clientId,
                })
            }
        } else {
            for clientConn := range room.Clients {
                clientConn.WriteMessage(websocket.TextMessage, msg)
            }
        }
        room.mu.Unlock()
    }
}

func handleClient(wsConn *websocket.Conn, roomID string) {
    mutex.Lock()
    room, exists := rooms[roomID]
    if !exists || room.HostConn == nil {
        log.Println("Sala ou host não disponível:", roomID)
        mutex.Unlock()
        return
    }
    room.mu.Lock()
    room.Clients[wsConn] = true
    room.mu.Unlock()
    mutex.Unlock()

    defer func() {
        room.mu.Lock()
        delete(room.Clients, wsConn)
        room.mu.Unlock()
        wsConn.Close()
    }()

    for {
        _, msg, err := wsConn.ReadMessage()
        if err != nil {
            log.Println("Cliente desconectado:", err)
            return
        }
        log.Println("Mensagem do cliente:", string(msg))

        if strings.HasPrefix(string(msg), "JOIN:") {
            parts := strings.Split(string(msg), ":")
            newRoomID := parts[1]
            mutex.Lock()
            newRoom, exists := rooms[newRoomID]
            if exists {
                room.mu.Lock()
                delete(room.Clients, wsConn)
                room.mu.Unlock()
                newRoom.mu.Lock()
                newRoom.Clients[wsConn] = true
                newRoom.mu.Unlock()
                room = newRoom
            }
            mutex.Unlock()
            continue
        }

        var data map[string]interface{}
        if err := json.Unmarshal(msg, &data); err != nil {
            log.Println("Erro ao decodificar mensagem JSON (ignorando):", err)
            continue
        }

        room.mu.Lock()
        if room.HostConn != nil {
            if data["type"] == "chat" {
                roomId := data["roomId"].(string)
                targetRoom, exists := rooms[roomId]
                if exists {
                    broadcastChat(targetRoom, msg, nil)
                }
            } else if data["type"] == "request-private" {
                room.HostConn.WriteMessage(websocket.TextMessage, msg)
            } else {
                room.HostConn.WriteMessage(websocket.TextMessage, msg)
            }
        }
        room.mu.Unlock()
    }
}

func broadcastChat(room *Room, msg []byte, excludeConn *websocket.Conn) {
    if room.HostConn != nil && room.HostConn != excludeConn {
        room.HostConn.WriteMessage(websocket.TextMessage, msg)
    }
    for clientConn := range room.Clients {
        clientConn.WriteMessage(websocket.TextMessage, msg)
    }
}
