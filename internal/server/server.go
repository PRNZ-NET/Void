package server

import (
	"log"
	"net"
	"sync"

	"Void/proto/chatpb"

	"google.golang.org/protobuf/proto"
)

type Server struct {
	rooms   map[string]*Room
	roomsMu sync.RWMutex
	port    string
}

func NewServer(port string) *Server {
	return &Server{
		rooms: make(map[string]*Room),
		port:  port,
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("Server started on port %s", s.port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go s.serveClient(conn)
	}
}

func (s *Server) serveClient(conn net.Conn) {
	client := newConnection(conn, s)

	defer func() {
		close(client.send)
		conn.Close()
		if client.Room != nil {
			s.removeClient(client)
		}
	}()

	go client.writePump()
	client.readPump()
}

func (s *Server) removeClient(c *Connection) {
	if c.Room == nil {
		return
	}

	empty := c.Room.RemoveClient(c.ID)

	if !empty {
		peerLeft := &chatpb.ServerMessage{
			Payload: &chatpb.ServerMessage_PeerLeft{
				PeerLeft: &chatpb.PeerLeft{
					UserId: c.ID,
				},
			},
		}
		data, _ := proto.Marshal(peerLeft)
		c.Room.Broadcast(data, c.ID)
	}

	if empty {
		s.roomsMu.Lock()
		delete(s.rooms, c.Room.ID)
		s.roomsMu.Unlock()
	}

	c.Room = nil
}

func (s *Server) GetPort() string {
	return s.port
}

