package db

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	fbdb "firebase.google.com/go/v4/db"
	"github.com/leandrodaf/domino-placar/internal/models"
	"google.golang.org/api/option"
)

var errNotFound = errors.New("not found")

// FirebaseStore implements Store using Firebase Realtime Database.
type FirebaseStore struct {
	client *fbdb.Client
}

// NewFirebaseStore creates a FirebaseStore connected to the Firebase database.
// databaseURL: database URL, e.g. "https://my-project-default-rtdb.firebaseio.com"
// credentialsJSON: service account JSON content (optional; if empty, uses Application Default Credentials)
func NewFirebaseStore(databaseURL, credentialsJSON string) (*FirebaseStore, error) {
	ctx := context.Background()
	cfg := &firebase.Config{DatabaseURL: databaseURL}

	var app *firebase.App
	var err error
	if credentialsJSON != "" {
		app, err = firebase.NewApp(ctx, cfg, option.WithCredentialsJSON([]byte(credentialsJSON)))
	} else {
		app, err = firebase.NewApp(ctx, cfg)
	}
	if err != nil {
		return nil, err
	}

	client, err := app.Database(ctx)
	if err != nil {
		return nil, err
	}
	return &FirebaseStore{client: client}, nil
}

// ─── Structs Firebase ────────────────────────────────────────────────────────

type fbMatch struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	BaseURL        string `json:"base_url"`
	WinnerPlayerID string `json:"winner_player_id,omitempty"`
	TurmaID        string `json:"turma_id,omitempty"`
	CreatedAt      string `json:"created_at"`
}

func (f *fbMatch) toModel() *models.Match {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return &models.Match{ID: f.ID, Status: f.Status, BaseURL: f.BaseURL, WinnerPlayerID: f.WinnerPlayerID, TurmaID: f.TurmaID, CreatedAt: t}
}

