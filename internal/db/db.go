package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/leandrodaf/domino-placar/internal/models"

	_ "modernc.org/sqlite"
)

func OpenDB(path string) (*sql.DB, error) {
	// Garante que o arquivo exista com permissões de escrita antes de abrir
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating db file: %w", err)
	}
	f.Close()

	// URI format: mode=rwc cria se não existir, _journal_mode=WAL melhora concorrência
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func CreateTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tournaments (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT 'Torneio',
			status TEXT NOT NULL DEFAULT 'registration',
			base_url TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tournament_players (
			id TEXT PRIMARY KEY,
			tournament_id TEXT NOT NULL,
			name TEXT NOT NULL,
			unique_identifier TEXT NOT NULL,
			table_number INTEGER NOT NULL DEFAULT 0,
			match_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tournament_id) REFERENCES tournaments(id)
		)`,
		`CREATE TABLE IF NOT EXISTS tournament_matches (
			tournament_id TEXT NOT NULL,
			match_id TEXT NOT NULL,
			table_number INTEGER NOT NULL,
			PRIMARY KEY (tournament_id, match_id),
			FOREIGN KEY (tournament_id) REFERENCES tournaments(id),
			FOREIGN KEY (match_id) REFERENCES matches(id)
		)`,
		`CREATE TABLE IF NOT EXISTS nickname_nominations (
			id TEXT PRIMARY KEY,
			nominated_unique_id TEXT NOT NULL,
			nominated_name TEXT NOT NULL DEFAULT '',
			match_id TEXT NOT NULL,
			nickname TEXT NOT NULL,
			vote_count INTEGER NOT NULL DEFAULT 1,
			proposer_unique_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS nickname_votes (
			nomination_id TEXT NOT NULL,
			voter_unique_id TEXT NOT NULL,
			PRIMARY KEY (nomination_id, voter_unique_id)
		)`,
		`CREATE TABLE IF NOT EXISTS matches (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'waiting',
			base_url TEXT NOT NULL,
			winner_player_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			match_id TEXT NOT NULL,
			name TEXT NOT NULL,
			unique_identifier TEXT NOT NULL,
			total_score INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (match_id) REFERENCES matches(id)
		)`,
		`CREATE TABLE IF NOT EXISTS rounds (
			id TEXT PRIMARY KEY,
			match_id TEXT NOT NULL,
			round_number INTEGER NOT NULL,
			winner_player_id TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			table_image_path TEXT NOT NULL DEFAULT '',
			table_detected_tiles TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (match_id) REFERENCES matches(id)
		)`,
		`CREATE TABLE IF NOT EXISTS hand_images (
			id TEXT PRIMARY KEY,
			round_id TEXT NOT NULL,
			player_id TEXT NOT NULL,
			image_path TEXT NOT NULL,
			points_calculated INTEGER NOT NULL DEFAULT 0,
			confirmed INTEGER NOT NULL DEFAULT 0,
			detected_tiles TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (round_id) REFERENCES rounds(id),
			FOREIGN KEY (player_id) REFERENCES players(id)
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("creating table: %w", err)
		}
	}
	// Migrations idempotentes para colunas adicionadas após a criação inicial
	_, _ = db.Exec(`ALTER TABLE matches ADD COLUMN winner_player_id TEXT`)
	_, _ = db.Exec(`ALTER TABLE rounds ADD COLUMN table_image_path TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE rounds ADD COLUMN table_detected_tiles TEXT NOT NULL DEFAULT '[]'`)
	_, _ = db.Exec(`ALTER TABLE hand_images ADD COLUMN detected_tiles TEXT NOT NULL DEFAULT '[]'`)
	_, _ = db.Exec(`ALTER TABLE rounds ADD COLUMN starter_player_id TEXT NOT NULL DEFAULT ''`)

	log.Println("Database tables initialized")
	return nil
}

// Match operations

func CreateMatch(db *sql.DB, id, baseURL string) error {
	_, err := db.Exec(`INSERT INTO matches (id, status, base_url) VALUES (?, 'waiting', ?)`, id, baseURL)
	return err
}

