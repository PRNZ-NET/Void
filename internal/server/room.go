package server

import (
	"sync"
)

type Room struct {
	ID       string
	Password string
	Clients  map[string]*Connection
	mu       sync.RWMutex
}

func NewRoom(id string, password string) *Room {
	return &Room{
		ID:       id,
		Password: password,
		Clients:  make(map[string]*Connection),
	}
}

func (r *Room) AddClient(conn *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[conn.ID] = conn
}

func (r *Room) RemoveClient(connID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Clients, connID)
	return len(r.Clients) == 0
}

func (r *Room) GetClients() map[string]*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*Connection)
	for id, conn := range r.Clients {
		result[id] = conn
	}
	return result
}

func (r *Room) GetClientsExcept(excludeID string) map[string]*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]*Connection)
	for id, conn := range r.Clients {
		if id != excludeID {
			result[id] = conn
		}
	}
	return result
}

func (r *Room) Broadcast(data []byte, excludeID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, conn := range r.Clients {
		if id != excludeID {
			conn.sendData(data)
		}
	}
}

func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Clients)
}