type fbPlayer struct {
	ID               string `json:"id"`
	MatchID          string `json:"match_id"`
	Name             string `json:"name"`
	UniqueIdentifier string `json:"unique_identifier"`
	TotalScore       int    `json:"total_score"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
}

func (f *fbPlayer) toModel() models.Player {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return models.Player{
		ID: f.ID, MatchID: f.MatchID, Name: f.Name,
		UniqueIdentifier: f.UniqueIdentifier, TotalScore: f.TotalScore,
		Status: f.Status, CreatedAt: t,
	}
}

type fbRound struct {
	ID                 string `json:"id"`
	MatchID            string `json:"match_id"`
	RoundNumber        int    `json:"round_number"`
	WinnerPlayerID     string `json:"winner_player_id,omitempty"`
	StarterPlayerID    string `json:"starter_player_id,omitempty"`
	Status             string `json:"status"`
	TableImagePath     string `json:"table_image_path,omitempty"`
	TableDetectedTiles string `json:"table_detected_tiles,omitempty"`
	CreatedAt          string `json:"created_at"`
}

func (f *fbRound) toModel() *models.Round {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return &models.Round{
		ID: f.ID, MatchID: f.MatchID, RoundNumber: f.RoundNumber,
		WinnerPlayerID: f.WinnerPlayerID, StarterPlayerID: f.StarterPlayerID,
		Status: f.Status, CreatedAt: t,
	}
}

type fbHandImage struct {
	ID                string `json:"id"`
	RoundID           string `json:"round_id"`
	PlayerID          string `json:"player_id"`
	ImagePath         string `json:"image_path"`
	PointsCalculated  int    `json:"points_calculated"`
	Confirmed         int    `json:"confirmed"`
	DetectedTilesJSON string `json:"detected_tiles"`
	CreatedAt         string `json:"created_at"`
}

func (f *fbHandImage) toModel() models.HandImage {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return models.HandImage{
		ID: f.ID, RoundID: f.RoundID, PlayerID: f.PlayerID,
		ImagePath: f.ImagePath, PointsCalculated: f.PointsCalculated,
		Confirmed: f.Confirmed, DetectedTilesJSON: f.DetectedTilesJSON, CreatedAt: t,
	}
}

type fbTournament struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	BaseURL   string `json:"base_url"`
	CreatedAt string `json:"created_at"`
}

func (f *fbTournament) toModel() *models.Tournament {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return &models.Tournament{ID: f.ID, Name: f.Name, Status: f.Status, BaseURL: f.BaseURL, CreatedAt: t}
}

type fbTournamentPlayer struct {
	ID               string `json:"id"`
	TournamentID     string `json:"tournament_id"`
	Name             string `json:"name"`
	UniqueIdentifier string `json:"unique_identifier"`
	TableNumber      int    `json:"table_number"`
	MatchID          string `json:"match_id"`
	CreatedAt        string `json:"created_at"`
}

func (f *fbTournamentPlayer) toModel() models.TournamentPlayer {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return models.TournamentPlayer{
		ID: f.ID, TournamentID: f.TournamentID, Name: f.Name,
		UniqueIdentifier: f.UniqueIdentifier, TableNumber: f.TableNumber,
		MatchID: f.MatchID, CreatedAt: t,
	}
}

type fbNicknameNomination struct {
	ID                string `json:"id"`
	NominatedUniqueID string `json:"nominated_unique_id"`
	NominatedName     string `json:"nominated_name"`
	MatchID           string `json:"match_id"`
	Nickname          string `json:"nickname"`
	VoteCount         int    `json:"vote_count"`
	ProposerUniqueID  string `json:"proposer_unique_id"`
	CreatedAt         string `json:"created_at"`
}

func (f *fbNicknameNomination) toModel() models.NicknameNomination {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return models.NicknameNomination{
		ID: f.ID, NominatedUniqueID: f.NominatedUniqueID, NominatedName: f.NominatedName,
		MatchID: f.MatchID, Nickname: f.Nickname, VoteCount: f.VoteCount,
		ProposerUniqueID: f.ProposerUniqueID, CreatedAt: t,
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (s *FirebaseStore) ref(path string) *fbdb.Ref {
	return s.client.NewRef(path)
}

func nowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// safeKey converts a string to a valid Firebase key (without '.', '#', '$', '[', ']').
func safeKey(k string) string {
	r := strings.NewReplacer(".", "_", "#", "_", "$", "_", "[", "_", "]", "_", "/", "_")
	return r.Replace(k)
}

// ─── Match ───────────────────────────────────────────────────────────────────

func (s *FirebaseStore) CreateMatch(id, baseURL string) error {
	ctx := context.Background()
	m := fbMatch{ID: id, Status: "waiting", BaseURL: baseURL, CreatedAt: nowStr()}
	return s.ref("matches/"+id).Set(ctx, m)
}

func (s *FirebaseStore) GetMatch(id string) (*models.Match, error) {
	ctx := context.Background()
	var m fbMatch
	if err := s.ref("matches/"+id).Get(ctx, &m); err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, errNotFound
	}
	return m.toModel(), nil
}

func (s *FirebaseStore) UpdateMatchStatus(id, status string) error {
	ctx := context.Background()
	return s.ref("matches/"+id).Update(ctx, map[string]interface{}{"status": status})
}

func (s *FirebaseStore) SetMatchWinner(matchID, playerID string) error {
	ctx := context.Background()
	return s.ref("matches/"+matchID).Update(ctx, map[string]interface{}{"winner_player_id": playerID})
}

// ─── Player ──────────────────────────────────────────────────────────────────

func (s *FirebaseStore) CreatePlayer(id, matchID, name, uniqueID string) error {
	ctx := context.Background()
	p := fbPlayer{
		ID: id, MatchID: matchID, Name: name, UniqueIdentifier: uniqueID,
		TotalScore: 0, Status: "active", CreatedAt: nowStr(),
	}
	if err := s.ref("players/"+id).Set(ctx, p); err != nil {
		return err
	}
	// Index match → players
	return s.ref("idx_player_match/"+matchID+"/"+id).Set(ctx, true)
}

func (s *FirebaseStore) GetPlayers(matchID string) ([]models.Player, error) {
	ctx := context.Background()
	// Read index to get IDs
	var ids map[string]bool
	if err := s.ref("idx_player_match/"+matchID).Get(ctx, &ids); err != nil {
		return nil, err
	}
	var players []models.Player
	for id := range ids {
		var p fbPlayer
		if err := s.ref("players/"+id).Get(ctx, &p); err == nil && p.ID != "" {
			players = append(players, p.toModel())
		}
	}
	sort.Slice(players, func(i, j int) bool {
		return players[i].CreatedAt.Before(players[j].CreatedAt)
	})
	return players, nil
}

func (s *FirebaseStore) GetPlayer(playerID string) (*models.Player, error) {
	ctx := context.Background()
	var p fbPlayer
	if err := s.ref("players/"+playerID).Get(ctx, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, errNotFound
	}
	m := p.toModel()
	return &m, nil
}

func (s *FirebaseStore) GetPlayerByUniqueID(matchID, uniqueID string) (*models.Player, error) {
	players, err := s.GetPlayers(matchID)
	if err != nil {
		return nil, err
	}
	for i := range players {
		if players[i].UniqueIdentifier == uniqueID {
			return &players[i], nil
		}
	}
	return nil, errNotFound
}

func (s *FirebaseStore) UpdatePlayerScore(playerID string, additionalPoints int) error {
	ctx := context.Background()
	return s.ref("players/"+playerID+"/total_score").Transaction(ctx, func(node fbdb.TransactionNode) (interface{}, error) {
		var current int
		if err := node.Unmarshal(&current); err != nil {
			return nil, err
		}
		return current + additionalPoints, nil
	})
}

func (s *FirebaseStore) UpdatePlayerStatus(playerID, status string) error {
	ctx := context.Background()
	return s.ref("players/"+playerID).Update(ctx, map[string]interface{}{"status": status})
}

func (s *FirebaseStore) UpdatePlayerName(playerID, name string) error {
	ctx := context.Background()
	return s.ref("players/"+playerID).Update(ctx, map[string]interface{}{"name": name})
}

func (s *FirebaseStore) SetPlayerScore(playerID string, score int) error {
	ctx := context.Background()
	status := "active"
	if score > 51 {
		status = "estourou"
	}
	return s.ref("players/"+playerID).Update(ctx, map[string]interface{}{
		"total_score": score,
		"status":      status,
	})
}

func (s *FirebaseStore) CountPlayersByMatch(matchID string) (int, error) {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_player_match/"+matchID).Get(ctx, &ids); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *FirebaseStore) GetRanking(matchID string) ([]models.Player, error) {
	players, err := s.GetPlayers(matchID)
	if err != nil {
		return nil, err
	}
	sort.Slice(players, func(i, j int) bool {
		return players[i].TotalScore < players[j].TotalScore
	})
	return players, nil
}

// ─── Round ───────────────────────────────────────────────────────────────────

type fbRoundIdx struct {
	RoundNumber int    `json:"round_number"`
	Status      string `json:"status"`
}

func (s *FirebaseStore) CreateRound(id, matchID string, roundNumber int) error {
	ctx := context.Background()
	r := fbRound{
		ID: id, MatchID: matchID, RoundNumber: roundNumber,
		Status: "active", TableDetectedTiles: "[]", CreatedAt: nowStr(),
	}
	if err := s.ref("rounds/"+id).Set(ctx, r); err != nil {
		return err
	}
	return s.ref("idx_round_match/"+matchID+"/"+id).Set(ctx, fbRoundIdx{
		RoundNumber: roundNumber, Status: "active",
	})
}

func (s *FirebaseStore) GetLastFinishedRound(matchID string) (*models.Round, error) {
	ctx := context.Background()
	var idx map[string]fbRoundIdx
	if err := s.ref("idx_round_match/"+matchID).Get(ctx, &idx); err != nil {
		return nil, err
	}
	bestID := ""
	bestNum := -1
	for id, ri := range idx {
		if ri.Status == "finished" && ri.RoundNumber > bestNum {
			bestNum = ri.RoundNumber
			bestID = id
		}
	}
	if bestID == "" {
		return nil, errNotFound
	}
	return s.GetRound(bestID)
}

func (s *FirebaseStore) GetCurrentRound(matchID string) (*models.Round, error) {
	ctx := context.Background()
	var idx map[string]fbRoundIdx
	if err := s.ref("idx_round_match/"+matchID).Get(ctx, &idx); err != nil {
		return nil, err
	}
	// Find active round with highest number
	bestID := ""
	bestNum := -1
	for id, ri := range idx {
		if ri.Status == "active" && ri.RoundNumber > bestNum {
			bestNum = ri.RoundNumber
			bestID = id
		}
	}
	if bestID == "" {
		return nil, errNotFound
	}
	return s.GetRound(bestID)
}

func (s *FirebaseStore) GetRound(roundID string) (*models.Round, error) {
	ctx := context.Background()
	var r fbRound
	if err := s.ref("rounds/"+roundID).Get(ctx, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, errNotFound
	}
	return r.toModel(), nil
}

func (s *FirebaseStore) SetRoundStarter(roundID, playerID string) error {
	ctx := context.Background()
	return s.ref("rounds/"+roundID).Update(ctx, map[string]interface{}{"starter_player_id": playerID})
}

func (s *FirebaseStore) CountRounds(matchID string) (int, error) {
	ctx := context.Background()
	var idx map[string]fbRoundIdx
	if err := s.ref("idx_round_match/"+matchID).Get(ctx, &idx); err != nil {
		return 0, err
	}
	return len(idx), nil
}

func (s *FirebaseStore) SetRoundWinner(roundID, playerID string) error {
	ctx := context.Background()
	return s.ref("rounds/"+roundID).Update(ctx, map[string]interface{}{"winner_player_id": playerID})
}

func (s *FirebaseStore) FinishRound(roundID string) error {
	ctx := context.Background()
	// Get matchID to update the index
	var r fbRound
	if err := s.ref("rounds/"+roundID).Get(ctx, &r); err != nil {
		return err
	}
	if err := s.ref("rounds/"+roundID).Update(ctx, map[string]interface{}{"status": "finished"}); err != nil {
		return err
	}
	if r.MatchID != "" {
		_ = s.ref("idx_round_match/"+r.MatchID+"/"+roundID+"/status").Set(ctx, "finished")
	}
	return nil
}

func (s *FirebaseStore) SetTableImage(roundID, imagePath, tilesJSON string) error {
	ctx := context.Background()
	return s.ref("rounds/"+roundID).Update(ctx, map[string]interface{}{
		"table_image_path":     imagePath,
		"table_detected_tiles": tilesJSON,
	})
}

func (s *FirebaseStore) GetRoundTableTiles(roundID string) (string, string, error) {
	ctx := context.Background()
	var r fbRound
	if err := s.ref("rounds/"+roundID).Get(ctx, &r); err != nil {
		return "", "", err
	}
	tiles := r.TableDetectedTiles
	if tiles == "" {
		tiles = "[]"
	}
	return r.TableImagePath, tiles, nil
}

// ─── HandImage ───────────────────────────────────────────────────────────────

func (s *FirebaseStore) CreateHandImage(id, roundID, playerID, imagePath string) error {
	ctx := context.Background()
	h := fbHandImage{
		ID: id, RoundID: roundID, PlayerID: playerID, ImagePath: imagePath,
		PointsCalculated: 0, Confirmed: 0, DetectedTilesJSON: "[]", CreatedAt: nowStr(),
	}
	if err := s.ref("hand_images/"+id).Set(ctx, h); err != nil {
		return err
	}
	// Index round → hand images
	if err := s.ref("idx_hand_round/"+roundID+"/"+id).Set(ctx, true); err != nil {
		return err
	}
	// Index round+player → hand image id
	return s.ref("idx_hand_round_player/"+roundID+"/"+safeKey(playerID)).Set(ctx, id)
}

func (s *FirebaseStore) GetHandImage(imageID string) (*models.HandImage, error) {
	ctx := context.Background()
	var h fbHandImage
	if err := s.ref("hand_images/"+imageID).Get(ctx, &h); err != nil {
		return nil, err
	}
	if h.ID == "" {
		return nil, errNotFound
	}
	m := h.toModel()
	return &m, nil
}

func (s *FirebaseStore) GetHandImageByRoundAndPlayer(roundID, playerID string) (*models.HandImage, error) {
	ctx := context.Background()
	var imageID string
	if err := s.ref("idx_hand_round_player/"+roundID+"/"+safeKey(playerID)).Get(ctx, &imageID); err != nil {
		return nil, err
	}
	if imageID == "" {
		return nil, errNotFound
	}
	return s.GetHandImage(imageID)
}

func (s *FirebaseStore) UpdateHandImagePoints(imageID string, points int, confirmed bool, tilesJSON string) error {
	ctx := context.Background()
	c := 0
	if confirmed {
		c = 1
	}
	return s.ref("hand_images/"+imageID).Update(ctx, map[string]interface{}{
		"points_calculated": points,
		"confirmed":         c,
		"detected_tiles":    tilesJSON,
	})
}

func (s *FirebaseStore) GetHandImages(roundID string) ([]models.HandImage, error) {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_hand_round/"+roundID).Get(ctx, &ids); err != nil {
		return nil, err
	}
	var images []models.HandImage
	for id := range ids {
		var h fbHandImage
		if err := s.ref("hand_images/"+id).Get(ctx, &h); err == nil && h.ID != "" {
			images = append(images, h.toModel())
		}
	}
	return images, nil
}

// ─── Tournament ───────────────────────────────────────────────────────────────

func (s *FirebaseStore) CreateTournament(id, name, baseURL string) error {
	ctx := context.Background()
	t := fbTournament{ID: id, Name: name, Status: "registration", BaseURL: baseURL, CreatedAt: nowStr()}
	return s.ref("tournaments/"+id).Set(ctx, t)
}

func (s *FirebaseStore) GetTournament(id string) (*models.Tournament, error) {
	ctx := context.Background()
	var t fbTournament
	if err := s.ref("tournaments/"+id).Get(ctx, &t); err != nil {
		return nil, err
	}
	if t.ID == "" {
		return nil, errNotFound
	}
	return t.toModel(), nil
}

func (s *FirebaseStore) UpdateTournamentStatus(id, status string) error {
	ctx := context.Background()
	return s.ref("tournaments/"+id).Update(ctx, map[string]interface{}{"status": status})
}

func (s *FirebaseStore) CreateTournamentPlayer(id, tournamentID, name, uniqueID string) error {
	ctx := context.Background()
	p := fbTournamentPlayer{
		ID: id, TournamentID: tournamentID, Name: name, UniqueIdentifier: uniqueID,
		TableNumber: 0, MatchID: "", CreatedAt: nowStr(),
	}
	if err := s.ref("tournament_players/"+id).Set(ctx, p); err != nil {
		return err
	}
	return s.ref("idx_tp_tournament/"+tournamentID+"/"+id).Set(ctx, true)
}

func (s *FirebaseStore) GetTournamentPlayers(tournamentID string) ([]models.TournamentPlayer, error) {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_tp_tournament/"+tournamentID).Get(ctx, &ids); err != nil {
		return nil, err
	}
	var players []models.TournamentPlayer
	for id := range ids {
		var p fbTournamentPlayer
		if err := s.ref("tournament_players/"+id).Get(ctx, &p); err == nil && p.ID != "" {
			players = append(players, p.toModel())
		}
	}
	sort.Slice(players, func(i, j int) bool {
		return players[i].CreatedAt.Before(players[j].CreatedAt)
	})
	return players, nil
}

func (s *FirebaseStore) GetTournamentPlayerByUniqueID(tournamentID, uniqueID string) (*models.TournamentPlayer, error) {
	players, err := s.GetTournamentPlayers(tournamentID)
	if err != nil {
		return nil, err
	}
	for i := range players {
		if players[i].UniqueIdentifier == uniqueID {
			return &players[i], nil
		}
	}
	return nil, errNotFound
}

func (s *FirebaseStore) CountTournamentPlayers(tournamentID string) (int, error) {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_tp_tournament/"+tournamentID).Get(ctx, &ids); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *FirebaseStore) AssignTournamentPlayer(playerID string, tableNum int, matchID string) error {
	ctx := context.Background()
	return s.ref("tournament_players/"+playerID).Update(ctx, map[string]interface{}{
		"table_number": tableNum,
		"match_id":     matchID,
	})
}

type fbTournamentMatch struct {
	TournamentID string `json:"tournament_id"`
	MatchID      string `json:"match_id"`
	TableNumber  int    `json:"table_number"`
}

func (s *FirebaseStore) CreateTournamentMatch(tournamentID, matchID string, tableNum int) error {
	ctx := context.Background()
	tm := fbTournamentMatch{TournamentID: tournamentID, MatchID: matchID, TableNumber: tableNum}
	key := safeKey(tournamentID + "_" + matchID)
	if err := s.ref("tournament_matches/"+key).Set(ctx, tm); err != nil {
		return err
	}
	return s.ref("idx_tm_tournament/"+tournamentID+"/"+safeKey(matchID)).Set(ctx, tableNum)
}

func (s *FirebaseStore) GetTournamentMatches(tournamentID string) ([]models.TournamentMatch, error) {
	ctx := context.Background()
	var tableNums map[string]int
	if err := s.ref("idx_tm_tournament/"+tournamentID).Get(ctx, &tableNums); err != nil {
		return nil, err
	}
	var matches []models.TournamentMatch
	for matchID, tableNum := range tableNums {
		matches = append(matches, models.TournamentMatch{
			TournamentID: tournamentID,
			MatchID:      matchID,
			TableNumber:  tableNum,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].TableNumber < matches[j].TableNumber
	})
	return matches, nil
}

func (s *FirebaseStore) GetTournamentRanking(tournamentID string) ([]models.TournamentRankEntry, error) {
	tPlayers, err := s.GetTournamentPlayers(tournamentID)
	if err != nil {
		return nil, err
	}
	var entries []models.TournamentRankEntry
	for _, tp := range tPlayers {
		entry := models.TournamentRankEntry{
			UniqueIdentifier: tp.UniqueIdentifier,
			Name:             tp.Name,
			TableNumber:      tp.TableNumber,
			MatchID:          tp.MatchID,
			Score:            0,
			Status:           "active",
		}
		if tp.MatchID != "" {
			if p, err := s.GetPlayerByUniqueID(tp.MatchID, tp.UniqueIdentifier); err == nil {
				entry.Score = p.TotalScore
				entry.Status = p.Status
			}
		}
		if entry.Score < 0 {
			entry.Score = 0
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score < entries[j].Score
	})
	return entries, nil
}

// ─── Global Stats ─────────────────────────────────────────────────────────────

func (s *FirebaseStore) getAllPlayers() ([]models.Player, error) {
	ctx := context.Background()
	var raw map[string]fbPlayer
	if err := s.ref("players").Get(ctx, &raw); err != nil {
		return nil, err
	}
	var players []models.Player
	for _, p := range raw {
		players = append(players, p.toModel())
	}
	return players, nil
}

func (s *FirebaseStore) getAllMatches() ([]models.Match, error) {
	ctx := context.Background()
	var raw map[string]fbMatch
	if err := s.ref("matches").Get(ctx, &raw); err != nil {
		return nil, err
	}
	var matches []models.Match
	for _, m := range raw {
		matches = append(matches, *m.toModel())
	}
	return matches, nil
}

func (s *FirebaseStore) getAllRounds() ([]models.Round, error) {
	ctx := context.Background()
	var raw map[string]fbRound
	if err := s.ref("rounds").Get(ctx, &raw); err != nil {
		return nil, err
	}
	var rounds []models.Round
	for _, r := range raw {
		rounds = append(rounds, *r.toModel())
	}
	return rounds, nil
}

func (s *FirebaseStore) getAllHandImages() ([]models.HandImage, error) {
	ctx := context.Background()
	var raw map[string]fbHandImage
	if err := s.ref("hand_images").Get(ctx, &raw); err != nil {
		return nil, err
	}
	var images []models.HandImage
	for _, h := range raw {
		images = append(images, h.toModel())
	}
	return images, nil
}

func (s *FirebaseStore) GetGlobalStats() ([]models.GlobalStat, error) {
	players, err := s.getAllPlayers()
	if err != nil {
		return nil, err
	}
	matches, err := s.getAllMatches()
	if err != nil {
		return nil, err
	}
	rounds, err := s.getAllRounds()
	if err != nil {
		return nil, err
	}

	// matchWinners: playerID → true se o match tem winner_player_id == playerID
	matchWinByUID := map[string]int{}
	for _, m := range matches {
		if m.WinnerPlayerID == "" {
			continue
		}
		// We need the uniqueIdentifier of the winner
		for _, p := range players {
			if p.ID == m.WinnerPlayerID {
				matchWinByUID[p.UniqueIdentifier]++
				break
			}
		}
	}

	roundWinByUID := map[string]int{}
	for _, r := range rounds {
		if r.WinnerPlayerID == "" {
			continue
		}
		for _, p := range players {
			if p.ID == r.WinnerPlayerID {
				roundWinByUID[p.UniqueIdentifier]++
				break
			}
		}
	}

	type aggregate struct {
		Name          string
		MatchesPlayed map[string]bool
		BustCount     int
		MaxBustScore  int
	}
	agg := map[string]*aggregate{}

	for _, p := range players {
		uid := p.UniqueIdentifier
		a, ok := agg[uid]
		if !ok {
			a = &aggregate{MatchesPlayed: map[string]bool{}}
			agg[uid] = a
		}
		a.Name = p.Name
		a.MatchesPlayed[p.MatchID] = true
		if p.Status == "estourou" {
			a.BustCount++
			if p.TotalScore > a.MaxBustScore {
				a.MaxBustScore = p.TotalScore
			}
		}
	}

	var stats []models.GlobalStat
	for uid, a := range agg {
		stats = append(stats, models.GlobalStat{
			UniqueIdentifier: uid,
			Name:             a.Name,
			MatchesPlayed:    len(a.MatchesPlayed),
			MatchesWon:       matchWinByUID[uid],
			BustCount:        a.BustCount,
			MaxBustScore:     a.MaxBustScore,
			TotalRoundWins:   roundWinByUID[uid],
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].MatchesWon != stats[j].MatchesWon {
			return stats[i].MatchesWon > stats[j].MatchesWon
		}
		if stats[i].TotalRoundWins != stats[j].TotalRoundWins {
			return stats[i].TotalRoundWins > stats[j].TotalRoundWins
		}
		return stats[i].BustCount < stats[j].BustCount
	})
	return stats, nil
}

func (s *FirebaseStore) GetMostRoundsLost() ([]models.ZoeiraStat, error) {
	handImages, err := s.getAllHandImages()
	if err != nil {
		return nil, err
	}
	players, err := s.getAllPlayers()
	if err != nil {
		return nil, err
	}
	rounds, err := s.getAllRounds()
	if err != nil {
		return nil, err
	}

	roundWinner := map[string]string{}
	for _, r := range rounds {
		if r.WinnerPlayerID != "" {
			roundWinner[r.ID] = r.WinnerPlayerID
		}
	}
	playerUID := map[string]string{}
	playerName := map[string]string{}
	for _, p := range players {
		playerUID[p.ID] = p.UniqueIdentifier
		playerName[p.UniqueIdentifier] = p.Name
	}

	cnt := map[string]int{}
	for _, hi := range handImages {
		if hi.Confirmed != 1 {
			continue
		}
		winner, ok := roundWinner[hi.RoundID]
		if !ok || winner == "" || winner == hi.PlayerID {
			continue
		}
		uid := playerUID[hi.PlayerID]
		if uid != "" {
			cnt[uid]++
		}
	}
	return buildZoeiraStats(cnt, playerName, 10), nil
}

func (s *FirebaseStore) GetPintoKings() ([]models.ZoeiraStat, error) {
	handImages, err := s.getAllHandImages()
	if err != nil {
		return nil, err
	}
	players, err := s.getAllPlayers()
	if err != nil {
		return nil, err
	}
	playerUID := map[string]string{}
	playerName := map[string]string{}
	for _, p := range players {
		playerUID[p.ID] = p.UniqueIdentifier
		playerName[p.UniqueIdentifier] = p.Name
	}
	cnt := map[string]int{}
	for _, hi := range handImages {
		if hi.Confirmed != 1 {
			continue
		}
		if strings.Contains(hi.DetectedTilesJSON, `"0-1"`) {
			uid := playerUID[hi.PlayerID]
			if uid != "" {
				cnt[uid]++
			}
		}
	}
	return buildZoeiraStats(cnt, playerName, 10), nil
}

func (s *FirebaseStore) GetBrancoKings() ([]models.ZoeiraStat, error) {
	handImages, err := s.getAllHandImages()
	if err != nil {
		return nil, err
	}
	players, err := s.getAllPlayers()
	if err != nil {
		return nil, err
	}
	playerUID := map[string]string{}
	playerName := map[string]string{}
	for _, p := range players {
		playerUID[p.ID] = p.UniqueIdentifier
		playerName[p.UniqueIdentifier] = p.Name
	}
	cnt := map[string]int{}
	for _, hi := range handImages {
		if hi.Confirmed == 1 && hi.DetectedTilesJSON == `["0-0"]` {
			uid := playerUID[hi.PlayerID]
			if uid != "" {
				cnt[uid]++
			}
		}
	}
	return buildZoeiraStats(cnt, playerName, 10), nil
}

func (s *FirebaseStore) GetCloseCallKings() ([]models.ZoeiraStat, error) {
	players, err := s.getAllPlayers()
	if err != nil {
		return nil, err
	}
	playerName := map[string]string{}
	for _, p := range players {
		playerName[p.UniqueIdentifier] = p.Name
	}
	cnt := map[string]int{}
	for _, p := range players {
		if p.TotalScore >= 45 && p.Status == "active" {
			cnt[p.UniqueIdentifier]++
		}
	}
	return buildZoeiraStats(cnt, playerName, 10), nil
}

func buildZoeiraStats(cnt map[string]int, names map[string]string, limit int) []models.ZoeiraStat {
	var stats []models.ZoeiraStat
	for uid, c := range cnt {
		stats = append(stats, models.ZoeiraStat{
			UniqueIdentifier: uid,
			Name:             names[uid],
			Count:            c,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})
	if len(stats) > limit {
		stats = stats[:limit]
	}
	return stats
}

// ─── Nicknames ────────────────────────────────────────────────────────────────

func (s *FirebaseStore) CreateNomination(id, nominatedUID, nominatedName, matchID, nickname, proposerUID string) error {
	ctx := context.Background()
	n := fbNicknameNomination{
		ID: id, NominatedUniqueID: nominatedUID, NominatedName: nominatedName,
		MatchID: matchID, Nickname: nickname, VoteCount: 1,
		ProposerUniqueID: proposerUID, CreatedAt: nowStr(),
	}
	if err := s.ref("nickname_nominations/"+id).Set(ctx, n); err != nil {
		return err
	}
	if err := s.ref("idx_nom_match/"+matchID+"/"+id).Set(ctx, true); err != nil {
		return err
	}
	return s.ref("idx_nom_uid/"+safeKey(nominatedUID)+"/"+id).Set(ctx, true)
}

func (s *FirebaseStore) VoteForNomination(nominationID, voterUID string) (bool, error) {
	ctx := context.Background()
	voteKey := safeKey(voterUID)
	voteRef := s.ref("idx_vote/" + nominationID + "/" + voteKey)

	var existing bool
	_ = voteRef.Get(ctx, &existing)
	if existing {
		return false, nil // already voted
	}

	if err := voteRef.Set(ctx, true); err != nil {
		return false, err
	}

	// Atomically increment the vote count
	err := s.ref("nickname_nominations/"+nominationID+"/vote_count").Transaction(ctx, func(node fbdb.TransactionNode) (interface{}, error) {
		var current int
		_ = node.Unmarshal(&current)
		return current + 1, nil
	})
	return true, err
}

func (s *FirebaseStore) GetNominationsForMatch(matchID string) ([]models.NicknameNomination, error) {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_nom_match/"+matchID).Get(ctx, &ids); err != nil {
		return nil, err
	}
	var noms []models.NicknameNomination
	for id := range ids {
		var n fbNicknameNomination
		if err := s.ref("nickname_nominations/"+id).Get(ctx, &n); err == nil && n.ID != "" {
			noms = append(noms, n.toModel())
		}
	}
	sort.Slice(noms, func(i, j int) bool {
		if noms[i].VoteCount != noms[j].VoteCount {
			return noms[i].VoteCount > noms[j].VoteCount
		}
		return noms[i].CreatedAt.Before(noms[j].CreatedAt)
	})
	return noms, nil
}

func (s *FirebaseStore) GetNominationsForPlayer(matchID, nominatedUID string) ([]models.NicknameNomination, error) {
	all, err := s.GetNominationsForMatch(matchID)
	if err != nil {
		return nil, err
	}
	var result []models.NicknameNomination
	for _, n := range all {
		if n.NominatedUniqueID == nominatedUID {
			result = append(result, n)
		}
	}
	return result, nil
}

func (s *FirebaseStore) GetTopNicknameForPlayer(uniqueID string) string {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_nom_uid/"+safeKey(uniqueID)).Get(ctx, &ids); err != nil || len(ids) == 0 {
		return ""
	}
	var best fbNicknameNomination
	for id := range ids {
		var n fbNicknameNomination
		if err := s.ref("nickname_nominations/"+id).Get(ctx, &n); err == nil && n.ID != "" {
			if n.VoteCount > best.VoteCount {
				best = n
			}
		}
	}
	return best.Nickname
}

func (s *FirebaseStore) GetAllTimeNicknames() ([]models.NicknameNomination, error) {
	ctx := context.Background()
	var raw map[string]fbNicknameNomination
	if err := s.ref("nickname_nominations").Get(ctx, &raw); err != nil {
		return nil, err
	}
	// Agrupa por nominated_unique_id, pega a de maior vote_count
	best := map[string]fbNicknameNomination{}
	for _, n := range raw {
		if n.VoteCount < 2 {
			continue
		}
		prev, ok := best[n.NominatedUniqueID]
		if !ok || n.VoteCount > prev.VoteCount {
			best[n.NominatedUniqueID] = n
		}
	}
	var result []models.NicknameNomination
	for _, n := range best {
		result = append(result, n.toModel())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].VoteCount > result[j].VoteCount
	})
	if len(result) > 20 {
		result = result[:20]
	}
	return result, nil
}

// ─── Turma ────────────────────────────────────────────────────────────────────

type fbTurma struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	InviteCode        string `json:"invite_code"`
	IsPrivate         bool   `json:"is_private"`
	CreatedByUniqueID string `json:"created_by_unique_id"`
	BaseURL           string `json:"base_url"`
	CreatedAt         string `json:"created_at"`
}

func (f *fbTurma) toModel() *models.Turma {
	t, _ := time.Parse(time.RFC3339, f.CreatedAt)
	return &models.Turma{
		ID: f.ID, Name: f.Name, Description: f.Description,
		InviteCode: f.InviteCode, IsPrivate: f.IsPrivate,
		CreatedByUniqueID: f.CreatedByUniqueID, BaseURL: f.BaseURL, CreatedAt: t,
	}
}

type fbTurmaMember struct {
	ID               string `json:"id"`
	TurmaID          string `json:"turma_id"`
	UniqueIdentifier string `json:"unique_identifier"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	JoinedAt         string `json:"joined_at"`
}

