package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"time"

	clawbot "github.com/importcjj/wechat-clawbot-client-go"
	"github.com/importcjj/wechat-clawbot-client-go/store"
)

const (
	dataDir    = "./data"
	listenAddr = ":9099"
	maxHistory = 200
)

// ChatMessage is a stored message for the UI history.
type ChatMessage struct {
	ID        string `json:"id"`
	BotID     string `json:"bot_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text"`
	Direction string `json:"direction"`          // "in" or "out"
	HasImage  bool   `json:"has_image"`
	ImageB64  string `json:"image_b64,omitempty"` // base64 for images
	HasFile   bool   `json:"has_file"`
	FileName  string `json:"file_name,omitempty"`
	FileB64   string `json:"file_b64,omitempty"`  // base64 for files (small files only)
	FileSize  int    `json:"file_size,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// MessageStore keeps chat history per bot, persisted to JSON files.
type MessageStore struct {
	mu       sync.RWMutex
	messages map[string][]ChatMessage // botID -> messages
	dir      string
}

func NewMessageStore(dir string) *MessageStore {
	os.MkdirAll(dir, 0o755)
	ms := &MessageStore{
		messages: make(map[string][]ChatMessage),
		dir:      dir,
	}
	ms.loadAll()
	return ms
}

func (ms *MessageStore) filePath(botID string) string {
	return ms.dir + "/" + botID + ".messages.json"
}

func (ms *MessageStore) loadAll() {
	entries, err := os.ReadDir(ms.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		const suffix = ".messages.json"
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			botID := name[:len(name)-len(suffix)]
			data, err := os.ReadFile(ms.dir + "/" + name)
			if err != nil {
				continue
			}
			var msgs []ChatMessage
			if json.Unmarshal(data, &msgs) == nil {
				ms.messages[botID] = msgs
			}
		}
	}
}

func (ms *MessageStore) persist(botID string) {
	msgs := ms.messages[botID]
	data, _ := json.Marshal(msgs)
	os.WriteFile(ms.filePath(botID), data, 0o644)
}

func (ms *MessageStore) Add(msg ChatMessage) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	msgs := ms.messages[msg.BotID]
	if len(msgs) >= maxHistory {
		msgs = msgs[len(msgs)-maxHistory+1:]
	}
	ms.messages[msg.BotID] = append(msgs, msg)
	ms.persist(msg.BotID)
}

func (ms *MessageStore) List(botID string) []ChatMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	msgs := ms.messages[botID]
	if msgs == nil {
		return []ChatMessage{}
	}
	return msgs
}


// BotManager manages multiple WeChat bot clients.
type BotManager struct {
	mu       sync.RWMutex
	bots     map[string]*clawbot.DefaultClient
	store    store.Store
	msgStore *MessageStore
}

func NewBotManager(s store.Store, ms *MessageStore) *BotManager {
	return &BotManager{
		bots:     make(map[string]*clawbot.DefaultClient),
		store:    s,
		msgStore: ms,
	}
}

func (m *BotManager) getOrCreate(clientID string) *clawbot.DefaultClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.bots[clientID]; ok {
		return c
	}

	client := clawbot.NewDefault(clientID, m.store,
		clawbot.WithDefaultEventHooks(clawbot.DefaultEventHooks{
			OnMessage: func(id string, msg *clawbot.Message) {
				slog.Info("message", "bot", id, "from", msg.From, "text", msg.Text,
					"images", len(msg.Images), "files", len(msg.Files))

				cm := ChatMessage{
					ID:        fmt.Sprintf("%d-%s", msg.MessageID, msg.From),
					BotID:     id,
					From:      msg.From,
					Direction: "in",
					Text:      msg.Text,
					Timestamp: msg.CreatedAt.UnixMilli(),
				}
				if len(msg.Images) > 0 {
					cm.HasImage = true
					cm.ImageB64 = base64.StdEncoding.EncodeToString(msg.Images[0].Data)
				}
				if len(msg.Files) > 0 {
					cm.HasFile = true
					cm.FileName = msg.Files[0].Filename
					cm.FileSize = len(msg.Files[0].Data)
				}
				if len(msg.Videos) > 0 {
					cm.HasFile = true
					cm.FileName = "video.mp4"
					cm.FileSize = len(msg.Videos[0].Data)
				}
				m.msgStore.Add(cm)
			},
			OnConnected: func(id string) {
				slog.Info("bot connected", "id", id)
			},
			OnSessionExpired: func(id string) {
				slog.Warn("bot session expired", "id", id)
			},
			OnDisconnected: func(id string, err error) {
				slog.Info("bot disconnected", "id", id, "error", err)
			},
		}),
	)

	m.bots[clientID] = client
	return client
}

func (m *BotManager) remove(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.bots[clientID]; ok {
		c.Stop()
		delete(m.bots, clientID)
	}
}

func (m *BotManager) list() []BotInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]BotInfo, 0, len(m.bots))
	for id, c := range m.bots {
		info := BotInfo{
			ClientID: id,
			State:    c.State().String(),
		}
		if creds, err := m.store.LoadCredentials(context.Background(), id); err == nil {
			info.UserID = creds.UserID
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ClientID < infos[j].ClientID
	})
	return infos
}

func (m *BotManager) get(clientID string) (*clawbot.DefaultClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.bots[clientID]
	return c, ok
}

// --- Bot registry: persists the list of activated bot IDs ---

const registryFile = "bots.json"

func (m *BotManager) registryPath() string {
	return dataDir + "/" + registryFile
}

func (m *BotManager) saveRegistry() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.bots))
	for id := range m.bots {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	data, _ := json.Marshal(ids)
	os.WriteFile(m.registryPath(), data, 0o644)
}

func (m *BotManager) loadRegistry() []string {
	data, err := os.ReadFile(m.registryPath())
	if err != nil {
		return nil
	}
	var ids []string
	json.Unmarshal(data, &ids)
	return ids
}

// RestoreAll reads the registry and starts all previously activated bots.
func (m *BotManager) RestoreAll(ctx context.Context) {
	ids := m.loadRegistry()
	for _, id := range ids {
		client := m.getOrCreate(id)
		if !client.HasCredentials() {
			slog.Warn("skip restore, no credentials", "id", id)
			continue
		}
		slog.Info("restoring bot", "id", id)
		go func(c *clawbot.DefaultClient, cid string) {
			if err := c.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("restore failed", "id", cid, "error", err)
			}
		}(client, id)
	}
	if len(ids) > 0 {
		slog.Info("restored bots", "count", len(ids))
	}
}

type BotInfo struct {
	ClientID string `json:"client_id"`
	State    string `json:"state"`
	UserID   string `json:"user_id,omitempty"` // the WeChat user this bot serves
}

type ActivateResponse struct {
	State string `json:"state"`
	QRURL string `json:"qr_url,omitempty"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fileStore := store.NewFileStore(dataDir)
	msgStore := NewMessageStore(dataDir)
	mgr := NewBotManager(fileStore, msgStore)

	// Restore previously activated bots
	mgr.RestoreAll(ctx)

	mux := http.NewServeMux()
	handler := cors(mux)

	// GET /api/bots
	mux.HandleFunc("GET /api/bots", func(w http.ResponseWriter, r *http.Request) {
		bots := mgr.list()
		if bots == nil {
			bots = []BotInfo{}
		}
		writeJSON(w, bots)
	})

	// POST /api/bots/{id}/activate
	mux.HandleFunc("POST /api/bots/{id}/activate", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("id")
		if clientID == "" {
			http.Error(w, "missing client id", 400)
			return
		}

		client := mgr.getOrCreate(clientID)
		mgr.saveRegistry()

		if client.State() == clawbot.StateRunning {
			writeJSON(w, ActivateResponse{State: "running"})
			return
		}

		if client.HasCredentials() {
			go client.Start(ctx)
			writeJSON(w, ActivateResponse{State: "running"})
			return
		}

		session, err := client.Login(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		writeJSON(w, ActivateResponse{State: "need_login", QRURL: session.QRCodeURL()})

		go func() {
			if err := session.Wait(ctx); err != nil {
				slog.Error("login wait failed", "id", clientID, "error", err)
				return
			}
			if err := client.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("start failed", "id", clientID, "error", err)
			}
		}()
	})

	// GET /api/bots/{id}/state
	mux.HandleFunc("GET /api/bots/{id}/state", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("id")
		client, ok := mgr.get(clientID)
		if !ok {
			writeJSON(w, map[string]string{"state": "not_found"})
			return
		}
		writeJSON(w, map[string]string{"client_id": clientID, "state": client.State().String()})
	})

	// POST /api/bots/{id}/deactivate
	mux.HandleFunc("POST /api/bots/{id}/deactivate", func(w http.ResponseWriter, r *http.Request) {
		mgr.remove(r.PathValue("id"))
		mgr.saveRegistry()
		writeJSON(w, map[string]string{"state": "stopped"})
	})



	// GET /api/bots/{id}/messages?user={userID}
	mux.HandleFunc("GET /api/bots/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		botID := r.PathValue("id")
		userFilter := r.URL.Query().Get("user")
		all := msgStore.List(botID)
		if userFilter == "" {
			writeJSON(w, all)
			return
		}
		var filtered []ChatMessage
		for _, m := range all {
			if m.From == userFilter || m.To == userFilter {
				filtered = append(filtered, m)
			}
		}
		if filtered == nil {
			filtered = []ChatMessage{}
		}
		writeJSON(w, filtered)
	})

	// POST /api/bots/{id}/send — send text message
	mux.HandleFunc("POST /api/bots/{id}/send", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("id")
		client, ok := mgr.get(clientID)
		if !ok {
			http.Error(w, "bot not found", 404)
			return
		}

		var body struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", 400)
			return
		}

		if err := client.SendText(r.Context(), body.To, body.Text); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		msgStore.Add(ChatMessage{
			ID:        fmt.Sprintf("out-%d", time.Now().UnixNano()),
			BotID:     clientID,
			To:        body.To,
			Text:      body.Text,
			Direction: "out",
			Timestamp: time.Now().UnixMilli(),
		})
		writeJSON(w, map[string]string{"status": "sent"})
	})

	// POST /api/bots/{id}/send-image — send image (base64)
	mux.HandleFunc("POST /api/bots/{id}/send-image", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("id")
		client, ok := mgr.get(clientID)
		if !ok {
			http.Error(w, "bot not found", 404)
			return
		}

		var body struct {
			To      string `json:"to"`
			ImageB64 string `json:"image_b64"`
			Caption string `json:"caption"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", 400)
			return
		}

		imgData, err := decodeBase64(body.ImageB64)
		if err != nil {
			http.Error(w, "invalid base64 image", 400)
			return
		}

		if err := client.SendImage(r.Context(), body.To, imgData, body.Caption); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		msgStore.Add(ChatMessage{
			ID:        fmt.Sprintf("out-%d", time.Now().UnixNano()),
			BotID:     clientID,
			To:        body.To,
			Text:      body.Caption,
			Direction: "out",
			HasImage:  true,
			ImageB64:  body.ImageB64,
			Timestamp: time.Now().UnixMilli(),
		})
		writeJSON(w, map[string]string{"status": "sent"})
	})

	// POST /api/bots/{id}/send-file — send file (base64)
	mux.HandleFunc("POST /api/bots/{id}/send-file", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("id")
		client, ok := mgr.get(clientID)
		if !ok {
			http.Error(w, "bot not found", 404)
			return
		}

		var body struct {
			To       string `json:"to"`
			FileB64  string `json:"file_b64"`
			FileName string `json:"file_name"`
			Caption  string `json:"caption"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", 400)
			return
		}

		fileData, err := decodeBase64(body.FileB64)
		if err != nil {
			http.Error(w, "invalid base64 file: "+err.Error(), 400)
			return
		}

		if err := client.SendFile(r.Context(), body.To, fileData, body.FileName, body.Caption); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		msgStore.Add(ChatMessage{
			ID:        fmt.Sprintf("out-%d", time.Now().UnixNano()),
			BotID:     clientID,
			To:        body.To,
			Text:      body.Caption,
			Direction: "out",
			HasFile:   true,
			FileName:  body.FileName,
			FileSize:  len(fileData),
			Timestamp: time.Now().UnixMilli(),
		})
		writeJSON(w, map[string]string{"status": "sent"})
	})

	// POST /api/bots/{id}/typing — send/cancel typing indicator
	mux.HandleFunc("POST /api/bots/{id}/typing", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.PathValue("id")
		client, ok := mgr.get(clientID)
		if !ok {
			http.Error(w, "bot not found", 404)
			return
		}

		var body struct {
			To     string `json:"to"`
			Cancel bool   `json:"cancel"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", 400)
			return
		}

		var err error
		if body.Cancel {
			err = client.CancelTyping(r.Context(), body.To)
		} else {
			err = client.SendTyping(r.Context(), body.To)
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})

	srv := &http.Server{Addr: listenAddr, Handler: handler}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Server listening on %s\n", listenAddr)
	fmt.Printf("  API:  http://localhost%s/api/bots\n", listenAddr)
	fmt.Printf("  UI:   cd ui && pnpm dev\n")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// decodeBase64 handles both standard and raw (no padding) base64.
func decodeBase64(s string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
