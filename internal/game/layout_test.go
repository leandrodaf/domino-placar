package game

import (
	"testing"
)

func TestFirstTilePlacedAtCenter(t *testing.T) {
	bl := NewBoardLayout(20, 10, 1)
	tile := Tile{High: 6, Low: 4}
	res := bl.CalculatePosition(tile, East)

	// Normal tile: 2 cols wide, center of 20 cols → x = 9
	if res.X != 9 {
		t.Errorf("expected X=9, got %d", res.X)
	}
	// 1 row tall, center of 10 rows → y = 4
	if res.Y != 4 {
		t.Errorf("expected Y=4, got %d", res.Y)
	}
	if res.Direction != East {
		t.Errorf("expected East, got %v", res.Direction)
	}
	if len(bl.Tiles) != 1 {
		t.Fatalf("expected 1 tile on board, got %d", len(bl.Tiles))
	}
}

func TestDoubleTilePlacedAtCenter(t *testing.T) {
	bl := NewBoardLayout(20, 10, 1)
	tile := Tile{High: 6, Low: 6}
	res := bl.CalculatePosition(tile, East)

	// Double: 1 col wide × 2 rows tall, center of 20 → x = 9
	if res.X != 9 {
		t.Errorf("expected X=9, got %d", res.X)
	}
	if res.Width != 1 {
		t.Errorf("expected Width=1, got %d", res.Width)
	}
	if res.Height != 2 {
		t.Errorf("expected Height=2, got %d", res.Height)
	}
}

func TestStraightEastPlacement(t *testing.T) {
	bl := NewBoardLayout(20, 10, 1)

	// Place first tile
	bl.CalculatePosition(Tile{High: 6, Low: 4}, East)

	// Place second tile going East
	res := bl.CalculatePosition(Tile{High: 4, Low: 3}, East)
	// Should be right after the first tile: x = 9 + 2 = 11
	if res.X != 11 {
		t.Errorf("expected X=11, got %d", res.X)
	}
	if res.Y != 4 {
		t.Errorf("expected Y=4, got %d", res.Y)
	}
	if res.IsCurve {
		t.Error("should not be a curve")
	}
}

func TestForcedCurveAtEastBorder(t *testing.T) {
	// Small board: 10 cols, padding=1, safe zone is cols 1..8
	bl := NewBoardLayout(10, 10, 1)

	dir := East
	// Place tiles going East until a curve is forced.
	tiles := []Tile{
		{6, 4}, {4, 3}, {3, 2}, {2, 1},
	}

	var lastRes PlacementResult
	for _, tile := range tiles {
		lastRes = bl.CalculatePosition(tile, dir)
		dir = lastRes.Direction
	}

	// At some point a curve should have been triggered.
	foundCurve := false
	for _, lt := range bl.Tiles {
		if lt.IsCurve {
			foundCurve = true
			break
		}
	}
	if !foundCurve {
		t.Error("expected at least one curve when approaching East border")
	}

	// After curve, direction should have flipped to West.
	if lastRes.Direction != West {
		t.Errorf("expected direction West after East-border curve, got %v", lastRes.Direction)
	}
}

func TestForcedCurveAtWestBorder(t *testing.T) {
	bl := NewBoardLayout(10, 10, 1)

	dir := West
	// Start from center, go West.
	tiles := []Tile{
		{6, 4}, {4, 3}, {3, 2}, {2, 1},
	}

	var lastRes PlacementResult
	for _, tile := range tiles {
		lastRes = bl.CalculatePosition(tile, dir)
		dir = lastRes.Direction
	}

	foundCurve := false
	for _, lt := range bl.Tiles {
		if lt.IsCurve {
			foundCurve = true
			break
		}
	}
	if !foundCurve {
		t.Error("expected at least one curve when approaching West border")
	}
	if lastRes.Direction != East {
		t.Errorf("expected direction East after West-border curve, got %v", lastRes.Direction)
	}
}