func (f *fbTurmaMember) toModel() models.TurmaMember {
	t, _ := time.Parse(time.RFC3339, f.JoinedAt)
	return models.TurmaMember{
		ID: f.ID, TurmaID: f.TurmaID, UniqueIdentifier: f.UniqueIdentifier,
		Name: f.Name, Role: f.Role, JoinedAt: t,
	}
}

func (s *FirebaseStore) CreateTurma(turma *models.Turma) error {
	ctx := context.Background()
	fb := fbTurma{
		ID: turma.ID, Name: turma.Name, Description: turma.Description,
		InviteCode: turma.InviteCode, IsPrivate: turma.IsPrivate,
		CreatedByUniqueID: turma.CreatedByUniqueID, BaseURL: turma.BaseURL,
		CreatedAt: nowStr(),
	}
	if err := s.ref("turmas/"+turma.ID).Set(ctx, fb); err != nil {
		return err
	}
	// Index invite code → turma id
	return s.ref("idx_turma_code/"+safeKey(turma.InviteCode)).Set(ctx, turma.ID)
}

func (s *FirebaseStore) GetTurma(id string) (*models.Turma, error) {
	ctx := context.Background()
	var t fbTurma
	if err := s.ref("turmas/"+id).Get(ctx, &t); err != nil {
		return nil, err
	}
	if t.ID == "" {
		return nil, errNotFound
	}
	return t.toModel(), nil
}

