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
	// Generate random secret if SESSION_SECRET is not defined.
	// Sessions expire on server restart — set SESSION_SECRET in production.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate app secret: " + err.Error())
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

// SetHostCookie stores the host authentication cookie for a match or tournament.
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

// IsHost checks if the request has valid host credentials.
func IsHost(r *http.Request, entityID string) bool {
	c, err := r.Cookie("domino_h_" + entityID)
	if err != nil {
		return false
	}
	return macEqual(c.Value, macSign("host:"+entityID))
}

// ─── Player Cookie ────────────────────────────────────────────────────────────

// SetPlayerCookie stores the player authentication cookie for a match.
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

// GetAuthPlayerID returns the authenticated playerID from cookie, or "" if invalid/missing.
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

// GenerateCSRFToken creates a time-based CSRF token (valid 2 hours, rotated hourly).
func GenerateCSRFToken(entityID string) string {
	hour := time.Now().UTC().Format("2006010215")
	return macSign("csrf:" + entityID + ":" + hour)
}

// ValidateCSRFToken checks if the submitted CSRF token is valid (current or previous hour).
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

// CheckUploadRateLimit — max 5 uploads per IP in 5 minutes.
func CheckUploadRateLimit(r *http.Request) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	return checkRate(uploadRateMap, clientIP(r), 5*time.Minute, 5)
}

// CheckActionRateLimit — max 60 POST actions per IP per minute.
func CheckActionRateLimit(r *http.Request) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	return checkRate(actionRateMap, clientIP(r), time.Minute, 60)
}

// CleanRateMap removes expired entries (call periodically).
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

// SecurityHeaders adds security headers and blocks direct access to /uploads/.
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

// SanitizeInput removes control characters, limits length, and trims whitespace.
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

// clientIP extracts the real client IP.
// Only trusts X-Forwarded-For when TRUST_PROXY=true is set (reverse proxy).
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
