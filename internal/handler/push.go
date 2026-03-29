package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"firebase.google.com/go/v4/messaging"
)

// PushManager gerencia tokens FCM dos dispositivos por partida
// e envia push notifications quando eventos SSE acontecem.
type PushManager struct {
	mu     sync.RWMutex
	client *messaging.Client
	// matchID → set de tokens FCM
	tokens map[string]map[string]bool
}

// NewPushManager cria um novo gerenciador de push notifications.
// Se client for nil, as notificações serão silenciosamente ignoradas (fallback gracioso).
func NewPushManager(client *messaging.Client) *PushManager {
	return &PushManager{
		client: client,
		tokens: make(map[string]map[string]bool),
	}
}

// RegisterRequest é o payload esperado pelo endpoint /api/push/register
type RegisterRequest struct {
	Token   string `json:"token"`
	MatchID string `json:"match_id"`
}

// RegisterHandler registra um token FCM para receber notificações de uma partida.
func (pm *PushManager) RegisterHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if req.Token == "" || req.MatchID == "" {
			http.Error(w, "token and match_id required", http.StatusBadRequest)
			return
		}

		pm.mu.Lock()
		if pm.tokens[req.MatchID] == nil {
			pm.tokens[req.MatchID] = make(map[string]bool)
		}
		pm.tokens[req.MatchID][req.Token] = true
		count := len(pm.tokens[req.MatchID])
		pm.mu.Unlock()

		log.Printf("FCM: registered token for match %s (total: %d devices)", req.MatchID, count)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// UnregisterMatch remove todos os tokens registrados para uma partida.
// Chamado quando a partida termina ou é cancelada (cleanup).
func (pm *PushManager) UnregisterMatch(matchID string) {
	pm.mu.Lock()
	delete(pm.tokens, matchID)
	pm.mu.Unlock()
	log.Printf("FCM: cleaned up tokens for match %s", matchID)
}

// PushEvent representa uma notificação a ser enviada.
type PushEvent struct {
	MatchID   string
	EventType string
	Title     string
	Body      string
}

// eventMessages mapeia eventos SSE para mensagens de push amigáveis (pt-BR).
var eventMessages = map[string]struct {
	Title string
	Body  string
}{
	"match_started":       {Title: "🎲 Partida Começou!", Body: "A partida está ativa. Boa sorte!"},
	"round_started":       {Title: "🃏 Nova Rodada", Body: "Uma nova rodada começou."},
	"round_winner_set":    {Title: "🏆 Rodada Encerrada", Body: "O vencedor da rodada foi definido."},
	"points_updated":      {Title: "📊 Placar Atualizado", Body: "Os pontos foram atualizados."},
	"player_estourou":     {Title: "💥 Estourou!", Body: "Um jogador passou de 51 pontos."},
	"match_finished":      {Title: "🎯 Partida Finalizada!", Body: "A partida acabou. Confira o ranking!"},
	"match_cancelled":     {Title: "❌ Partida Cancelada", Body: "A partida foi cancelada pelo organizador."},
	"player_joined":       {Title: "👋 Novo Jogador", Body: "Um novo jogador entrou na partida!"},
	"nickname_updated":    {Title: "🏷️ Apelido", Body: "Uma votação de apelido foi atualizada."},
	"table_image_updated": {Title: "📸 Foto da Mesa", Body: "Uma foto da mesa foi enviada."},
	"tournament_started":  {Title: "🏆 Torneio Começou!", Body: "O torneio está ativo! Confira sua mesa."},
}

// NotifyMatch envia push notification para todos os dispositivos registrados em uma partida.
// O evento SSE é convertido em mensagem amigável automaticamente.
// Esta função é non-blocking — dispara uma goroutine.
func (pm *PushManager) NotifyMatch(matchID, sseEvent string) {
	if pm.client == nil {
		return
	}

	// Extrai o tipo base do evento (e.g., "round_started:abc123" → "round_started")
	eventType := sseEvent
	for i, c := range sseEvent {
		if c == ':' {
			eventType = sseEvent[:i]
			break
		}
	}

	msg, ok := eventMessages[eventType]
	if !ok {
		// Evento desconhecido — não envia push
		return
	}

	pm.mu.RLock()
	tokenSet := pm.tokens[matchID]
	if len(tokenSet) == 0 {
		pm.mu.RUnlock()
		return
	}
	tokens := make([]string, 0, len(tokenSet))
	for t := range tokenSet {
		tokens = append(tokens, t)
	}
	pm.mu.RUnlock()

	go pm.sendToTokens(matchID, eventType, msg.Title, msg.Body, tokens)
}

func (pm *PushManager) sendToTokens(matchID, eventType, title, body string, tokens []string) {
	ctx := context.Background()

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Data: map[string]string{
			"event":    eventType,
			"match_id": matchID,
			"title":    title,
			"body":     body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Title:        title,
				Body:         body,
				ChannelID:    "domino_match_events",
				DefaultSound: true,
			},
		},
	}

	resp, err := pm.client.SendEachForMulticast(ctx, message)
	if err != nil {
		log.Printf("FCM: error sending to match %s: %v", matchID, err)
		return
	}

	log.Printf("FCM: match %s — sent %d, success %d, failure %d",
		matchID, len(tokens), resp.SuccessCount, resp.FailureCount)

	// Remove tokens inválidos
	if resp.FailureCount > 0 {
		pm.mu.Lock()
		for i, sendResp := range resp.Responses {
			if sendResp.Error != nil {
				delete(pm.tokens[matchID], tokens[i])
				log.Printf("FCM: removed invalid token for match %s: %v", matchID, sendResp.Error)
			}
		}
		pm.mu.Unlock()
	}
}