func (s *FirebaseStore) GetTurmaByInviteCode(code string) (*models.Turma, error) {
	ctx := context.Background()
	var turmaID string
	if err := s.ref("idx_turma_code/"+safeKey(code)).Get(ctx, &turmaID); err != nil {
		return nil, err
	}
	if turmaID == "" {
		return nil, errNotFound
	}
	return s.GetTurma(turmaID)
}

func (s *FirebaseStore) AddTurmaMember(member *models.TurmaMember) error {
	ctx := context.Background()
	fb := fbTurmaMember{
		ID: member.ID, TurmaID: member.TurmaID, UniqueIdentifier: member.UniqueIdentifier,
		Name: member.Name, Role: member.Role, JoinedAt: nowStr(),
	}
	if err := s.ref("turma_members/"+member.ID).Set(ctx, fb); err != nil {
		return err
	}
	// Index turma → members
	if err := s.ref("idx_turma_members/"+member.TurmaID+"/"+member.ID).Set(ctx, true); err != nil {
		return err
	}
	// Index uid → turmas
	return s.ref("idx_member_turmas/"+safeKey(member.UniqueIdentifier)+"/"+member.TurmaID).Set(ctx, true)
}

func (s *FirebaseStore) GetTurmaMembers(turmaID string) ([]models.TurmaMember, error) {
	ctx := context.Background()
	var ids map[string]bool
	if err := s.ref("idx_turma_members/"+turmaID).Get(ctx, &ids); err != nil {
		return nil, err
	}
	var members []models.TurmaMember
	for id := range ids {
		var m fbTurmaMember
		if err := s.ref("turma_members/"+id).Get(ctx, &m); err == nil && m.ID != "" {
			members = append(members, m.toModel())
		}
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].JoinedAt.Before(members[j].JoinedAt)
	})
	return members, nil
}

