package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/handler"
	"github.com/leandrodaf/domino-placar/internal/i18n"
)

// Build-time variables (set via ldflags by GoReleaser).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.Printf("domino-placar %s (commit=%s built=%s)", version, commit, date)
	// Ensure uploads directory exists
	if err := os.MkdirAll("uploads", 0750); err != nil {
		log.Fatalf("Failed to create uploads dir: %v", err)
	}

	// Initialize store (Firebase if FIREBASE_DATABASE_URL is set, otherwise SQLite)
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
		defer func() { _ = sqlDB.Close() }()

		if err := db.CreateTables(sqlDB); err != nil {
			log.Fatalf("Failed to create tables: %v", err)
		}
		store = db.NewSQLiteStore(sqlDB)
		log.Println("Using SQLite database")
	}

	// Initialize i18n
	i18n.Init()

	// Initialize SSE hub
	hub := handler.NewSSEHub()

	// Initialize Push Notification Manager (FCM)
	pushMgr := initPushManager()
	hub.SetPushManager(pushMgr)

	// Initialize templates
	tmpl, err := handler.NewTemplates()
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Periodic rate limiter cleanup (every 10 min)
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

	// Android App Links — verificação de domínio para deep links
	// O SHA256 do certificado deve ser atualizado com o fingerprint real da keystore de release.
	// Gere com: keytool -list -v -keystore release.jks -alias key0 | grep SHA256
	mux.HandleFunc("GET /.well-known/assetlinks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
  "relation": ["delegate_permission/common.handle_all_urls"],
  "target": {
    "namespace": "android_app",
    "package_name": "net.dominoplacar.app",
    "sha256_cert_fingerprints": ["TODO:SUBSTITUIR_COM_SHA256_DA_KEYSTORE_DE_RELEASE"]
  }
}]`))
	})

	// SEO — robots.txt e sitemap.xml
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		host := "https://dominoplacar.net"
		if h := r.Host; h != "" {
			scheme := "https"
			if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
				scheme = "http"
			}
			host = scheme + "://" + h
		}
		_, _ = w.Write([]byte("User-agent: *\n" +
			"Allow: /\n" +
			"Allow: /global-ranking\n" +
			"Allow: /privacy\n" +
			"Allow: /data-deletion\n" +
			"Allow: /static/\n" +
			"Disallow: /match/\n" +
			"Disallow: /tournament/\n" +
			"Disallow: /turma/\n" +
			"Disallow: /api/\n" +
			"Disallow: /uploads/\n\n" +
			"Sitemap: " + host + "/sitemap.xml\n"))
	})

	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		host := "https://dominoplacar.net"
		if h := r.Host; h != "" {
			scheme := "https"
			if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
				scheme = "http"
			}
			host = scheme + "://" + h
		}
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>` + host + `/</loc>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="` + host + `/?lang=pt"/>
    <xhtml:link rel="alternate" hreflang="en"    href="` + host + `/?lang=en"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="` + host + `/"/>
  </url>
  <url>
    <loc>` + host + `/global-ranking</loc>
    <changefreq>hourly</changefreq>
    <priority>0.9</priority>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="` + host + `/global-ranking?lang=pt"/>
    <xhtml:link rel="alternate" hreflang="en"    href="` + host + `/global-ranking?lang=en"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="` + host + `/global-ranking"/>
  </url>
  <url>
    <loc>` + host + `/privacy</loc>
    <changefreq>yearly</changefreq>
    <priority>0.3</priority>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="` + host + `/privacy?lang=pt"/>
    <xhtml:link rel="alternate" hreflang="en"    href="` + host + `/privacy?lang=en"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="` + host + `/privacy"/>
  </url>
  <url>
    <loc>` + host + `/data-deletion</loc>
    <changefreq>yearly</changefreq>
    <priority>0.2</priority>
    <xhtml:link rel="alternate" hreflang="pt-BR" href="` + host + `/data-deletion?lang=pt"/>
    <xhtml:link rel="alternate" hreflang="en"    href="` + host + `/data-deletion?lang=en"/>
    <xhtml:link rel="alternate" hreflang="x-default" href="` + host + `/data-deletion"/>
  </url>
</urlset>`))
	})

	// Home
	mux.HandleFunc("GET /", handler.HomeHandler(tmpl))

	// Privacy Policy
	mux.HandleFunc("GET /privacy", handler.PrivacyHandler(tmpl))
	mux.HandleFunc("GET /data-deletion", handler.DataDeletionPageHandler(tmpl))
	mux.HandleFunc("POST /data-deletion", handler.DataDeletionSubmitHandler())

	// Match creation
	mux.HandleFunc("POST /match", handler.CreateMatchHandler(store))

	// Resume match by ID — redirects to current state
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

	// Table photo
	mux.HandleFunc("POST /match/{id}/round/{rid}/table-image", handler.TableImageHandler(store, hub))

	// Manual score correction
	mux.HandleFunc("POST /match/{id}/player/{pid}/score", handler.ManualScoreHandler(store, hub))

	// Ranking global
	mux.HandleFunc("GET /global-ranking", handler.GlobalRankingHandler(store, tmpl))

	// Turma (grupos privados)
	mux.HandleFunc("GET /turma/new", handler.CreateTurmaPageHandler(tmpl))
	mux.HandleFunc("POST /turma", handler.CreateTurmaHandler(store))
	mux.HandleFunc("GET /turma/join", handler.JoinByCodeHandler(store, tmpl))
	mux.HandleFunc("GET /turma/my", handler.TurmasByMemberHandler(store))
	mux.HandleFunc("GET /turma/{id}", handler.TurmaDashboardHandler(store, tmpl))
	mux.HandleFunc("GET /turma/{id}/join", handler.JoinTurmaPageHandler(store, tmpl))
	mux.HandleFunc("POST /turma/{id}/join", handler.JoinTurmaHandler(store, hub))
	mux.HandleFunc("GET /turma/{id}/ranking", handler.TurmaRankingHandler(store, tmpl))
	mux.HandleFunc("GET /turma/{id}/qrcode", handler.TurmaQRCodeHandler(store))
	mux.HandleFunc("POST /turma/{id}/match", handler.CreateMatchInTurmaHandler(store, hub))
	mux.HandleFunc("POST /turma/{id}/match/{mid}/delete", handler.DeleteMatchFromTurmaHandler(store, hub))
	mux.HandleFunc("POST /turma/{id}/remove-member/{uid}", handler.RemoveTurmaMemberHandler(store, hub))
	mux.HandleFunc("GET /turma/{id}/events", handler.TurmaSSEHandler(hub))
	mux.HandleFunc("GET /turma/{id}/online", handler.TurmaOnlineHandler(hub))

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

	// Nicknames
	mux.HandleFunc("GET /match/{id}/nicknames", handler.NicknamePageHandler(store, tmpl))
	mux.HandleFunc("POST /match/{id}/player/{pid}/nominate", handler.NominateHandler(store, hub))
	mux.HandleFunc("POST /match/{id}/nickname/{nid}/vote", handler.VoteNicknameHandler(store, hub))

	// SSE
	mux.HandleFunc("GET /match/{id}/events", handler.SSEHandler(hub))

	// Push notifications (FCM token registration from Android app)
	mux.HandleFunc("POST /api/push/register", pushMgr.RegisterHandler())

	// Online Domino Game — /game/* routes
	if sqlStore, ok := store.(*db.SQLiteStore); ok {
		gameMgr := handler.NewGameSessionManager(db.NewGameSQLiteStore(sqlStore))
		mux.HandleFunc("GET /game/new", handler.CreateGamePageHandler(tmpl))
		mux.HandleFunc("POST /game", handler.CreateGameHandler(gameMgr, hub, tmpl))
		mux.HandleFunc("POST /game/quickplay", handler.QuickPlayHandler(gameMgr, hub))
		mux.HandleFunc("GET /game/{id}/lobby", handler.GameLobbyHandler(gameMgr, tmpl))
		mux.HandleFunc("POST /game/{id}/join", handler.GameJoinHandler(gameMgr, hub))
		mux.HandleFunc("GET /game/{id}/join", handler.GameJoinPageHandler(gameMgr, tmpl))
		mux.HandleFunc("POST /game/{id}/start", handler.GameStartHandler(gameMgr, hub))
		mux.HandleFunc("GET /game/{id}/play", handler.GamePlayHandler(gameMgr, tmpl))
		mux.HandleFunc("GET /game/{id}/state", handler.GameStateHandler(gameMgr))
		mux.HandleFunc("POST /game/{id}/action", handler.GameActionHandler(gameMgr, hub))
		mux.HandleFunc("GET /game/{id}/events", handler.GameSSEHandler(hub))
	} else {
		log.Println("Online game requires SQLite — /game/* routes not registered (Firebase mode)")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("Domino scorekeeping server starting on %s", addr)

	// Apply security middleware to all routes
	if err := http.ListenAndServe(addr, handler.SecurityHeaders(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// initPushManager cria o FCM messaging client usando as mesmas credenciais Firebase.
// Se não houver credenciais disponíveis, retorna um PushManager "noop" (sem envio).
func initPushManager() *handler.PushManager {
	fbCreds := os.Getenv("FIREBASE_CREDENTIALS")
	if fbCreds == "" {
		log.Println("FCM: no credentials, push notifications disabled")
		return handler.NewPushManager(nil)
	}

	ctx := context.Background()
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(fbCreds)))
	if err != nil {
		log.Printf("FCM: failed to init Firebase app: %v", err)
		return handler.NewPushManager(nil)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("FCM: failed to init messaging client: %v", err)
		return handler.NewPushManager(nil)
	}

	log.Println("FCM: push notifications enabled")
	return handler.NewPushManager(msgClient)
}
