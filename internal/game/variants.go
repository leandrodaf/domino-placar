package game

// PointMode defines how a round winner earns points.
type PointMode string

const (
	PointModeWinnerSum PointMode = "winner_sum"
	PointModeAllFives  PointMode = "all_fives"
	PointModeLowest    PointMode = "lowest"
)

// VariantRules defines the rules for one game variant.
type VariantRules struct {
	Name             string
	HasBoneyard      bool
	TilesPerPlayer   int
	PointMode        PointMode
	BlockWinByLowest bool // true = lowest hand wins block
	MaxPip           int  // 6 for standard set
}

// Variants is the registry of supported game variants.
var Variants = map[string]VariantRules{
	"pontinho": {
		Name: "Pontinho", HasBoneyard: false, TilesPerPlayer: 7,
		PointMode: PointModeWinnerSum, BlockWinByLowest: true, MaxPip: 6,
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