func (s *FirebaseStore) GetTurmaMember(turmaID, uniqueID string) (*models.TurmaMember, error) {
	members, err := s.GetTurmaMembers(turmaID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].UniqueIdentifier == uniqueID {
			return &members[i], nil
		}
	}
	return nil, errNotFound
}

func (s *FirebaseStore) RemoveTurmaMember(turmaID, uniqueID string) error {
	member, err := s.GetTurmaMember(turmaID, uniqueID)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := s.ref("turma_members/" + member.ID).Delete(ctx); err != nil {
		return err
	}
	if err := s.ref("idx_turma_members/" + turmaID + "/" + member.ID).Delete(ctx); err != nil {
		return err
	}
	return s.ref("idx_member_turmas/" + safeKey(uniqueID) + "/" + turmaID).Delete(ctx)
}

func (s *FirebaseStore) IsTurmaMember(turmaID, uniqueID string) (bool, error) {
	_, err := s.GetTurmaMember(turmaID, uniqueID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *FirebaseStore) GetTurmasByMember(uniqueID string) ([]models.Turma, error) {
	ctx := context.Background()
	var turmaIDs map[string]bool
	if err := s.ref("idx_member_turmas/"+safeKey(uniqueID)).Get(ctx, &turmaIDs); err != nil {
		return nil, err
	}
	var turmas []models.Turma
	for tid := range turmaIDs {
		if t, err := s.GetTurma(tid); err == nil {
			turmas = append(turmas, *t)
		}
	}
	sort.Slice(turmas, func(i, j int) bool {
		return turmas[i].CreatedAt.After(turmas[j].CreatedAt)
	})
	return turmas, nil
}

func (s *FirebaseStore) GetTurmaMatches(turmaID string) ([]models.Match, error) {
	allMatches, err := s.getAllMatches()
	if err != nil {
		return nil, err
	}
	var result []models.Match
	for _, m := range allMatches {
		if m.TurmaID == turmaID {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *FirebaseStore) CreateMatchInTurma(id, baseURL, turmaID string) error {
	ctx := context.Background()
	m := fbMatch{ID: id, Status: "waiting", BaseURL: baseURL, TurmaID: turmaID, CreatedAt: nowStr()}
	return s.ref("matches/"+id).Set(ctx, m)
}

func (s *FirebaseStore) GetTurmaRanking(turmaID string) ([]models.TurmaRankEntry, error) {
	players, err := s.getAllPlayers()
	if err != nil {
		return nil, err
	}
	allMatches, err := s.getAllMatches()
	if err != nil {
		return nil, err
	}
	rounds, err := s.getAllRounds()
	if err != nil {
		return nil, err
	}

	// Build set of match IDs belonging to this turma
	turmaMatchIDs := map[string]bool{}
	for _, m := range allMatches {
		if m.TurmaID == turmaID {
			turmaMatchIDs[m.ID] = true
		}
	}

	// Match win map
	matchWinByUID := map[string]int{}
	for _, m := range allMatches {
		if !turmaMatchIDs[m.ID] || m.WinnerPlayerID == "" {
			continue
		}
		for _, p := range players {
			if p.ID == m.WinnerPlayerID {
				matchWinByUID[p.UniqueIdentifier]++
				break
			}
		}
	}

	// Round win map
	roundWinByUID := map[string]int{}
	for _, r := range rounds {
		if !turmaMatchIDs[r.MatchID] || r.WinnerPlayerID == "" {
			continue
		}
		for _, p := range players {
			if p.ID == r.WinnerPlayerID {
				roundWinByUID[p.UniqueIdentifier]++
				break
			}
		}
	}

	type aggregate struct {
		Name          string
		MatchesPlayed map[string]bool
		TotalScore    int
		BustCount     int
	}
	agg := map[string]*aggregate{}
	for _, p := range players {
		if !turmaMatchIDs[p.MatchID] {
			continue
		}
		uid := p.UniqueIdentifier
		a, ok := agg[uid]
		if !ok {
			a = &aggregate{MatchesPlayed: map[string]bool{}}
			agg[uid] = a
		}
		a.Name = p.Name
		a.MatchesPlayed[p.MatchID] = true
		a.TotalScore += p.TotalScore
		if p.Status == "estourou" {
			a.BustCount++
		}
	}

	var entries []models.TurmaRankEntry
	for uid, a := range agg {
		entries = append(entries, models.TurmaRankEntry{
			UniqueIdentifier: uid,
			Name:             a.Name,
			MatchesPlayed:    len(a.MatchesPlayed),
			MatchesWon:       matchWinByUID[uid],
			TotalScore:       a.TotalScore,
			BustCount:        a.BustCount,
			TotalRoundWins:   roundWinByUID[uid],
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].MatchesWon != entries[j].MatchesWon {
			return entries[i].MatchesWon > entries[j].MatchesWon
		}
		if entries[i].TotalRoundWins != entries[j].TotalRoundWins {
			return entries[i].TotalRoundWins > entries[j].TotalRoundWins
		}
		return entries[i].BustCount < entries[j].BustCount
	})
	return entries, nil
}