func GetMatch(db *sql.DB, id string) (*models.Match, error) {
	m := &models.Match{}
	var winnerID sql.NullString
	err := db.QueryRow(`SELECT id, status, base_url, COALESCE(winner_player_id,''), created_at FROM matches WHERE id = ?`, id).
		Scan(&m.ID, &m.Status, &m.BaseURL, &winnerID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	if winnerID.Valid {
		m.WinnerPlayerID = winnerID.String
	}
	return m, nil
}

func UpdateMatchStatus(db *sql.DB, id, status string) error {
	_, err := db.Exec(`UPDATE matches SET status = ? WHERE id = ?`, status, id)
	return err
}

// Player operations

func CreatePlayer(db *sql.DB, id, matchID, name, uniqueID string) error {
	_, err := db.Exec(
		`INSERT INTO players (id, match_id, name, unique_identifier, total_score, status) VALUES (?, ?, ?, ?, 0, 'active')`,
		id, matchID, name, uniqueID,
	)
	return err
}

func GetPlayers(db *sql.DB, matchID string) ([]models.Player, error) {
	rows, err := db.Query(`SELECT id, match_id, name, unique_identifier, total_score, status, created_at FROM players WHERE match_id = ? ORDER BY created_at`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.Player
	for rows.Next() {
		var p models.Player
		if err := rows.Scan(&p.ID, &p.MatchID, &p.Name, &p.UniqueIdentifier, &p.TotalScore, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func GetPlayer(db *sql.DB, playerID string) (*models.Player, error) {
	p := &models.Player{}
	err := db.QueryRow(`SELECT id, match_id, name, unique_identifier, total_score, status, created_at FROM players WHERE id = ?`, playerID).
		Scan(&p.ID, &p.MatchID, &p.Name, &p.UniqueIdentifier, &p.TotalScore, &p.Status, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func GetPlayerByUniqueID(db *sql.DB, matchID, uniqueID string) (*models.Player, error) {
	p := &models.Player{}
	err := db.QueryRow(`SELECT id, match_id, name, unique_identifier, total_score, status, created_at FROM players WHERE match_id = ? AND unique_identifier = ?`, matchID, uniqueID).
		Scan(&p.ID, &p.MatchID, &p.Name, &p.UniqueIdentifier, &p.TotalScore, &p.Status, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func UpdatePlayerScore(db *sql.DB, playerID string, additionalPoints int) error {
	_, err := db.Exec(`UPDATE players SET total_score = total_score + ? WHERE id = ?`, additionalPoints, playerID)
	return err
}

func UpdatePlayerStatus(db *sql.DB, playerID, status string) error {
	_, err := db.Exec(`UPDATE players SET status = ? WHERE id = ?`, status, playerID)
	return err
}

// Round operations

func CreateRound(db *sql.DB, id, matchID string, roundNumber int) error {
	_, err := db.Exec(`INSERT INTO rounds (id, match_id, round_number, status) VALUES (?, ?, ?, 'active')`, id, matchID, roundNumber)
	return err
}

func GetLastFinishedRound(db *sql.DB, matchID string) (*models.Round, error) {
	r := &models.Round{}
	var winnerID, starterID sql.NullString
	err := db.QueryRow(`SELECT id, match_id, round_number, winner_player_id, COALESCE(starter_player_id,''), status, created_at FROM rounds WHERE match_id = ? AND status = 'finished' ORDER BY round_number DESC LIMIT 1`, matchID).
		Scan(&r.ID, &r.MatchID, &r.RoundNumber, &winnerID, &starterID, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	if winnerID.Valid {
		r.WinnerPlayerID = winnerID.String
	}
	if starterID.Valid {
		r.StarterPlayerID = starterID.String
	}
	return r, nil
}

func GetCurrentRound(db *sql.DB, matchID string) (*models.Round, error) {
	r := &models.Round{}
	var winnerID, starterID sql.NullString
	err := db.QueryRow(`SELECT id, match_id, round_number, winner_player_id, COALESCE(starter_player_id,''), status, created_at FROM rounds WHERE match_id = ? AND status = 'active' ORDER BY round_number DESC LIMIT 1`, matchID).
		Scan(&r.ID, &r.MatchID, &r.RoundNumber, &winnerID, &starterID, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	if winnerID.Valid {
		r.WinnerPlayerID = winnerID.String
	}
	if starterID.Valid {
		r.StarterPlayerID = starterID.String
	}
	return r, nil
}

func GetRound(db *sql.DB, roundID string) (*models.Round, error) {
	r := &models.Round{}
	var winnerID, starterID sql.NullString
	err := db.QueryRow(`SELECT id, match_id, round_number, winner_player_id, COALESCE(starter_player_id,''), status, created_at FROM rounds WHERE id = ?`, roundID).
		Scan(&r.ID, &r.MatchID, &r.RoundNumber, &winnerID, &starterID, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	if winnerID.Valid {
		r.WinnerPlayerID = winnerID.String
	}
	if starterID.Valid {
		r.StarterPlayerID = starterID.String
	}
	return r, nil
}

func SetRoundStarter(db *sql.DB, roundID, playerID string) error {
	_, err := db.Exec(`UPDATE rounds SET starter_player_id = ? WHERE id = ?`, playerID, roundID)
	return err
}

func CountRounds(db *sql.DB, matchID string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM rounds WHERE match_id = ?`, matchID).Scan(&count)
	return count, err
}

func SetRoundWinner(db *sql.DB, roundID, playerID string) error {
	_, err := db.Exec(`UPDATE rounds SET winner_player_id = ? WHERE id = ?`, playerID, roundID)
	return err
}

func FinishRound(db *sql.DB, roundID string) error {
	_, err := db.Exec(`UPDATE rounds SET status = 'finished' WHERE id = ?`, roundID)
	return err
}

// HandImage operations

func CreateHandImage(db *sql.DB, id, roundID, playerID, imagePath string) error {
	_, err := db.Exec(`INSERT INTO hand_images (id, round_id, player_id, image_path, points_calculated, confirmed, detected_tiles) VALUES (?, ?, ?, ?, 0, 0, '[]')`, id, roundID, playerID, imagePath)
	return err
}

func GetHandImage(db *sql.DB, imageID string) (*models.HandImage, error) {
	h := &models.HandImage{}
	err := db.QueryRow(`SELECT id, round_id, player_id, image_path, points_calculated, confirmed, detected_tiles, created_at FROM hand_images WHERE id = ?`, imageID).
		Scan(&h.ID, &h.RoundID, &h.PlayerID, &h.ImagePath, &h.PointsCalculated, &h.Confirmed, &h.DetectedTilesJSON, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func GetHandImageByRoundAndPlayer(db *sql.DB, roundID, playerID string) (*models.HandImage, error) {
	h := &models.HandImage{}
	err := db.QueryRow(`SELECT id, round_id, player_id, image_path, points_calculated, confirmed, detected_tiles, created_at FROM hand_images WHERE round_id = ? AND player_id = ?`, roundID, playerID).
		Scan(&h.ID, &h.RoundID, &h.PlayerID, &h.ImagePath, &h.PointsCalculated, &h.Confirmed, &h.DetectedTilesJSON, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return h, nil
}

func UpdateHandImagePoints(db *sql.DB, imageID string, points int, confirmed bool, tilesJSON string) error {
	c := 0
	if confirmed {
		c = 1
	}
	_, err := db.Exec(`UPDATE hand_images SET points_calculated = ?, confirmed = ?, detected_tiles = ? WHERE id = ?`, points, c, tilesJSON, imageID)
	return err
}

func GetHandImages(db *sql.DB, roundID string) ([]models.HandImage, error) {
	rows, err := db.Query(`SELECT id, round_id, player_id, image_path, points_calculated, confirmed, detected_tiles, created_at FROM hand_images WHERE round_id = ?`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []models.HandImage
	for rows.Next() {
		var h models.HandImage
		if err := rows.Scan(&h.ID, &h.RoundID, &h.PlayerID, &h.ImagePath, &h.PointsCalculated, &h.Confirmed, &h.DetectedTilesJSON, &h.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, h)
	}
	return images, rows.Err()
}

// SetTableImage salva o caminho e as pedras detectadas na foto da mesa.
func SetTableImage(db *sql.DB, roundID, imagePath, tilesJSON string) error {
	_, err := db.Exec(`UPDATE rounds SET table_image_path = ?, table_detected_tiles = ? WHERE id = ?`, imagePath, tilesJSON, roundID)
	return err
}

// GetRoundTableTiles retorna as pedras detectadas na foto da mesa de uma rodada.
func GetRoundTableTiles(db *sql.DB, roundID string) (string, string, error) {
	var path, tilesJSON string
	err := db.QueryRow(`SELECT table_image_path, table_detected_tiles FROM rounds WHERE id = ?`, roundID).Scan(&path, &tilesJSON)
	return path, tilesJSON, err
}

func GetRanking(db *sql.DB, matchID string) ([]models.Player, error) {
	rows, err := db.Query(`SELECT id, match_id, name, unique_identifier, total_score, status, created_at FROM players WHERE match_id = ? ORDER BY total_score ASC`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.Player
	for rows.Next() {
		var p models.Player
		if err := rows.Scan(&p.ID, &p.MatchID, &p.Name, &p.UniqueIdentifier, &p.TotalScore, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// SetMatchWinner registra o vencedor da partida.
func SetMatchWinner(db *sql.DB, matchID, playerID string) error {
	_, err := db.Exec(`UPDATE matches SET winner_player_id = ? WHERE id = ?`, playerID, matchID)
	return err
}

// SetPlayerScore define a pontuação absoluta de um jogador e recalcula seu status.
func SetPlayerScore(db *sql.DB, playerID string, score int) error {
	status := "active"
	if score > 51 {
		status = "estourou"
	}
	_, err := db.Exec(`UPDATE players SET total_score = ?, status = ? WHERE id = ?`, score, status, playerID)
	return err
}

// CountPlayersByMatch retorna quantos jogadores estão em uma partida.
func CountPlayersByMatch(db *sql.DB, matchID string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM players WHERE match_id = ?`, matchID).Scan(&count)
	return count, err
}

// Tournament operations

func CreateTournament(db *sql.DB, id, name, baseURL string) error {
	_, err := db.Exec(`INSERT INTO tournaments (id, name, status, base_url) VALUES (?, ?, 'registration', ?)`, id, name, baseURL)
	return err
}

func GetTournament(db *sql.DB, id string) (*models.Tournament, error) {
	t := &models.Tournament{}
	err := db.QueryRow(`SELECT id, name, status, base_url, created_at FROM tournaments WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Status, &t.BaseURL, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func UpdateTournamentStatus(db *sql.DB, id, status string) error {
	_, err := db.Exec(`UPDATE tournaments SET status = ? WHERE id = ?`, status, id)
	return err
}

// Tournament player operations

func CreateTournamentPlayer(db *sql.DB, id, tournamentID, name, uniqueID string) error {
	_, err := db.Exec(
		`INSERT INTO tournament_players (id, tournament_id, name, unique_identifier, table_number, match_id) VALUES (?, ?, ?, ?, 0, '')`,
		id, tournamentID, name, uniqueID,
	)
	return err
}

func GetTournamentPlayers(db *sql.DB, tournamentID string) ([]models.TournamentPlayer, error) {
	rows, err := db.Query(`SELECT id, tournament_id, name, unique_identifier, table_number, match_id, created_at FROM tournament_players WHERE tournament_id = ? ORDER BY created_at`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.TournamentPlayer
	for rows.Next() {
		var p models.TournamentPlayer
		if err := rows.Scan(&p.ID, &p.TournamentID, &p.Name, &p.UniqueIdentifier, &p.TableNumber, &p.MatchID, &p.CreatedAt); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func GetTournamentPlayerByUniqueID(db *sql.DB, tournamentID, uniqueID string) (*models.TournamentPlayer, error) {
	p := &models.TournamentPlayer{}
	err := db.QueryRow(`SELECT id, tournament_id, name, unique_identifier, table_number, match_id, created_at FROM tournament_players WHERE tournament_id = ? AND unique_identifier = ?`, tournamentID, uniqueID).
		Scan(&p.ID, &p.TournamentID, &p.Name, &p.UniqueIdentifier, &p.TableNumber, &p.MatchID, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func CountTournamentPlayers(db *sql.DB, tournamentID string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM tournament_players WHERE tournament_id = ?`, tournamentID).Scan(&count)
	return count, err
}

func AssignTournamentPlayer(db *sql.DB, playerID string, tableNum int, matchID string) error {
	_, err := db.Exec(`UPDATE tournament_players SET table_number = ?, match_id = ? WHERE id = ?`, tableNum, matchID, playerID)
	return err
}

// Tournament match operations

func CreateTournamentMatch(db *sql.DB, tournamentID, matchID string, tableNum int) error {
	_, err := db.Exec(`INSERT INTO tournament_matches (tournament_id, match_id, table_number) VALUES (?, ?, ?)`, tournamentID, matchID, tableNum)
	return err
}

func GetTournamentMatches(db *sql.DB, tournamentID string) ([]models.TournamentMatch, error) {
	rows, err := db.Query(`SELECT tournament_id, match_id, table_number FROM tournament_matches WHERE tournament_id = ? ORDER BY table_number`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.TournamentMatch
	for rows.Next() {
		var m models.TournamentMatch
		if err := rows.Scan(&m.TournamentID, &m.MatchID, &m.TableNumber); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func GetTournamentRanking(db *sql.DB, tournamentID string) ([]models.TournamentRankEntry, error) {
	const q = `
	SELECT tp.unique_identifier, tp.name, tp.table_number, tp.match_id,
	       COALESCE(p.total_score, 0) as score, COALESCE(p.status, 'active') as status
	FROM tournament_players tp
	LEFT JOIN players p ON p.match_id = tp.match_id AND p.unique_identifier = tp.unique_identifier
	WHERE tp.tournament_id = ?
	ORDER BY score ASC`

	rows, err := db.Query(q, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.TournamentRankEntry
	for rows.Next() {
		var e models.TournamentRankEntry
		if err := rows.Scan(&e.UniqueIdentifier, &e.Name, &e.TableNumber, &e.MatchID, &e.Score, &e.Status); err != nil {
			return nil, err
		}
		if e.Score < 0 {
			e.Score = 0
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetGlobalStats retorna estatísticas globais agrupadas por unique_identifier.
func GetGlobalStats(db *sql.DB) ([]models.GlobalStat, error) {
	const q = `
	WITH match_wins AS (
		SELECT p.unique_identifier, COUNT(*) AS wins
		FROM players p
		JOIN matches m ON p.match_id = m.id AND m.winner_player_id = p.id
		GROUP BY p.unique_identifier
	),
	round_wins AS (
		SELECT p.unique_identifier, COUNT(*) AS wins
		FROM players p
		JOIN rounds r ON r.winner_player_id = p.id
		GROUP BY p.unique_identifier
	),
	base AS (
		SELECT
			unique_identifier,
			MAX(name) AS name,
			COUNT(DISTINCT match_id) AS matches_played,
			SUM(CASE WHEN status = 'estourou' THEN 1 ELSE 0 END) AS bust_count,
			MAX(CASE WHEN status = 'estourou' THEN total_score ELSE 0 END) AS max_bust_score
		FROM players
		GROUP BY unique_identifier
	)
	SELECT
		b.unique_identifier,
		b.name,
		b.matches_played,
		COALESCE(mw.wins, 0) AS matches_won,
		b.bust_count,
		b.max_bust_score,
		COALESCE(rw.wins, 0) AS total_round_wins
	FROM base b
	LEFT JOIN match_wins mw ON b.unique_identifier = mw.unique_identifier
	LEFT JOIN round_wins rw ON b.unique_identifier = rw.unique_identifier
	ORDER BY matches_won DESC, total_round_wins DESC, b.bust_count ASC
	`
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.GlobalStat
	for rows.Next() {
		var s models.GlobalStat
		if err := rows.Scan(&s.UniqueIdentifier, &s.Name, &s.MatchesPlayed, &s.MatchesWon, &s.BustCount, &s.MaxBustScore, &s.TotalRoundWins); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// ─── Zoeiras / Hall da Fama ──────────────────────────────────────────────────

// GetMostRoundsLost retorna quem perdeu mais rodadas (não venceu, mas estava ativo).
func GetMostRoundsLost(db *sql.DB) ([]models.ZoeiraStat, error) {
	const q = `
	SELECT p.unique_identifier, MAX(p.name) as name, COUNT(*) as cnt
	FROM hand_images hi
	JOIN players p ON hi.player_id = p.id
	JOIN rounds r ON hi.round_id = r.id
	WHERE hi.confirmed = 1 AND r.winner_player_id IS NOT NULL AND r.winner_player_id != hi.player_id
	GROUP BY p.unique_identifier
	ORDER BY cnt DESC
	LIMIT 10`
	return queryZoeiraStats(db, q)
}

// GetPintoKings retorna quem mais terminou rodadas segurando a pedra de 1 ponto (0-1).
func GetPintoKings(db *sql.DB) ([]models.ZoeiraStat, error) {
	const q = `
	SELECT p.unique_identifier, MAX(p.name) as name, COUNT(*) as cnt
	FROM hand_images hi
	JOIN players p ON hi.player_id = p.id
	WHERE hi.confirmed = 1 AND hi.detected_tiles LIKE '%"0-1"%'
	GROUP BY p.unique_identifier
	ORDER BY cnt DESC
	LIMIT 10`
	return queryZoeiraStats(db, q)
}

// GetBrancoKings retorna quem mais terminou com apenas a pedra branca (0-0) — e pagou 12 pts.
func GetBrancoKings(db *sql.DB) ([]models.ZoeiraStat, error) {
	const q = `
	SELECT p.unique_identifier, MAX(p.name) as name, COUNT(*) as cnt
	FROM hand_images hi
	JOIN players p ON hi.player_id = p.id
	WHERE hi.confirmed = 1 AND hi.detected_tiles = '["0-0"]'
	GROUP BY p.unique_identifier
	ORDER BY cnt DESC
	LIMIT 10`
	return queryZoeiraStats(db, q)
}

// GetCloseCallKings retorna quem mais ficou no limite (entre 45-51 pontos) sem estourar.
func GetCloseCallKings(db *sql.DB) ([]models.ZoeiraStat, error) {
	const q = `
	SELECT unique_identifier, MAX(name) as name, COUNT(*) as cnt
	FROM players
	WHERE total_score >= 45 AND status = 'active'
	GROUP BY unique_identifier
	ORDER BY cnt DESC
	LIMIT 10`
	return queryZoeiraStats(db, q)
}

func queryZoeiraStats(db *sql.DB, q string) ([]models.ZoeiraStat, error) {
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []models.ZoeiraStat
	for rows.Next() {
		var s models.ZoeiraStat
		if err := rows.Scan(&s.UniqueIdentifier, &s.Name, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// ─── Apelidos ────────────────────────────────────────────────────────────────

// CreateNomination cria uma nova proposta de apelido.
func CreateNomination(db *sql.DB, id, nominatedUID, nominatedName, matchID, nickname, proposerUID string) error {
	_, err := db.Exec(
		`INSERT INTO nickname_nominations (id, nominated_unique_id, nominated_name, match_id, nickname, vote_count, proposer_unique_id)
		 VALUES (?, ?, ?, ?, ?, 1, ?)`,
		id, nominatedUID, nominatedName, matchID, nickname, proposerUID,
	)
	return err
}

// VoteForNomination registra um voto e incrementa o contador.
// Retorna false (sem erro) se o votante já votou nesta nomination.
func VoteForNomination(db *sql.DB, nominationID, voterUID string) (bool, error) {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO nickname_votes (nomination_id, voter_unique_id) VALUES (?, ?)`,
		nominationID, voterUID,
	)
	if err != nil {
		return false, err
	}
	// Verifica se a linha foi inserida (changes > 0 = voto novo)
	var changes int
	db.QueryRow(`SELECT changes()`).Scan(&changes)
	if changes == 0 {
		return false, nil // já votou
	}
	_, err = db.Exec(`UPDATE nickname_nominations SET vote_count = vote_count + 1 WHERE id = ?`, nominationID)
	return true, err
}

// GetNominationsForMatch retorna todas as nominations de uma partida, ordenadas por votos.
func GetNominationsForMatch(db *sql.DB, matchID string) ([]models.NicknameNomination, error) {
	rows, err := db.Query(
		`SELECT id, nominated_unique_id, nominated_name, match_id, nickname, vote_count, proposer_unique_id, created_at
		 FROM nickname_nominations WHERE match_id = ? ORDER BY vote_count DESC, created_at ASC`,
		matchID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var noms []models.NicknameNomination
	for rows.Next() {
		var n models.NicknameNomination
		if err := rows.Scan(&n.ID, &n.NominatedUniqueID, &n.NominatedName, &n.MatchID, &n.Nickname, &n.VoteCount, &n.ProposerUniqueID, &n.CreatedAt); err != nil {
			return nil, err
		}
		noms = append(noms, n)
	}
	return noms, rows.Err()
}

// GetNominationsForPlayer retorna nominations para um unique_id específico numa partida.
func GetNominationsForPlayer(db *sql.DB, matchID, nominatedUID string) ([]models.NicknameNomination, error) {
	rows, err := db.Query(
		`SELECT id, nominated_unique_id, nominated_name, match_id, nickname, vote_count, proposer_unique_id, created_at
		 FROM nickname_nominations WHERE match_id = ? AND nominated_unique_id = ? ORDER BY vote_count DESC`,
		matchID, nominatedUID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var noms []models.NicknameNomination
	for rows.Next() {
		var n models.NicknameNomination
		if err := rows.Scan(&n.ID, &n.NominatedUniqueID, &n.NominatedName, &n.MatchID, &n.Nickname, &n.VoteCount, &n.ProposerUniqueID, &n.CreatedAt); err != nil {
			return nil, err
		}
		noms = append(noms, n)
	}
	return noms, rows.Err()
}

// GetTopNicknameForPlayer retorna o apelido mais votado para um unique_id (em qualquer partida).
func GetTopNicknameForPlayer(db *sql.DB, uniqueID string) string {
	var nickname string
	db.QueryRow(
		`SELECT nickname FROM nickname_nominations
		 WHERE nominated_unique_id = ? ORDER BY vote_count DESC LIMIT 1`,
		uniqueID,
	).Scan(&nickname)
	return nickname
}

// GetAllTimeNicknames retorna os apelidos mais votados por jogador (para Hall da Fama).
func GetAllTimeNicknames(db *sql.DB) ([]models.NicknameNomination, error) {
	const q = `
	SELECT id, nominated_unique_id, nominated_name, match_id, nickname, vote_count, proposer_unique_id, created_at
	FROM (
		SELECT *, ROW_NUMBER() OVER (PARTITION BY nominated_unique_id ORDER BY vote_count DESC) as rn
		FROM nickname_nominations
	) WHERE rn = 1 AND vote_count >= 2
	ORDER BY vote_count DESC
	LIMIT 20`
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var noms []models.NicknameNomination
	for rows.Next() {
		var n models.NicknameNomination
		if err := rows.Scan(&n.ID, &n.NominatedUniqueID, &n.NominatedName, &n.MatchID, &n.Nickname, &n.VoteCount, &n.ProposerUniqueID, &n.CreatedAt); err != nil {
			return nil, err
		}
		noms = append(noms, n)
	}
	return noms, rows.Err()
}
