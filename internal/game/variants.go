package game

// PointMode defines how a round winner earns points.
type PointMode string

const (
	// PointModeWinnerSum: round winner earns the sum of all opponents' hands.
	PointModeWinnerSum PointMode = "winner_sum"
	// PointModeAllFives: winner earns rounded-to-5 sum of opponents' hands.
	PointModeAllFives PointMode = "all_fives"
	// PointModeLowest: used in blocked rounds — lowest hand wins; PointsAwarded = total.
	PointModeLowest PointMode = "lowest"
	// PointModeLosersPay: each non-winner adds their OWN hand value to their score.
	// Used by Pontinho: bad to accumulate points; reach MaxPoints → eliminated.
	PointModeLosersPay PointMode = "losers_pay"
)

// VariantRules defines the rules for one game variant.
type VariantRules struct {
	Name             string
	HasBoneyard      bool
	TilesPerPlayer   int
	PointMode        PointMode
	BlockWinByLowest bool // true = lowest hand wins block
	MaxPip           int  // 6 for standard set
	BlankBlankBonus  bool // true = the 0|0 tile counts as 12 when it's the only tile in hand
}

// Variants is the registry of supported game variants.
var Variants = map[string]VariantRules{
	"pontinho": {
		Name: "Pontinho", HasBoneyard: true, TilesPerPlayer: 7,
		PointMode: PointModeLosersPay, BlockWinByLowest: true, MaxPip: 6,
		BlankBlankBonus: true,
	},
	"bloqueio": {
		Name: "Bloqueio Clássico", HasBoneyard: false, TilesPerPlayer: 7,
		PointMode: PointModeLowest, BlockWinByLowest: true, MaxPip: 6,
	},
	"all_fives": {
		Name: "All Fives (Muggins)", HasBoneyard: true, TilesPerPlayer: 5,
		PointMode: PointModeAllFives, BlockWinByLowest: true, MaxPip: 6,
	},
	"com_pedra": {
		Name: "Com Pedra do Monte", HasBoneyard: true, TilesPerPlayer: 7,
		PointMode: PointModeWinnerSum, BlockWinByLowest: true, MaxPip: 6,
	},
}

// GetVariant returns variant rules, defaulting to pontinho.
func GetVariant(name string) VariantRules {
	if v, ok := Variants[name]; ok {
		return v
	}
	return Variants["pontinho"]
}
