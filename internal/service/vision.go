package service

// ApplySpecialRules applies Pontinho special rules after confirmation:
//   - Blank tile alone (only "0-0" in hand) → worth 12 points.
//   - Returns the final points to use.
func ApplySpecialRules(tiles []string, points int) int {
	if len(tiles) == 1 && tiles[0] == "0-0" {
		return 12
	}
	return points
}
