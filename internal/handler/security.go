package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ─── App Secret ──────────────────────────────────────────────────────────────

var appSecret = initSecret()

func initSecret() []byte {
	if s := os.Getenv("SESSION_SECRET"); s != "" {
		h := sha256.Sum256([]byte(s))
		return h[:]
	}
	// Gera segredo aleatório se SESSION_SECRET não estiver definido.
	// Sessões expiram ao reiniciar o servidor — defina SESSION_SECRET em produção.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("falha ao gerar segredo da aplicação: " + err.Error())
	}
	return b
}

func macSign(data string) string {
	mac := hmac.New(sha256.New, appSecret)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func macEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// ─── Host Cookie ─────────────────────────────────────────────────────────────

// SetHostCookie grava o cookie de autenticação do anfitrião para uma partida ou torneio.
func SetHostCookie(w http.ResponseWriter, entityID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "domino_h_" + entityID,
		Value:    macSign("host:" + entityID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 30,
	})
}

// IsHost verifica se a requisição possui credenciais válidas de anfitrião.
func IsHost(r *http.Request, entityID string) bool {
	c, err := r.Cookie("domino_h_" + entityID)
	if err != nil {
		return false
	}
	return macEqual(c.Value, macSign("host:"+entityID))
}

// ─── Player Cookie ────────────────────────────────────────────────────────────

// SetPlayerCookie grava o cookie de autenticação do jogador para uma partida.
func SetPlayerCookie(w http.ResponseWriter, matchID, playerID string) {
	tok := playerID + "." + macSign("pid:"+matchID+":"+playerID)
	http.SetCookie(w, &http.Cookie{
		Name:     "domino_p_" + matchID,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 30,
	})
}

// GetAuthPlayerID retorna o playerID autenticado via cookie, ou "" se inválido/ausente.
func GetAuthPlayerID(r *http.Request, matchID string) string {
	c, err := r.Cookie("domino_p_" + matchID)
	if err != nil {
		return ""
	}
	dot := strings.IndexByte(c.Value, '.')
	if dot < 0 {
		return ""
	}
	pid, sig := c.Value[:dot], c.Value[dot+1:]
	if !macEqual(sig, macSign("pid:"+matchID+":"+pid)) {
		return ""
	}
	return pid
}

// ─── CSRF Token ───────────────────────────────────────────────────────────────

// GenerateCSRFToken cria um token CSRF baseado em tempo (válido 2 horas, rotação por hora).
func GenerateCSRFToken(entityID string) string {
	hour := time.Now().UTC().Format("2006010215")
	return macSign("csrf:" + entityID + ":" + hour)
}

// ValidateCSRFToken verifica se o token CSRF enviado é válido (hora atual ou anterior).
func ValidateCSRFToken(token, entityID string) bool {
	if token == "" {
		return false
	}
	now := time.Now().UTC()
	h1 := now.Format("2006010215")
	h2 := now.Add(-time.Hour).Format("2006010215")
	return macEqual(token, macSign("csrf:"+entityID+":"+h1)) ||
		macEqual(token, macSign("csrf:"+entityID+":"+h2))
}

// ─── Rate Limiting ────────────────────────────────────────────────────────────

type rateRecord struct {
	count     int
	windowEnd time.Time
}

var (
	rateMu        sync.Mutex
	uploadRateMap = map[string]*rateRecord{}
	actionRateMap = map[string]*rateRecord{}
)

func checkRate(m map[string]*rateRecord, key string, window time.Duration, max int) bool {
	now := time.Now()
	rec, ok := m[key]
	if !ok || now.After(rec.windowEnd) {
		m[key] = &rateRecord{count: 1, windowEnd: now.Add(window)}
		return true
	}
	if rec.count >= max {
		return false
	}
	rec.count++
	return true
}

// CheckUploadRateLimit — máx. 5 uploads por IP em 5 minutos.
func CheckUploadRateLimit(r *http.Request) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	return checkRate(uploadRateMap, clientIP(r), 5*time.Minute, 5)
}

// CheckActionRateLimit — máx. 60 ações POST por IP por minuto.
func CheckActionRateLimit(r *http.Request) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	return checkRate(actionRateMap, clientIP(r), time.Minute, 60)
}

// CleanRateMap remove entradas expiradas (chame periodicamente).
func CleanRateMap() {
	rateMu.Lock()
	defer rateMu.Unlock()
	now := time.Now()
	for k, v := range uploadRateMap {
		if now.After(v.windowEnd) {
			delete(uploadRateMap, k)
		}
	}
	for k, v := range actionRateMap {
		if now.After(v.windowEnd) {
			delete(actionRateMap, k)
		}
	}
}

// ─── Security Headers ─────────────────────────────────────────────────────────

// SecurityHeaders adiciona headers de segurança e bloqueia acesso direto a /uploads/.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=self, microphone=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src https://fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self'")
		if strings.HasPrefix(r.URL.Path, "/uploads/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Input Sanitization ───────────────────────────────────────────────────────

// SanitizeInput remove caracteres de controle, limita comprimento e corta espaços.
func SanitizeInput(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	s = strings.TrimSpace(b.String())
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen])
	}
	return s
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// clientIP extrai o IP real do cliente.
// Só confia em X-Forwarded-For quando TRUST_PROXY=true está definido (proxy reverso).
func clientIP(r *http.Request) string {
	if os.Getenv("TRUST_PROXY") != "" {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if i := strings.Index(fwd, ","); i >= 0 {
				return strings.TrimSpace(fwd[:i])
			}
			return strings.TrimSpace(fwd)
		}
		if fwd := r.Header.Get("X-Real-IP"); fwd != "" {
			return strings.TrimSpace(fwd)
		}
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[:i]
	}
	return addr
}
