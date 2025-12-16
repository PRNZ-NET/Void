package main

import (
	"context"
	"fmt"
	"sync"

	chatclient "Void/internal/client"
	"Void/internal/keyverify"
	"Void/internal/server"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	client *chatclient.ChatClient
	mu     sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) ConnectToRoom(serverAddress string, roomID string, username string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		a.client.Close()
	}

	client, err := chatclient.NewChatClient(username)
	if err != nil {
		return "", err
	}

	client.SetOnMessage(func(userID string, username string, content string) {
		runtime.EventsEmit(a.ctx, "message", userID, username, content)
	})

	client.SetOnPeerJoin(func(userID string, username string, publicKey [32]byte) {
		fingerprint := keyverify.ComputeKeyFingerprint(&publicKey)
		runtime.EventsEmit(a.ctx, "peerJoin", userID, username, fingerprint)
	})

	client.SetOnPeerLeft(func(userID string) {
		runtime.EventsEmit(a.ctx, "peerLeft", userID)
	})

	client.SetOnRoomResponse(func(peers []chatclient.PeerInfo) {
		for _, peer := range peers {
			publicKey, exists := client.GetPeerKey(peer.UserID)
			if exists {
				fingerprint := keyverify.ComputeKeyFingerprint(&publicKey)
				runtime.EventsEmit(a.ctx, "peerJoin", peer.UserID, peer.Username, fingerprint)
			} else {
				runtime.EventsEmit(a.ctx, "peerJoin", peer.UserID, peer.Username, "")
			}
		}
	})

	client.SetOnKeyMismatch(func(userID string, username string, expectedFingerprint string, receivedFingerprint string) {
		runtime.EventsEmit(a.ctx, "keyMismatch", userID, username, expectedFingerprint, receivedFingerprint)
	})

	a.client = client

	if err := client.Connect(serverAddress, roomID); err != nil {
		return "", err
	}

	return "connected", nil
}

func (a *App) SendMessage(content string) error {
	a.mu.Lock()
	client := a.client
	if client == nil {
		a.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	username := client.GetUsername()
	userID := client.GetUserID()
	a.mu.Unlock()

	go func() {
		runtime.EventsEmit(a.ctx, "message", userID, username, content)
	}()

	return client.SendMessage(content)
}

func (a *App) GenerateRoomID() string {
	return server.GenerateRoomID()
}

func (a *App) GetMyPublicKeyFingerprint() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil {
		return ""
	}

	publicKey := a.client.GetPublicKey()
	return keyverify.ComputeKeyFingerprint(&publicKey)
}

func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		err := a.client.Close()
		a.client = nil
		return err
	}
	return nil
}

func (a *App) SetPeerFingerprint(userID string, fingerprint string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		a.client.SetKnownFingerprint(userID, fingerprint)
	}
}

func (a *App) GetPeerFingerprint(userID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		fingerprint, _ := a.client.GetKnownFingerprint(userID)
		return fingerprint
	}
	return ""
}

func (a *App) GetPeerKeyFingerprint(userID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil {
		return ""
	}

	publicKey, exists := a.client.GetPeerKey(userID)
	if !exists {
		return ""
	}

	return keyverify.ComputeKeyFingerprint(&publicKey)
}
