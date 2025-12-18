package server

import (
	"net"
	"time"

	"Void/internal/crypto"
	"Void/proto/chatpb"

	"google.golang.org/protobuf/proto"
)

type Connection struct {
	ID        string
	Username  string
	PublicKey [32]byte
	Conn      net.Conn
	Room      *Room
	send      chan []byte
	server    *Server
}

func newConnection(conn net.Conn, srv *Server) *Connection {
	return &Connection{
		ID:     generateID(),
		Conn:   conn,
		send:   make(chan []byte, 256),
		server: srv,
	}
}

func (c *Connection) readPump() {
	buf := make([]byte, 4096)
	for {
		n, err := c.Conn.Read(buf)
		if err != nil {
			break
		}

		msg := &chatpb.ClientMessage{}
		if err := proto.Unmarshal(buf[:n], msg); err != nil {
			continue
		}

		switch payload := msg.Payload.(type) {
		case *chatpb.ClientMessage_JoinRoom:
			c.joinRoom(payload.JoinRoom)
		case *chatpb.ClientMessage_SendMessage:
			c.sendMessage(payload.SendMessage)
		case *chatpb.ClientMessage_LeaveRoom:
			c.leaveRoom()
		}
	}
}

func (c *Connection) writePump() {
	for message := range c.send {
		if _, err := c.Conn.Write(message); err != nil {
			return
		}
	}
}

func (c *Connection) sendData(data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

func (c *Connection) joinRoom(req *chatpb.RoomRequest) {
	copy(c.PublicKey[:], req.PublicKey)
	c.Username = req.Username

	s := c.server
	s.roomsMu.Lock()
	room, exists := s.rooms[req.RoomId]

	if !exists {
		room = NewRoom(req.RoomId, req.Password)
		s.rooms[req.RoomId] = room
	} else {
		if room.Password != "" && room.Password != req.Password {
			s.roomsMu.Unlock()
			roomResp := &chatpb.RoomResponse{
				Success: false,
				Message: "Invalid password",
				Peers:   nil,
				UserId:  "",
			}
			response := &chatpb.ServerMessage{
				Payload: &chatpb.ServerMessage_RoomResponse{
					RoomResponse: roomResp,
				},
			}
			data, _ := proto.Marshal(response)
			c.sendData(data)
			return
		}
	}
	s.roomsMu.Unlock()

	room.AddClient(c)
	c.Room = room

	peers := room.GetClientsExcept(c.ID)
	peerList := make([]*chatpb.Peer, 0, len(peers))
	for _, peer := range peers {
		peerList = append(peerList, &chatpb.Peer{
			UserId:    peer.ID,
			Username:  peer.Username,
			PublicKey: peer.PublicKey[:],
		})
	}

	roomResp := &chatpb.RoomResponse{
		Success: true,
		Message: "Joined room",
		Peers:   peerList,
		UserId:  c.ID,
	}
	response := &chatpb.ServerMessage{
		Payload: &chatpb.ServerMessage_RoomResponse{
			RoomResponse: roomResp,
		},
	}

	data, _ := proto.Marshal(response)
	c.sendData(data)

	peerJoined := &chatpb.ServerMessage{
		Payload: &chatpb.ServerMessage_PeerJoined{
			PeerJoined: &chatpb.PeerJoined{
				UserId:    c.ID,
				Username:  c.Username,
				PublicKey: c.PublicKey[:],
			},
		},
	}
	peerJoinedData, _ := proto.Marshal(peerJoined)
	room.Broadcast(peerJoinedData, c.ID)
}

func (c *Connection) sendMessage(msg *chatpb.SendMessage) {
	if c.Room == nil {
		return
	}

	encryptedMessages, err := crypto.UnpackEncryptedMessages(msg.EncryptedContent)
	if err != nil {
		return
	}

	peers := c.Room.GetClientsExcept(c.ID)

	if len(encryptedMessages) != len(peers) {
		return
	}

	msgID := generateID()
	username := c.Username
	userID := c.ID
	timestamp := time.Now().UnixNano()

	peerList := make([]*Connection, 0, len(peers))
	for _, peer := range peers {
		peerList = append(peerList, peer)
	}

	if len(encryptedMessages) != len(peerList) {
		return
	}

	for i, peer := range peerList {
		if i >= len(encryptedMessages) {
			break
		}

		receiveMsg := &chatpb.ReceiveMessage{
			Id:               msgID,
			UserId:           userID,
			Username:         username,
			EncryptedContent: crypto.PackEncryptedMessages([][]byte{encryptedMessages[i]}),
			Timestamp:        timestamp,
		}

		serverMsg := &chatpb.ServerMessage{
			Payload: &chatpb.ServerMessage_Message{
				Message: receiveMsg,
			},
		}
		data, _ := proto.Marshal(serverMsg)
		peer.sendData(data)
	}
}

func (c *Connection) leaveRoom() {
	if c.Room == nil {
		return
	}
	c.server.removeClient(c)
}
