package client

import (
	"crypto/rand"
	"io"
	"net"
	"sync"

	"Void/internal/crypto"
	"Void/internal/keyverify"
	"Void/proto/chatpb"

	"golang.org/x/crypto/nacl/box"
	"google.golang.org/protobuf/proto"
)

type PeerInfo struct {
	UserID   string
	Username string
}

type ChatClient struct {
	conn                net.Conn
	publicKey           *[32]byte
	privateKey          *[32]byte
	peers               map[string][32]byte
	peersMu             sync.RWMutex
	knownFingerprints   map[string]string
	fingerprintsMu      sync.RWMutex
	username            string
	roomID              string
	myUserID            string
	onMessage           func(userID string, username string, content string)
	onPeerJoin          func(userID string, username string, publicKey [32]byte)
	onPeerLeft          func(userID string)
	onRoomResponse      func(peers []PeerInfo)
	onKeyMismatch       func(userID string, username string, expectedFingerprint string, receivedFingerprint string)
}

func NewChatClient(username string) (*ChatClient, error) {
	publicKey, privateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	return &ChatClient{
		publicKey:         publicKey,
		privateKey:        privateKey,
		peers:             make(map[string][32]byte),
		knownFingerprints: make(map[string]string),
		username:          username,
		onMessage:         func(string, string, string) {},
		onPeerJoin:        func(string, string, [32]byte) {},
		onPeerLeft:        func(string) {},
		onRoomResponse:    func([]PeerInfo) {},
		onKeyMismatch:     func(string, string, string, string) {},
	}, nil
}

func (cc *ChatClient) Connect(address string, roomID string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	cc.conn = conn
	cc.roomID = roomID

	req := &chatpb.ClientMessage{
		Payload: &chatpb.ClientMessage_JoinRoom{
			JoinRoom: &chatpb.RoomRequest{
				RoomId:    roomID,
				UserId:    "",
				Username:  cc.username,
				PublicKey: cc.publicKey[:],
			},
		},
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	if _, err := conn.Write(data); err != nil {
		return err
	}

	go cc.readLoop()
	return nil
}

func (cc *ChatClient) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := cc.conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				return
			}
			break
		}

		msg := &chatpb.ServerMessage{}
		if err := proto.Unmarshal(buf[:n], msg); err != nil {
			continue
		}

		switch payload := msg.Payload.(type) {
		case *chatpb.ServerMessage_Message:
			cc.receiveMessage(payload.Message)
		case *chatpb.ServerMessage_PeerJoined:
			cc.peerJoined(payload.PeerJoined)
		case *chatpb.ServerMessage_PeerLeft:
			cc.peerLeft(payload.PeerLeft)
		case *chatpb.ServerMessage_RoomResponse:
			cc.roomResponse(payload.RoomResponse)
		}
	}
}

func (cc *ChatClient) roomResponse(resp *chatpb.RoomResponse) {
	cc.myUserID = resp.GetUserId()
	cc.peersMu.Lock()
	peerInfos := make([]PeerInfo, 0, len(resp.GetPeers()))
	for _, peer := range resp.GetPeers() {
		var key [32]byte
		copy(key[:], peer.GetPublicKey())
		userID := peer.GetUserId()
		cc.peers[userID] = key
		cc.verifyPeerKey(userID, peer.GetUsername(), &key)
		peerInfos = append(peerInfos, PeerInfo{
			UserID:   userID,
			Username: peer.GetUsername(),
		})
	}
	cc.peersMu.Unlock()
	cc.onRoomResponse(peerInfos)
}

func (cc *ChatClient) peerJoined(peer *chatpb.PeerJoined) {
	var key [32]byte
	copy(key[:], peer.PublicKey)
	userID := peer.UserId
	cc.peersMu.Lock()
	cc.peers[userID] = key
	cc.peersMu.Unlock()
	cc.verifyPeerKey(userID, peer.Username, &key)
	cc.onPeerJoin(userID, peer.Username, key)
}

func (cc *ChatClient) peerLeft(peer *chatpb.PeerLeft) {
	cc.peersMu.Lock()
	delete(cc.peers, peer.UserId)
	cc.peersMu.Unlock()
	cc.onPeerLeft(peer.UserId)
}

func (cc *ChatClient) receiveMessage(msg *chatpb.ReceiveMessage) {
	cc.peersMu.RLock()
	peerKey, exists := cc.peers[msg.UserId]
	cc.peersMu.RUnlock()

	if !exists {
		return
	}

	messages, err := crypto.UnpackEncryptedMessages(msg.EncryptedContent)
	if err != nil || len(messages) == 0 {
		return
	}

	decrypted, err := crypto.DecryptMessage(messages[0], &peerKey, cc.privateKey)
	if err == nil {
		cc.onMessage(msg.UserId, msg.Username, string(decrypted))
	}
}