func TestDoubleTileNearBorder(t *testing.T) {
	bl := NewBoardLayout(10, 10, 1)

	dir := East
	// Push several tiles East, then a double near the edge.
	tiles := []Tile{
		{6, 4}, {4, 3}, {3, 3},
	}
	var lastRes PlacementResult
	for _, tile := range tiles {
		lastRes = bl.CalculatePosition(tile, dir)
		dir = lastRes.Direction
	}

	// The double (3|3) should still be on the board and within bounds.
	last := bl.Tiles[len(bl.Tiles)-1]
	if last.X < 0 || last.X >= bl.MaxCols || last.Y < 0 || last.Y >= bl.MaxRows {
		t.Errorf("double placed out of bounds at (%d,%d)", last.X, last.Y)
	}
	_ = lastRes
}

func TestLayoutChainSerpentine(t *testing.T) {
	// 12 cols × 20 rows should allow a full snake with many tiles.
	tiles := []Tile{
		{6, 5}, {5, 4}, {4, 3}, {3, 2}, {2, 1}, // →
		{1, 0}, {0, 6}, {6, 3}, {3, 1}, {1, 5}, // ← after curve
		{5, 2}, {2, 6}, {6, 6}, {6, 1}, {1, 4}, // → after second curve
	}

	bl := LayoutChain(tiles, 12, 20, 1)

	if len(bl.Tiles) != len(tiles) {
		t.Fatalf("expected %d tiles placed, got %d", len(tiles), len(bl.Tiles))
	}

	// Verify no tile is out of bounds.
	for i, lt := range bl.Tiles {
		w := tileWidth(lt.IsDouble, lt.Direction)
		h := tileHeight(lt.IsDouble, lt.Direction)
		if lt.X < 0 || lt.Y < 0 || lt.X+w > bl.MaxCols || lt.Y+h > bl.MaxRows {
			t.Errorf("tile %d (%v) out of bounds at (%d,%d) size %dx%d",
				i, lt.Tile, lt.X, lt.Y, w, h)
		}
	}

	// Should have at least one curve.
	curves := 0
	for _, lt := range bl.Tiles {
		if lt.IsCurve {
			curves++
		}
	}
	if curves == 0 {
		t.Error("expected at least one curve in a 15-tile chain on a 12-col board")
	}
}

func TestNoOverlappingTiles(t *testing.T) {
	tiles := []Tile{
		{6, 5}, {5, 4}, {4, 3}, {3, 2}, {2, 1},
		{1, 0}, {0, 6}, {6, 3}, {3, 1}, {1, 5},
	}
	bl := LayoutChain(tiles, 12, 20, 1)

	// Build occupancy from scratch and check for double-assignments.
	occ := make(map[[2]int]int) // cell → tile index
	for i, lt := range bl.Tiles {
		w := tileWidth(lt.IsDouble, lt.Direction)
		h := tileHeight(lt.IsDouble, lt.Direction)
		for dy := 0; dy < h; dy++ {
			for dx := 0; dx < w; dx++ {
				cell := [2]int{lt.X + dx, lt.Y + dy}
				if prev, ok := occ[cell]; ok {
					t.Errorf("cell (%d,%d) claimed by tile %d and tile %d",
						cell[0], cell[1], prev, i)
				}
				occ[cell] = i
			}
		}
	}
}

func TestDirectionOpposite(t *testing.T) {
	cases := [][2]Direction{
		{East, West}, {West, East}, {South, North}, {North, South},
	}
	for _, c := range cases {
		if got := c[0].Opposite(); got != c[1] {
			t.Errorf("%v.Opposite() = %v, want %v", c[0], got, c[1])
		}
	}
}

func TestSmallBoardDoesNotPanic(t *testing.T) {
	// Minimum viable board: 4×4 with padding=0.
	bl := NewBoardLayout(4, 4, 0)
	tiles := []Tile{{6, 5}, {5, 4}, {4, 3}}
	dir := East
	for _, tile := range tiles {
		res := bl.CalculatePosition(tile, dir)
		dir = res.Direction
	}
	if len(bl.Tiles) != 3 {
		t.Errorf("expected 3 tiles, got %d", len(bl.Tiles))
	}
}
