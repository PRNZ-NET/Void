import { useState, useEffect, useRef } from "react";
import "./App.css";
import {
  ConnectToRoom,
  SendMessage,
  GenerateRoomID,
  Disconnect,
  GetMyPublicKeyFingerprint,
  SetPeerFingerprint,
  GetPeerFingerprint,
  GetPeerKeyFingerprint,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { t, setLanguage, getLanguage } from "./i18n";

interface Message {
  id: string;
  userId: string;
  username: string;
  content: string;
  timestamp: number;
  isSystem?: boolean;
  systemType?: "join" | "leave";
}

interface Peer {
  userId: string;
  username: string;
}

function App() {
  const [connected, setConnected] = useState(false);
  const [nodeUrl, setNodeUrl] = useState("localhost:8080");
  const [roomID, setRoomID] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [joinPassword, setJoinPassword] = useState("");
  const [message, setMessage] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [peers, setPeers] = useState<Peer[]>([]);
  const [myUserId, setMyUserId] = useState<string>("");
  const [language, setLanguageState] = useState(getLanguage());
  const [peerFingerprints, setPeerFingerprints] = useState<Map<string, string>>(
    new Map(),
  );
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messageIdsRef = useRef<Set<string>>(new Set());
  const messageCounterRef = useRef<number>(0);
  const roomLoadedRef = useRef<boolean>(false);

  const changeLanguage = (lang: string) => {
    setLanguage(lang);
    setLanguageState(lang);
  };

  useEffect(() => {
    const messageCallback = (
      userId: string,
      username: string,
      content: string,
    ) => {
      const timestamp = Date.now();
      messageCounterRef.current += 1;
      const msgId = `${userId}-${timestamp}-${messageCounterRef.current}-${content}`;

      setMessages((prev) => {
        const recentMessage = prev.find(
          (msg) =>
            msg.userId === userId &&
            msg.content === content &&
            Math.abs(msg.timestamp - timestamp) < 2000,
        );

        if (recentMessage) {
          return prev;
        }

        if (messageIdsRef.current.has(msgId)) {
          return prev;
        }

        messageIdsRef.current.add(msgId);
        if (messageIdsRef.current.size > 1000) {
          const first = Array.from(messageIdsRef.current)[0];
          if (first) {
            messageIdsRef.current.delete(first);
          }
        }

        return [
          ...prev,
          {
            id: msgId,
            userId,
            username,
            content,
            timestamp,
          },
        ];
      });
    };

    const peerJoinCallback = async (
      userId: string,
      username: string,
      fingerprint?: string,
    ) => {
      if (fingerprint) {
        const storageKey = `fingerprint:${userId}`;
        const savedFingerprint = localStorage.getItem(storageKey);

        if (savedFingerprint && savedFingerprint !== fingerprint) {
          console.warn(
            `Key fingerprint mismatch for ${username} (${userId}): expected ${savedFingerprint}, got ${fingerprint}`,
          );
        } else if (!savedFingerprint) {
          localStorage.setItem(storageKey, fingerprint);
          await SetPeerFingerprint(userId, fingerprint);
        } else {
          await SetPeerFingerprint(userId, fingerprint);
        }

        setPeerFingerprints((prev) => new Map(prev).set(userId, fingerprint));
      }

      setPeers((prev) => {
        const alreadyExists = prev.find((p) => p.userId === userId);
        if (alreadyExists) return prev;

        if (userId !== myUserId && roomLoadedRef.current) {
          const timestamp = Date.now();
          messageCounterRef.current += 1;
          const msgId = `system-join-${userId}-${timestamp}`;

          setMessages((prevMessages) => {
            const existingMessage = prevMessages.find(
              (msg) =>
                msg.isSystem &&
                msg.systemType === "join" &&
                msg.userId === userId &&
                Math.abs(msg.timestamp - timestamp) < 3000,
            );

            if (existingMessage) {
              return prevMessages;
            }

            return [
              ...prevMessages,
              {
                id: msgId,
                userId: userId,
                username: username,
                content: "chat.userJoined",
                timestamp: timestamp,
                isSystem: true,
                systemType: "join",
              },
            ];
          });
        }

        return [...prev, { userId, username }];
      });
    };

    const peerLeftCallback = (userId: string) => {
      setPeers((prev) => {
        const peer = prev.find((p) => p.userId === userId);

        if (peer && userId !== myUserId && roomLoadedRef.current) {
          const timestamp = Date.now();
          messageCounterRef.current += 1;
          const msgId = `system-leave-${userId}-${timestamp}`;

          setMessages((prevMessages) => {
            const existingMessage = prevMessages.find(
              (msg) =>
                msg.isSystem &&
                msg.systemType === "leave" &&
                msg.userId === userId &&
                Math.abs(msg.timestamp - timestamp) < 3000,
            );

            if (existingMessage) {
              return prevMessages;
            }

            return [
              ...prevMessages,
              {
                id: msgId,
                userId: userId,
                username: peer.username,
                content: "chat.userLeft",
                timestamp: timestamp,
                isSystem: true,
                systemType: "leave",
              },
            ];
          });
        }

        return prev.filter((p) => p.userId !== userId);
      });
    };

    const keyMismatchCallback = (
      userId: string,
      username: string,
      expectedFingerprint: string,
      receivedFingerprint: string,
    ) => {
      console.warn(
        `SECURITY WARNING: Key fingerprint mismatch for ${username} (${userId})`,
      );
      console.warn(`Expected: ${expectedFingerprint}`);
      console.warn(`Received: ${receivedFingerprint}`);
      alert(
        `${t("security.keyMismatch")} ${username}\n${t("security.expected")}: ${expectedFingerprint}\n${t("security.received")}: ${receivedFingerprint}`,
      );
    };

    const roomErrorCallback = (message: string) => {
      alert(t("errors.invalidPassword"));
      setConnected(false);
    };

    const myUserIdCallback = (userId: string) => {
      setMyUserId(userId);
      roomLoadedRef.current = true;
    };

    EventsOn("message", messageCallback);
    EventsOn("peerJoin", peerJoinCallback);
    EventsOn("peerLeft", peerLeftCallback);
    EventsOn("keyMismatch", keyMismatchCallback);
    EventsOn("roomError", roomErrorCallback);
    EventsOn("myUserId", myUserIdCallback);

    return () => {};
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const createNewChat = async () => {
    if (!nodeUrl || !username) return;

    const newRoomID = await GenerateRoomID();
    setRoomID(newRoomID);

    try {
      await ConnectToRoom(nodeUrl, newRoomID, username, password);
      setConnected(true);
    } catch (error) {
      console.error("Connection error:", error);
      alert(t("errors.connectionFailed"));
    }
  };

  const joinChat = async () => {
    if (!nodeUrl || !roomID || !username) return;

    try {
      await ConnectToRoom(nodeUrl, roomID, username, joinPassword);
      setConnected(true);

      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        if (key && key.startsWith("fingerprint:")) {
          const userId = key.replace("fingerprint:", "");
          const fingerprint = localStorage.getItem(key);
          if (fingerprint) {
            await SetPeerFingerprint(userId, fingerprint);
          }
        }
      }
    } catch (error) {
      console.error("Connection error:", error);
      alert(t("errors.connectionFailed"));
    }
  };

  const disconnect = async () => {
    await Disconnect();
    setConnected(false);
    setMessages([]);
    setPeers([]);
    setPeerFingerprints(new Map());
    messageIdsRef.current.clear();
    roomLoadedRef.current = false;
  };

  const onSendMessage = async () => {
    if (!message.trim() || !connected) return;

    try {
      await SendMessage(message);
      setMessage("");
    } catch (error) {
      console.error("Send error:", error);
    }
  };

  const onKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      onSendMessage();
    }
  };

  if (!connected) {
    return (
      <div className="app">
        <div className="app-header">
          <div className="language-selector">
            <button
              className={language === "en" ? "lang-btn active" : "lang-btn"}
              onClick={() => changeLanguage("en")}
            >
              EN
            </button>
            <button
              className={language === "ru" ? "lang-btn active" : "lang-btn"}
              onClick={() => changeLanguage("ru")}
            >
              RU
            </button>
          </div>
        </div>
        <div className="connection-container">
          <div className="connection-section">
            <div className="section-title">{t("connection.createNewChat")}</div>
            <div className="input-group">
              <label className="input-label">{t("connection.nodeUrl")}</label>
              <input
                type="text"
                value={nodeUrl}
                onChange={(e) => setNodeUrl(e.target.value)}
                placeholder="localhost:8080"
                className="input-field"
              />
            </div>
            <div className="input-group">
              <label className="input-label">{t("connection.username")}</label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={t("connection.usernamePlaceholder")}
                className="input-field"
                onKeyPress={(e) => e.key === "Enter" && createNewChat()}
              />
            </div>
            <div className="input-group">
              <label className="input-label">{t("connection.password")}</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("connection.passwordPlaceholder")}
                className="input-field"
                onKeyPress={(e) => e.key === "Enter" && createNewChat()}
              />
            </div>
            <button onClick={createNewChat} className="btn-primary">
              {t("connection.createNewChat")}
            </button>
          </div>

          <div className="divider">
            <div className="divider-line"></div>
            <span className="divider-text">{t("connection.or")}</span>
            <div className="divider-line"></div>
          </div>

          <div className="connection-section">
            <div className="section-title">{t("connection.joinChat")}</div>
            <div className="input-group">
              <label className="input-label">{t("connection.username")}</label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={t("connection.usernamePlaceholder")}
                className="input-field"
              />
            </div>
            <div className="input-group">
              <input
                type="text"
                value={roomID}
                onChange={(e) => setRoomID(e.target.value)}
                placeholder={t("connection.enterChatId")}
                className="input-field"
              />
            </div>
            <div className="input-group">
              <label className="input-label">{t("connection.password")}</label>
              <input
                type="password"
                value={joinPassword}
                onChange={(e) => setJoinPassword(e.target.value)}
                placeholder={t("connection.passwordPlaceholder")}
                className="input-field"
                onKeyPress={(e) => e.key === "Enter" && joinChat()}
              />
            </div>
            <button onClick={joinChat} className="btn-secondary">
              {t("connection.joinChat")}
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="app">
      <div className="chat-container">
        <div className="chat-header">
          <div className="room-info">
            <span className="room-label">{t("chat.room")}:</span>
            <span className="room-id">{roomID}</span>
            <span className="peers-count">
              {peers.length + 1} {t("chat.online")}
            </span>
            {peerFingerprints.size > 0 && (
              <div className="security-info">
                <div className="security-text">🔒 {t("chat.encrypted")}</div>
              </div>
            )}
          </div>
          <div className="header-right">
            <div className="chat-language-selector">
              <button
                className={language === "en" ? "lang-btn active" : "lang-btn"}
                onClick={() => changeLanguage("en")}
              >
                EN
              </button>
              <button
                className={language === "ru" ? "lang-btn active" : "lang-btn"}
                onClick={() => changeLanguage("ru")}
              >
                RU
              </button>
            </div>
            <button onClick={disconnect} className="disconnect-button">
              {t("connection.disconnect")}
            </button>
          </div>
        </div>
        <div className="chat-messages">
          {messages.length === 0 ? (
            <div className="empty-state">
              <div className="empty-state-text">{t("chat.noMessages")}</div>
            </div>
          ) : (
            messages.map((msg) => {
              if (msg.isSystem) {
                const systemText =
                  msg.content === "chat.userJoined"
                    ? t("chat.userJoined")
                    : msg.content === "chat.userLeft"
                      ? t("chat.userLeft")
                      : msg.content;
                return (
                  <div key={msg.id} className="message-system">
                    <span className="system-message-text">
                      {msg.username} {systemText}
                    </span>
                  </div>
                );
              }

              const isOwn = msg.userId === myUserId;
              return (
                <div
                  key={msg.id}
                  className={`message ${isOwn ? "message-own" : "message-peer"}`}
                >
                  <div className="message-header">
                    <span className="message-username">{msg.username}</span>
                    <span className="message-time">
                      {new Date(msg.timestamp).toLocaleTimeString([], {
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                  <div className="message-content">{msg.content}</div>
                </div>
              );
            })
          )}
          <div ref={messagesEndRef} />
        </div>
        <div className="chat-input-container">
          <input
            type="text"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyPress={onKeyPress}
            placeholder={t("chat.typeMessage")}
            className="chat-input"
          />
          <button onClick={onSendMessage} className="send-button">
            {t("chat.send")}
          </button>
        </div>
      </div>
    </div>
  );
}

export default App;