func (cc *ChatClient) SendMessage(content string) error {
	if len(content) == 0 {
		return nil
	}

	encryptedMessages, err := cc.encryptForAllPeers(content)
	if err != nil || len(encryptedMessages) == 0 {
		return err
	}

	packed := crypto.PackEncryptedMessages(encryptedMessages)

	msg := &chatpb.ClientMessage{
		Payload: &chatpb.ClientMessage_SendMessage{
			SendMessage: &chatpb.SendMessage{
				RoomId:           cc.roomID,
				EncryptedContent: packed,
			},
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = cc.conn.Write(data)
	return err
}

func (cc *ChatClient) encryptForAllPeers(content string) ([][]byte, error) {
	cc.peersMu.RLock()
	defer cc.peersMu.RUnlock()

	if len(cc.peers) == 0 {
		return nil, nil
	}

	type peerKeyPair struct {
		userID string
		key    [32]byte
	}
	
	peers := make([]peerKeyPair, 0, len(cc.peers))
	for userID, key := range cc.peers {
		peers = append(peers, peerKeyPair{userID: userID, key: key})
	}

	contentBytes := []byte(content)
	encryptedMessages := make([][]byte, 0, len(peers))

	for _, peer := range peers {
		encrypted, err := crypto.EncryptMessage(contentBytes, &peer.key, cc.privateKey)
		if err != nil {
			continue
		}
		encryptedMessages = append(encryptedMessages, encrypted)
	}

	return encryptedMessages, nil
}

func (cc *ChatClient) SetOnMessage(fn func(userID string, username string, content string)) {
	cc.onMessage = fn
}

func (cc *ChatClient) SetOnPeerJoin(fn func(userID string, username string, publicKey [32]byte)) {
	cc.onPeerJoin = fn
}

func (cc *ChatClient) SetOnPeerLeft(fn func(userID string)) {
	cc.onPeerLeft = fn
}

func (cc *ChatClient) SetOnRoomResponse(fn func(peers []PeerInfo)) {
	cc.onRoomResponse = fn
}

func (cc *ChatClient) SetOnKeyMismatch(fn func(userID string, username string, expectedFingerprint string, receivedFingerprint string)) {
	cc.onKeyMismatch = fn
}

func (cc *ChatClient) verifyPeerKey(userID string, username string, publicKey *[32]byte) {
	currentFingerprint := keyverify.ComputeKeyFingerprint(publicKey)
	
	cc.fingerprintsMu.RLock()
	knownFingerprint, exists := cc.knownFingerprints[userID]
	cc.fingerprintsMu.RUnlock()
	
	if exists && knownFingerprint != currentFingerprint {
		cc.onKeyMismatch(userID, username, knownFingerprint, currentFingerprint)
		return
	}
	
	if !exists {
		cc.fingerprintsMu.Lock()
		cc.knownFingerprints[userID] = currentFingerprint
		cc.fingerprintsMu.Unlock()
	}
}

func (cc *ChatClient) SetKnownFingerprint(userID string, fingerprint string) {
	cc.fingerprintsMu.Lock()
	cc.knownFingerprints[userID] = fingerprint
	cc.fingerprintsMu.Unlock()
}

func (cc *ChatClient) GetKnownFingerprint(userID string) (string, bool) {
	cc.fingerprintsMu.RLock()
	defer cc.fingerprintsMu.RUnlock()
	fingerprint, exists := cc.knownFingerprints[userID]
	return fingerprint, exists
}

func (cc *ChatClient) GetPublicKey() [32]byte {
	return *cc.publicKey
}

func (cc *ChatClient) GetUserID() string {
	return cc.myUserID
}

func (cc *ChatClient) GetUsername() string {
	return cc.username
}

func (cc *ChatClient) GetPeerKey(userID string) ([32]byte, bool) {
	cc.peersMu.RLock()
	defer cc.peersMu.RUnlock()
	key, exists := cc.peers[userID]
	return key, exists
}

func (cc *ChatClient) Close() error {
	if cc.conn != nil {
		msg := &chatpb.ClientMessage{
			Payload: &chatpb.ClientMessage_LeaveRoom{
				LeaveRoom: &chatpb.RoomRequest{
					RoomId: cc.roomID,
				},
			},
		}
		data, _ := proto.Marshal(msg)
		cc.conn.Write(data)
		return cc.conn.Close()
	}
	return nil
}

