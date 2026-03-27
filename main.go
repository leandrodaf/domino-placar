package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/handler"
)

func main() {
	// Ensure uploads directory exists
	if err := os.MkdirAll("uploads", 0750); err != nil {
		log.Fatalf("Failed to create uploads dir: %v", err)
	}

	// Initialize store (Firebase se FIREBASE_DATABASE_URL estiver definido, caso contrário SQLite)
	var store db.Store

	if fbURL := os.Getenv("FIREBASE_DATABASE_URL"); fbURL != "" {
		fbCreds := os.Getenv("FIREBASE_CREDENTIALS")
		var err error
		store, err = db.NewFirebaseStore(fbURL, fbCreds)
		if err != nil {
			log.Fatalf("Failed to init Firebase: %v", err)
		}
		log.Println("Using Firebase Realtime Database")
	} else {
		sqlDB, err := db.OpenDB("domino.db")
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer sqlDB.Close()

		if err := db.CreateTables(sqlDB); err != nil {
			log.Fatalf("Failed to create tables: %v", err)
		}
		store = db.NewSQLiteStore(sqlDB)
		log.Println("Using SQLite database")
	}

	// Initialize SSE hub
	hub := handler.NewSSEHub()

	// Initialize templates
	tmpl, err := handler.NewTemplates()
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Limpeza periódica do rate limiter (a cada 10 min)
	go func() {
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for range t.C {
			handler.CleanRateMap()
		}
	}()

	// Set up routes
	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Home
	mux.HandleFunc("GET /", handler.HomeHandler(tmpl))

	// Match creation
	mux.HandleFunc("POST /match", handler.CreateMatchHandler(store))

	// Retomar partida pelo ID — redireciona para o estado atual
	mux.HandleFunc("GET /match/{id}", handler.MatchResumeHandler(store))

	// Lobby
	mux.HandleFunc("GET /match/{id}/lobby", handler.LobbyHandler(store, tmpl))
	mux.HandleFunc("GET /match/{id}/qrcode", handler.QRCodeHandler(store))
	mux.HandleFunc("GET /match/{id}/players-partial", handler.PlayersPartialHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/start", handler.StartMatchHandler(store, hub))

	// Join
	mux.HandleFunc("GET /match/{id}/join", handler.JoinPageHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/join", handler.JoinHandler(store, hub))
	mux.HandleFunc("GET /match/{id}/waiting", handler.WaitingHandler(store, tmpl))

	// Round management
	mux.HandleFunc("POST /match/{id}/round", handler.CreateRoundHandler(store, hub))
	mux.HandleFunc("POST /match/{id}/round/{rid}/winner/{pid}", handler.SetRoundWinnerHandler(store, hub))
	mux.HandleFunc("POST /match/{id}/round/{rid}/starter/{pid}", handler.SetRoundStarterHandler(store, hub))
	mux.HandleFunc("GET /match/{id}/round/{rid}/round-scores", handler.RoundScoresPageHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/round/{rid}/round-scores", handler.BulkScoreHandler(store, hub))

	// Game page
	mux.HandleFunc("GET /match/{id}/round/{rid}/game", handler.GameHandler(store, tmpl))

	// Upload
	mux.HandleFunc("GET /match/{id}/round/{rid}/upload/{pid}", handler.UploadPageHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/round/{rid}/upload/{pid}", handler.UploadHandler(store, hub, tmpl))

	// Confirm
	mux.HandleFunc("GET /match/{id}/round/{rid}/confirm/{pid}", handler.ConfirmPageHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/round/{rid}/confirm/{pid}", handler.ConfirmHandler(store, hub))

	// Ranking
	mux.HandleFunc("GET /match/{id}/round/{rid}/ranking", handler.RankingHandler(store, tmpl))
	mux.HandleFunc("GET /match/{id}/ranking", handler.RankingHandler(store, tmpl))

	// Finish / cancel match
	mux.HandleFunc("POST /match/{id}/finish", handler.FinishMatchHandler(store, hub))
	mux.HandleFunc("POST /match/{id}/cancel", handler.CancelMatchHandler(store, hub))

	// Foto da mesa
	mux.HandleFunc("POST /match/{id}/round/{rid}/table-image", handler.TableImageHandler(store, hub))

	// Correção manual de pontuação
	mux.HandleFunc("POST /match/{id}/player/{pid}/score", handler.ManualScoreHandler(store, hub))

	// Ranking global
	mux.HandleFunc("GET /global-ranking", handler.GlobalRankingHandler(store, tmpl))

	// Tournament
	mux.HandleFunc("POST /tournament", handler.CreateTournamentHandler(store))
	mux.HandleFunc("GET /tournament/{id}/lobby", handler.TournamentLobbyHandler(store, tmpl))
	mux.HandleFunc("GET /tournament/{id}/qrcode", handler.TournamentQRCodeHandler(store))
	mux.HandleFunc("GET /tournament/{id}/players-partial", handler.TournamentPlayersPartialHandler(store, tmpl))
	mux.HandleFunc("POST /tournament/{id}/start", handler.StartTournamentHandler(store, hub))
	mux.HandleFunc("GET /tournament/{id}/join", handler.TournamentJoinPageHandler(store, tmpl))
	mux.HandleFunc("POST /tournament/{id}/join", handler.JoinTournamentHandler(store, hub))
	mux.HandleFunc("GET /tournament/{id}/waiting", handler.TournamentWaitingHandler(store, tmpl))
	mux.HandleFunc("GET /tournament/{id}/tables", handler.TournamentTablesHandler(store, tmpl))
	mux.HandleFunc("GET /tournament/{id}/ranking", handler.TournamentRankingHandler(store, tmpl))
	mux.HandleFunc("GET /tournament/{id}/events", handler.SSEHandler(hub))

	// Nicknames / apelidos
	mux.HandleFunc("GET /match/{id}/nicknames", handler.NicknamePageHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/player/{pid}/nominate", handler.NominateHandler(store, hub))
	mux.HandleFunc("POST /match/{id}/nickname/{nid}/vote", handler.VoteNicknameHandler(store, hub))

	// SSE
	mux.HandleFunc("GET /match/{id}/events", handler.SSEHandler(hub))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("Domino scorekeeping server starting on %s", addr)

	// Aplica middleware de segurança em todas as rotas
	if err := http.ListenAndServe(addr, handler.SecurityHeaders(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
