package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"driftbottle/internal/auth"
	"driftbottle/internal/store"
)

// WebSocket 心跳与超时参数
const (
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = 25 * time.Second
	wsMaxMessageSize = 1024
	wsSendBufferSize = 16
)

// upgrader WebSocket 升级器；同源访问，无需跨域校验
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// wsClient 单个 WebSocket 连接
type wsClient struct {
	userID string
	conn   *websocket.Conn
	send   chan []byte
}

// wsHub 连接管理器：按用户 ID 维护其全部活跃连接
type wsHub struct {
	mu      sync.RWMutex
	clients map[string]map[*wsClient]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{clients: make(map[string]map[*wsClient]struct{})}
}

func (h *wsHub) add(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.userID]
	if !ok {
		set = make(map[*wsClient]struct{})
		h.clients[c.userID] = set
	}
	set[c] = struct{}{}
}

func (h *wsHub) remove(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.userID]
	if !ok {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.clients, c.userID)
	}
}

// isOnline 用户当前是否存在活跃连接（供清理任务判断，在线账户跳过清理）
func (h *wsHub) isOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

// push 向指定用户推送事件；无活跃连接时静默丢弃。
func (h *wsHub) push(userID, eventType string, payload any) {
	body, err := json.Marshal(map[string]any{"type": eventType, "data": payload})
	if err != nil {
		log.Printf("序列化推送事件失败: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		select {
		case c.send <- body:
		default:
			// 发送缓冲区已满，丢弃本次推送，避免阻塞
		}
	}
}

// 服务端 → 客户端推送事件
const (
	wsEventMessage       = "message"
	wsEventBottleUpdate  = "bottle_update"
	wsEventBottleDeleted = "bottle_deleted"
)

// wsPeer 推送载荷中的"对方"信息
type wsPeer struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Gender   int    `json:"gender"`
	Age      int    `json:"age"`
}

type wsMessagePayload struct {
	BottleID int64           `json:"bottleId"`
	Message  messageResponse `json:"message"`
}

type wsBottleUpdatePayload struct {
	BottleID int64   `json:"bottleId"`
	State    int     `json:"state"`
	Peer     *wsPeer `json:"peer"`
}

type wsBottleDeletedPayload struct {
	BottleID int64 `json:"bottleId"`
}

// peerFrom 由用户构造推送中的对方信息
func peerFrom(u *store.User) *wsPeer {
	p := &wsPeer{ID: u.ID, Nickname: u.Nickname, Gender: u.Gender, Age: u.Age}
	if u.Avatar != "" {
		p.Avatar = "/" + u.Avatar
	}
	return p
}

// pushBottleUpdate 向 recipient 推送瓶子状态变更；peer 为 recipient 视角的对方（可为 nil）。
func (s *Server) pushBottleUpdate(recipient string, bottleID int64, state int, peer *wsPeer) {
	if recipient == "" {
		return
	}
	s.ws.push(recipient, wsEventBottleUpdate, wsBottleUpdatePayload{BottleID: bottleID, State: state, Peer: peer})
}

// handleWS WebSocket 连接：以查询参数 token 鉴权，失败则拒绝握手。
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.ParseToken(s.cfg.JWTSecret, r.URL.Query().Get("token"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "未登录或登录已过期")
		return
	}
	u, err := s.store.GetUserByID(r.Context(), claims.Subject)
	if err != nil || u.TokenVersion != claims.TokenVersion {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "未登录或登录已过期")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade 失败时已写出响应
	}

	client := &wsClient{userID: u.ID, conn: conn, send: make(chan []byte, wsSendBufferSize)}
	s.ws.add(client)
	go s.writePump(client)
	s.readPump(client)
}

// readPump 读取客户端消息（仅心跳 ping）并在连接关闭时注销。
func (s *Server) readPump(c *wsClient) {
	defer func() {
		s.ws.remove(c)
		close(c.send)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(wsMaxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.Type == "ping" {
			c.trySend(map[string]any{"type": "pong", "data": map[string]any{}})
		}
	}
}

// writePump 定时发送 ping 心跳并写出待推送事件。
func (s *Server) writePump(c *wsClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case body, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, body); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// trySend 非阻塞写入发送缓冲区
func (c *wsClient) trySend(v any) {
	body, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- body:
	default:
	}
}
