package game

import (
	"fmt"
	"math"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func testBoard() BoardConfig {
	return BoardConfig{
		ScreenWidth:  800,
		ScreenHeight: 600,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}
}

func tallTestBoard() BoardConfig {
	return BoardConfig{
		ScreenWidth:  800,
		ScreenHeight: 1600,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}
}

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 0.5 }

func normalTile() Tile { return Tile{High: 5, Low: 3} }
func doubleTile() Tile { return Tile{High: 4, Low: 4} }

// ──────────────────────────────────────────────────────────────────────────────
// FirstRenderedTile
// ──────────────────────────────────────────────────────────────────────────────

func TestFirstTileAtCenter(t *testing.T) {
	b := testBoard()
	rt := FirstRenderedTile(b, normalTile())

	if !almostEqual(rt.X, 400) {
		t.Errorf("X: want 400, got %.1f", rt.X)
	}
	if !almostEqual(rt.Y, 300) {
		t.Errorf("Y: want 300, got %.1f", rt.Y)
	}
	if rt.Rotation != 0 {
		t.Errorf("Rotation: want 0, got %d", rt.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Straight East / West
// ──────────────────────────────────────────────────────────────────────────────

func TestStraightEastNormalTile(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	next, dir := CalculateNextPosition(b, first, normalTile(), DirEast, true)

	if dir != DirEast {
		t.Errorf("direction: want E, got %s", dir)
	}
	if !almostEqual(next.X, 460) {
		t.Errorf("X: want 460, got %.1f", next.X)
	}
	if !almostEqual(next.Y, 300) {
		t.Errorf("Y: want 300, got %.1f", next.Y)
	}
	if next.Rotation != 0 {
		t.Errorf("Rotation: want 0, got %d", next.Rotation)
	}
}

func TestStraightWestNormalTile(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	next, dir := CalculateNextPosition(b, first, normalTile(), DirWest, false)

	if dir != DirWest {
		t.Errorf("direction: want W, got %s", dir)
	}
	if !almostEqual(next.X, 340) {
		t.Errorf("X: want 340, got %.1f", next.X)
	}
	if !almostEqual(next.Y, 300) {
		t.Errorf("Y: want 300, got %.1f", next.Y)
	}
	if next.Rotation != 180 {
		t.Errorf("Rotation: want 180, got %d", next.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Perpendicular double (mid-row)
// ──────────────────────────────────────────────────────────────────────────────

func TestDoubleGoesPerpendicularEast(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	next, dir := CalculateNextPosition(b, first, doubleTile(), DirEast, true)

	if dir != DirEast {
		t.Errorf("direction: want E, got %s", dir)
	}
	if next.Rotation != 90 {
		t.Errorf("double Rotation: want 90, got %d", next.Rotation)
	}
	if !almostEqual(next.X, 445) {
		t.Errorf("X: want 445, got %.1f", next.X)
	}
	if !almostEqual(next.Y, 300) {
		t.Errorf("Y (same row): want 300, got %.1f", next.Y)
	}
}

func TestDoubleGoesPerpendicularWest(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	next, dir := CalculateNextPosition(b, first, doubleTile(), DirWest, false)

	if dir != DirWest {
		t.Errorf("direction: want W, got %s", dir)
	}
	if next.Rotation != 90 {
		t.Errorf("double Rotation: want 90, got %d", next.Rotation)
	}
	if !almostEqual(next.X, 355) {
		t.Errorf("X: want 355, got %.1f", next.X)
	}
}

func TestTileAfterMidRowDouble(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	dbl, dir := CalculateNextPosition(b, first, doubleTile(), DirEast, true)
	next, _ := CalculateNextPosition(b, dbl, normalTile(), dir, true)

	if !almostEqual(next.Y, first.Y) {
		t.Errorf("Y after mid-row double: want %.1f, got %.1f", first.Y, next.Y)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DOWNWARD curves (right side — growDown=true)
// ──────────────────────────────────────────────────────────────────────────────

func TestCurveDownEnterRightBorder(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	enter, dirAfter := CalculateNextPosition(b, currentEnd, normalTile(), DirEast, true)

	if dirAfter != DirEast {
		t.Errorf("direction after CurveEnter: want E, got %s", dirAfter)
	}
	if enter.CurveType != CurveEnter {
		t.Errorf("CurveType: want %d, got %d", CurveEnter, enter.CurveType)
	}
	if enter.Rotation != 90 {
		t.Errorf("Rotation: want 90, got %d", enter.Rotation)
	}
	// CurveEnter is shifted down by TW/2 + TL/2 so its top aligns with the row bottom.
	expectedY := 300 + b.TileWidth/2 + b.TileLength/2
	if !almostEqual(enter.Y, expectedY) {
		t.Errorf("Y: want %.1f, got %.1f", expectedY, enter.Y)
	}
}

func TestCurveDownExitBelowEnter(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	enter, dirE := CalculateNextPosition(b, currentEnd, normalTile(), DirEast, true)
	exit, dirW := CalculateNextPosition(b, enter, normalTile(), dirE, true)

	if dirW != DirWest {
		t.Errorf("direction after CurveExit: want W, got %s", dirW)
	}
	if exit.CurveType != CurveExit {
		t.Errorf("CurveType: want %d, got %d", CurveExit, exit.CurveType)
	}
	if !almostEqual(exit.X, enter.X) {
		t.Errorf("CurveExit X (%.1f) must equal CurveEnter X (%.1f)", exit.X, enter.X)
	}
	expectedY := enter.Y + b.TileLength
	if !almostEqual(exit.Y, expectedY) {
		t.Errorf("CurveExit Y: want %.1f (below), got %.1f", expectedY, exit.Y)
	}
	if exit.Rotation != 270 {
		t.Errorf("CurveExit Rotation: want 270 (inverted from 90), got %d", exit.Rotation)
	}
}

func TestCurveDownRowDrop(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	enter, dirE := CalculateNextPosition(b, currentEnd, normalTile(), DirEast, true)
	exit, dirW := CalculateNextPosition(b, enter, normalTile(), dirE, true)
	after, _ := CalculateNextPosition(b, exit, normalTile(), dirW, true)

	// rowDrop - TW/2 + Padding/4 for visual breathing room
	expectedY := exit.Y + b.TileLength/2 + b.TileWidth/2 + b.Padding - b.TileWidth/2 + b.Padding/4
	if !almostEqual(after.Y, expectedY) {
		t.Errorf("row drop Y: want %.1f, got %.1f", expectedY, after.Y)
	}
	if after.Rotation != 180 {
		t.Errorf("tile after curve (West): want rot=180, got %d", after.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// UPWARD curves (left side — growDown=false)
// ──────────────────────────────────────────────────────────────────────────────

func TestCurveUpEnterLeftBorder(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 100, Y: 300, Rotation: 180}

	enter, dirAfter := CalculateNextPosition(b, currentEnd, normalTile(), DirWest, false)

	if dirAfter != DirWest {
		t.Errorf("direction after CurveEnter: want W, got %s", dirAfter)
	}
	if enter.CurveType != CurveEnter {
		t.Errorf("CurveType: want %d, got %d", CurveEnter, enter.CurveType)
	}
	if enter.Rotation != 270 {
		t.Errorf("Rotation: want 270, got %d", enter.Rotation)
	}
}

func TestCurveUpExitAboveEnter(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 100, Y: 300, Rotation: 180}

	enter, dirW := CalculateNextPosition(b, currentEnd, normalTile(), DirWest, false)
	exit, dirE := CalculateNextPosition(b, enter, normalTile(), dirW, false)

	if dirE != DirEast {
		t.Errorf("direction after CurveExit: want E, got %s", dirE)
	}
	if exit.CurveType != CurveExit {
		t.Errorf("CurveType: want %d, got %d", CurveExit, exit.CurveType)
	}
	if !almostEqual(exit.X, enter.X) {
		t.Errorf("CurveExit X (%.1f) must equal CurveEnter X (%.1f)", exit.X, enter.X)
	}
	// CurveExit must be ABOVE CurveEnter.
	expectedY := enter.Y - b.TileLength
	if !almostEqual(exit.Y, expectedY) {
		t.Errorf("CurveExit Y: want %.1f (above), got %.1f", expectedY, exit.Y)
	}
	if exit.Rotation != 90 {
		t.Errorf("CurveExit Rotation: want 90 (inverted from 270), got %d", exit.Rotation)
	}
}

func TestCurveUpRowDrop(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 100, Y: 300, Rotation: 180}

	enter, dirW := CalculateNextPosition(b, currentEnd, normalTile(), DirWest, false)
	exit, dirE := CalculateNextPosition(b, enter, normalTile(), dirW, false)
	after, _ := CalculateNextPosition(b, exit, normalTile(), dirE, false)

	// Row ascends with tighter gap + Padding/4 breathing room
	expectedY := exit.Y - b.TileLength/2 - b.TileWidth/2 - b.Padding + b.TileWidth/2 - b.Padding/4
	if !almostEqual(after.Y, expectedY) {
		t.Errorf("upward row drop Y: want %.1f, got %.1f", expectedY, after.Y)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Curve → seamless connection
// ──────────────────────────────────────────────────────────────────────────────

func TestCurveEnterConnectsToTipNoGap(t *testing.T) {
	b := testBoard()

	// East side (down) — CurveEnter is shifted left by halfW so the
	// bottom row sits below the last tile of the top row.
	prevEast := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}
	enter, _ := CalculateNextPosition(b, prevEast, normalTile(), DirEast, true)
	tipEast := prevEast.X + b.TileLength/2
	expectedEastX := tipEast - b.TileWidth/2
	if !almostEqual(enter.X, expectedEastX) {
		t.Errorf("East CurveEnter center: got=%.1f, want=%.1f", enter.X, expectedEastX)
	}

	// West side (up) — CurveEnter is shifted right by halfW so the
	// top row sits above the last tile of the bottom row.
	prevWest := RenderedTile{Tile: normalTile(), X: 100, Y: 300, Rotation: 180}
	enterW, _ := CalculateNextPosition(b, prevWest, normalTile(), DirWest, false)
	tipWest := prevWest.X - b.TileLength/2
	expectedX := tipWest + b.TileWidth/2
	if !almostEqual(enterW.X, expectedX) {
		t.Errorf("West CurveEnter center: got=%.1f, want=%.1f", enterW.X, expectedX)
	}
}

func TestNextRowConnectsToCurve(t *testing.T) {
	b := testBoard()

	// Down curve at right border: first tile nudged toward curve.
	prev := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}
	enter, dirE := CalculateNextPosition(b, prev, normalTile(), DirEast, true)
	exit, dirW := CalculateNextPosition(b, enter, normalTile(), dirE, true)
	after, _ := CalculateNextPosition(b, exit, normalTile(), dirW, true)

	// CurveExit tip uses right edge so the bottom row stays close to the curve.
	nudgedTipX := exit.X + b.TileWidth/2
	afterRightEdge := after.X + b.TileLength/2
	if !almostEqual(afterRightEdge, nudgedTipX) {
		t.Errorf("next row X mismatch: afterRight=%.1f, nudgedTip=%.1f", afterRightEdge, nudgedTipX)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Double at border also uses 2-tile vertical curve
// ──────────────────────────────────────────────────────────────────────────────

func TestDoubleAtBorderUsesCurve(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	enter, dirE := CalculateNextPosition(b, currentEnd, doubleTile(), DirEast, true)
	if enter.CurveType != CurveEnter {
		t.Errorf("double at border should be CurveEnter, got %d", enter.CurveType)
	}

	exit, dirW := CalculateNextPosition(b, enter, normalTile(), dirE, true)
	if exit.CurveType != CurveExit {
		t.Errorf("tile after CurveEnter should be CurveExit, got %d", exit.CurveType)
	}
	if dirW != DirWest {
		t.Errorf("direction after CurveExit: want W, got %s", dirW)
	}
}

func TestDoubleCurveEnterIsVertical(t *testing.T) {
	b := testBoard()

	tests := []struct {
		name    string
		dir     string
		wantRot int
	}{
		{name: "East_CurveEnter_double_rotation_90", dir: DirEast, wantRot: 90},
		{name: "West_CurveEnter_double_rotation_270", dir: DirWest, wantRot: 270},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var endX float64
			var endRot int
			if tc.dir == DirEast {
				endX = 700
				endRot = 0
			} else {
				endX = 100
				endRot = 180
			}

			currentEnd := RenderedTile{Tile: normalTile(), X: endX, Y: 300, Rotation: endRot}
			growDown := tc.dir == DirEast

			enter, _ := CalculateNextPosition(b, currentEnd, doubleTile(), tc.dir, growDown)
			if enter.CurveType != CurveEnter {
				t.Fatalf("expected CurveEnter, got %d", enter.CurveType)
			}
			if enter.Rotation != tc.wantRot {
				t.Errorf("double CurveEnter rotation: want %d, got %d", tc.wantRot, enter.Rotation)
			}
		})
	}
}

func TestDoubleCurveExitIsVertical(t *testing.T) {
	b := testBoard()

	tests := []struct {
		name    string
		dir     string
		wantRot int
	}{
		{name: "East_exit_to_West_double_rotation_270", dir: DirEast, wantRot: 270},
		{name: "West_exit_to_East_double_rotation_90", dir: DirWest, wantRot: 90},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var enterRot int
			if tc.dir == DirEast {
				enterRot = 90
			} else {
				enterRot = 270
			}

			prevCurve := RenderedTile{
				Tile: normalTile(), X: 750, Y: 315,
				Rotation: enterRot, CurveType: CurveEnter,
			}
			growDown := tc.dir == DirEast

			exit, _ := CalculateNextPosition(b, prevCurve, doubleTile(), tc.dir, growDown)
			if exit.CurveType != CurveExit {
				t.Fatalf("expected CurveExit, got %d", exit.CurveType)
			}
			if exit.Rotation != tc.wantRot {
				t.Errorf("double CurveExit rotation: want %d, got %d", tc.wantRot, exit.Rotation)
			}
		})
	}
}

func TestNonDoubleCurveStaysVertical(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	enter, dirE := CalculateNextPosition(b, currentEnd, normalTile(), DirEast, true)
	if enter.Rotation != 90 {
		t.Errorf("non-double CurveEnter should be vertical (90), got %d", enter.Rotation)
	}

	exit, _ := CalculateNextPosition(b, enter, normalTile(), dirE, true)
	if exit.Rotation != 270 {
		t.Errorf("non-double CurveExit should be vertical (270), got %d", exit.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Bidirectional: left goes UP, right goes DOWN
// ──────────────────────────────────────────────────────────────────────────────

func TestBidiRightCurvesDown(t *testing.T) {
	b := testBoard()
	// 8 tiles on the right: 5 horizontal + CurveEnter + CurveExit + 1 on new row.
	tiles := []Tile{
		{6, 5}, // centre (idx 0)
		{5, 4}, {4, 3}, {3, 2}, {2, 1}, {1, 0}, {0, 6}, {6, 3}, // right (idx 1-7)
	}
	chain := make([]PlacedTile, len(tiles))
	for i, tile := range tiles {
		chain[i] = placed(tile, false)
	}

	result := RenderBidirectionalChain(b, chain, 0)
	centerY := result[0].Y

	anyBelow := false
	for _, rt := range result[1:] {
		if rt.Y > centerY+1 {
			anyBelow = true
			break
		}
	}
	if !anyBelow {
		t.Error("right side should curve DOWN (Y > center) but all tiles are at center Y")
	}
}

func TestBidiLeftCurvesUp(t *testing.T) {
	b := testBoard()
	// 8 tiles on the left: 5 horizontal + CurveEnter + CurveExit + 1 on new row.
	tiles := []Tile{
		{6, 3}, {0, 6}, {1, 0}, {2, 1}, {3, 2}, {4, 3}, {5, 4}, // left (idx 0-6)
		{6, 5}, // centre (idx 7)
	}
	chain := make([]PlacedTile, len(tiles))
	for i, tile := range tiles {
		chain[i] = placed(tile, false)
	}
	centerIdx := 7

	result := RenderBidirectionalChain(b, chain, centerIdx)
	centerY := result[centerIdx].Y

	anyAbove := false
	for i := 0; i < centerIdx; i++ {
		if result[i].Y < centerY-1 {
			anyAbove = true
			break
		}
	}
	if !anyAbove {
		t.Error("left side should curve UP (Y < center) but all tiles are at or below center Y")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RenderChain — full serpentine, no bounds violation
// ──────────────────────────────────────────────────────────────────────────────

func TestRenderChainNoBoundsViolation(t *testing.T) {
	b := tallTestBoard()
	tiles := []Tile{
		{6, 5}, {5, 4}, {4, 3}, {3, 2}, {2, 1},
		{1, 0}, {0, 6}, {6, 3}, {3, 1}, {1, 5},
		{5, 2}, {2, 6}, {6, 6}, {6, 1}, {1, 4},
	}
	result := RenderChain(b, tiles)

	if len(result) != len(tiles) {
		t.Fatalf("want %d RenderedTiles, got %d", len(tiles), len(result))
	}

	right := b.ScreenWidth - b.Padding
	bottom := b.ScreenHeight - b.Padding

	for i, rt := range result {
		var hw, hh float64
		if isVerticalRT(rt) {
			hw, hh = b.TileWidth/2, b.TileLength/2
		} else {
			hw, hh = b.TileLength/2, b.TileWidth/2
		}

		if rt.X-hw < b.Padding-borderEps {
			t.Errorf("tile %d left edge %.1f < padding %.1f", i, rt.X-hw, b.Padding)
		}
		if rt.X+hw > right+borderEps {
			t.Errorf("tile %d right edge %.1f > %.1f", i, rt.X+hw, right)
		}
		if rt.Y-hh < b.Padding-borderEps {
			t.Errorf("tile %d top edge %.1f < padding %.1f", i, rt.Y-hh, b.Padding)
		}
		if rt.Y+hh > bottom+borderEps {
			t.Errorf("tile %d bottom edge %.1f > %.1f", i, rt.Y+hh, bottom)
		}
	}
}

func TestRenderChainFirstTileAtCenter(t *testing.T) {
	b := testBoard()
	tiles := []Tile{{6, 5}, {5, 4}}
	result := RenderChain(b, tiles)

	if !almostEqual(result[0].X, b.ScreenWidth/2) || !almostEqual(result[0].Y, b.ScreenHeight/2) {
		t.Errorf("first tile should be at centre (%.0f, %.0f), got (%.1f, %.1f)",
			b.ScreenWidth/2, b.ScreenHeight/2, result[0].X, result[0].Y)
	}
}

func TestRenderChainEmpty(t *testing.T) {
	result := RenderChain(testBoard(), nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Rotation helpers
// ──────────────────────────────────────────────────────────────────────────────

func TestInvertDir(t *testing.T) {
	if invertDir(DirEast) != DirWest {
		t.Error("invertDir(E) should be W")
	}
	if invertDir(DirWest) != DirEast {
		t.Error("invertDir(W) should be E")
	}
}

func TestIsVerticalRT(t *testing.T) {
	for _, rot := range []int{90, 270} {
		if !isVerticalRT(RenderedTile{Rotation: rot}) {
			t.Errorf("Rotation=%d should be vertical", rot)
		}
	}
	for _, rot := range []int{0, 180} {
		if isVerticalRT(RenderedTile{Rotation: rot}) {
			t.Errorf("Rotation=%d should NOT be vertical", rot)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RenderBidirectionalChain — basic tests
// ──────────────────────────────────────────────────────────────────────────────

func placed(tile Tile, flipped bool) PlacedTile {
	return PlacedTile{Tile: tile, Flipped: flipped}
}

func TestBidiCenterAtScreenCenter(t *testing.T) {
	b := testBoard()
	chain := []PlacedTile{placed(normalTile(), false)}
	result := RenderBidirectionalChain(b, chain, 0)
	if len(result) != 1 {
		t.Fatalf("want 1, got %d", len(result))
	}
	rt := result[0]
	if !almostEqual(rt.X, b.ScreenWidth/2) || !almostEqual(rt.Y, b.ScreenHeight/2) {
		t.Errorf("centre: want (%.0f,%.0f), got (%.1f,%.1f)", b.ScreenWidth/2, b.ScreenHeight/2, rt.X, rt.Y)
	}
}

func TestBidiCenterFlippedRotation(t *testing.T) {
	b := testBoard()
	chain := []PlacedTile{placed(normalTile(), true)}
	result := RenderBidirectionalChain(b, chain, 0)
	if result[0].Rotation != 180 {
		t.Errorf("flipped centre rotation: want 180, got %d", result[0].Rotation)
	}
}

func TestBidiRightTileEastOfCenter(t *testing.T) {
	b := testBoard()
	chain := []PlacedTile{placed(normalTile(), false), placed(normalTile(), false)}
	result := RenderBidirectionalChain(b, chain, 0)
	if result[1].X <= result[0].X {
		t.Errorf("right tile X (%.1f) must be > centre X (%.1f)", result[1].X, result[0].X)
	}
}

func TestBidiLeftTileWestOfCenter(t *testing.T) {
	b := testBoard()
	chain := []PlacedTile{placed(normalTile(), false), placed(normalTile(), false)}
	result := RenderBidirectionalChain(b, chain, 1)
	if result[0].X >= result[1].X {
		t.Errorf("left tile X (%.1f) must be < centre X (%.1f)", result[0].X, result[1].X)
	}
}

func TestBidiHorizontalNoFlipRotation(t *testing.T) {
	b := testBoard()
	chain := []PlacedTile{
		placed(normalTile(), false),
		placed(normalTile(), false),
		placed(normalTile(), false),
	}
	result := RenderBidirectionalChain(b, chain, 1)
	for i, rt := range result {
		if rt.Rotation != 0 {
			t.Errorf("tile %d (Flipped=false): want rotation=0, got %d", i, rt.Rotation)
		}
	}
}

func TestBidiHorizontalFlippedRotation(t *testing.T) {
	b := testBoard()
	chain := []PlacedTile{
		placed(normalTile(), true),
		placed(normalTile(), false),
		placed(normalTile(), true),
	}
	result := RenderBidirectionalChain(b, chain, 1)
	if result[0].Rotation != 180 {
		t.Errorf("left flipped: want 180, got %d", result[0].Rotation)
	}
	if result[2].Rotation != 180 {
		t.Errorf("right flipped: want 180, got %d", result[2].Rotation)
	}
}

func TestBidiNoBoundsViolation(t *testing.T) {
	b := testBoard()
	tiles := []Tile{
		{3, 2}, {2, 1}, {1, 0},
		{6, 5},
		{5, 4}, {4, 3}, {3, 1}, {1, 5}, {5, 2}, {2, 6}, {6, 6},
	}
	chain := make([]PlacedTile, len(tiles))
	for i, tile := range tiles {
		chain[i] = placed(tile, false)
	}
	centerIdx := 3

	result := RenderBidirectionalChain(b, chain, centerIdx)
	if len(result) != len(tiles) {
		t.Fatalf("want %d results, got %d", len(tiles), len(result))
	}

	right := b.ScreenWidth - b.Padding
	bottom := b.ScreenHeight - b.Padding

	for i, rt := range result {
		var hw, hh float64
		if isVerticalRT(rt) {
			hw, hh = b.TileWidth/2, b.TileLength/2
		} else {
			hw, hh = b.TileLength/2, b.TileWidth/2
		}
		if rt.X-hw < b.Padding-borderEps || rt.X+hw > right+borderEps ||
			rt.Y-hh < b.Padding-borderEps || rt.Y+hh > bottom+borderEps {
			t.Errorf("tile %d (%v) out of bounds: centre=(%.1f,%.1f) rot=%d",
				i, rt.Tile, rt.X, rt.Y, rt.Rotation)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Full 28-tile serpentine — smoke test
// ──────────────────────────────────────────────────────────────────────────────

func TestFullDominoSet28Tiles(t *testing.T) {
	b := BoardConfig{
		ScreenWidth:  1024,
		ScreenHeight: 2000,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}

	var tiles []Tile
	for high := 0; high <= 6; high++ {
		for low := 0; low <= high; low++ {
			tiles = append(tiles, Tile{High: high, Low: low})
		}
	}

	result := RenderChain(b, tiles)
	if len(result) != 28 {
		t.Fatalf("want 28 RenderedTiles, got %d", len(result))
	}

	right := b.ScreenWidth - b.Padding
	bottom := b.ScreenHeight - b.Padding

	for i, rt := range result {
		var hw, hh float64
		if isVerticalRT(rt) {
			hw, hh = b.TileWidth/2, b.TileLength/2
		} else {
			hw, hh = b.TileLength/2, b.TileWidth/2
		}

		if rt.X-hw < b.Padding-borderEps || rt.X+hw > right+borderEps ||
			rt.Y-hh < b.Padding-borderEps || rt.Y+hh > bottom+borderEps {
			t.Errorf("tile %d (%v) out of bounds: centre=(%.1f, %.1f) rot=%d",
				i, rt.Tile, rt.X, rt.Y, rt.Rotation)
		}
	}
}

func TestTraceUserChain(t *testing.T) {
	b := BoardConfig{
		ScreenWidth:  600,
		ScreenHeight: 900,
		TileLength:   86,
		TileWidth:    43,
		Padding:      17,
	}

	// Chain: 2|0, 0|5, 5|5, 5|3, 3|2, 2|1, 1|1, 1|5, 5|6, 6|4, 4|5, 5|2, 2|4, 4|1, 1|6, 6|6, 6|3, 3|4
	chain := []PlacedTile{
		{Tile: Tile{High: 2, Low: 0}},
		{Tile: Tile{High: 5, Low: 0}},
		{Tile: Tile{High: 5, Low: 5}},
		{Tile: Tile{High: 5, Low: 3}},
		{Tile: Tile{High: 3, Low: 2}},
		{Tile: Tile{High: 2, Low: 1}},
		{Tile: Tile{High: 1, Low: 1}},
		{Tile: Tile{High: 5, Low: 1}},
		{Tile: Tile{High: 6, Low: 5}},
		{Tile: Tile{High: 6, Low: 4}},
		{Tile: Tile{High: 5, Low: 4}},
		{Tile: Tile{High: 5, Low: 2}},
		{Tile: Tile{High: 4, Low: 2}},
		{Tile: Tile{High: 4, Low: 1}},
		{Tile: Tile{High: 6, Low: 1}},
		{Tile: Tile{High: 6, Low: 6}},
		{Tile: Tile{High: 6, Low: 3}},
		{Tile: Tile{High: 4, Low: 3}},
	}

	for _, ci := range []int{0, 2, 6} {
		t.Logf("=== centerIdx=%d (tile %d|%d) ===", ci, chain[ci].Tile.High, chain[ci].Tile.Low)
		result := RenderBidirectionalChain(b, chain, ci)

		curveLabel := map[int]string{0: "---", 1: "ENT", 2: "EXT"}
		for i, rt := range result {
			marker := ""
			if rt.Tile.IsDouble() {
				marker = " DOUBLE"
			}
			if rt.CurveType != CurveNone {
				marker += " CURVE"
			}
			side := "R"
			if i < ci {
				side = "L"
			} else if i == ci {
				side = "C"
			}
			t.Logf("  [%2d] %s %d|%d  X=%6.1f  Y=%6.1f  Rot=%3d  %s%s",
				i, side, rt.Tile.High, rt.Tile.Low, rt.X, rt.Y, rt.Rotation,
				curveLabel[rt.CurveType], marker)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RotateLayout tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRotateLayout_ZeroIsNoop(t *testing.T) {
	b := testBoard()
	tiles := []RenderedTile{
		{Tile: Tile{3, 2}, X: 100, Y: 200, Rotation: 0},
		{Tile: Tile{4, 3}, X: 160, Y: 200, Rotation: 0},
	}

	orig := make([]RenderedTile, len(tiles))
	copy(orig, tiles)

	RotateLayout(tiles, b, b, 0)

	for i := range tiles {
		if tiles[i].X != orig[i].X || tiles[i].Y != orig[i].Y || tiles[i].Rotation != orig[i].Rotation {
			t.Errorf("tile[%d] changed with angle=0: got (%v, %v, %v), want (%v, %v, %v)",
				i, tiles[i].X, tiles[i].Y, tiles[i].Rotation,
				orig[i].X, orig[i].Y, orig[i].Rotation)
		}
	}
}

func TestRotateLayout_360IsIdentity(t *testing.T) {
	b := testBoard()
	tiles := []RenderedTile{
		{Tile: Tile{3, 2}, X: 100, Y: 200, Rotation: 0},
		{Tile: Tile{4, 3}, X: 500, Y: 350, Rotation: 90},
		{Tile: Tile{6, 6}, X: 400, Y: 300, Rotation: 180},
	}

	orig := make([]RenderedTile, len(tiles))
	copy(orig, tiles)

	RotateLayout(tiles, b, b, 90)
	RotateLayout(tiles, b, b, 90)
	RotateLayout(tiles, b, b, 90)
	RotateLayout(tiles, b, b, 90)

	const eps = 1e-9
	for i := range tiles {
		if math.Abs(tiles[i].X-orig[i].X) > eps || math.Abs(tiles[i].Y-orig[i].Y) > eps {
			t.Errorf("tile[%d] position drift after 4×90: got (%.2f, %.2f), want (%.2f, %.2f)",
				i, tiles[i].X, tiles[i].Y, orig[i].X, orig[i].Y)
		}

		if tiles[i].Rotation%360 != orig[i].Rotation%360 {
			t.Errorf("tile[%d] rotation drift after 4×90: got %d, want %d",
				i, tiles[i].Rotation%360, orig[i].Rotation%360)
		}
	}
}

func TestRotateLayout_90RotatesCorrectly(t *testing.T) {
	b := BoardConfig{
		ScreenWidth:  800,
		ScreenHeight: 600,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}

	tiles := []RenderedTile{
		{Tile: Tile{3, 2}, X: b.ScreenWidth / 2, Y: b.ScreenHeight / 2, Rotation: 0},
	}

	RotateLayout(tiles, b, b, 90)

	if tiles[0].X != b.ScreenWidth/2 || tiles[0].Y != b.ScreenHeight/2 {
		t.Errorf("center tile should remain at center after 90° rotation: got (%.2f, %.2f), want (%.2f, %.2f)",
			tiles[0].X, tiles[0].Y, b.ScreenWidth/2, b.ScreenHeight/2)
	}

	if tiles[0].Rotation != 90 {
		t.Errorf("rotation should be 90, got %d", tiles[0].Rotation)
	}
}

func TestRotateLayout_SwappedDimensions(t *testing.T) {
	screenCfg := BoardConfig{
		ScreenWidth:  800,
		ScreenHeight: 600,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}
	engineCfg := BoardConfig{
		ScreenWidth:  600,
		ScreenHeight: 800,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}

	tiles := []RenderedTile{
		{Tile: Tile{3, 2}, X: 300, Y: 400, Rotation: 0},
	}

	RotateLayout(tiles, engineCfg, screenCfg, 90)

	if tiles[0].X != 400 || tiles[0].Y != 300 {
		t.Errorf("center of swapped engine should map to screen center: got (%.2f, %.2f), want (400, 300)",
			tiles[0].X, tiles[0].Y)
	}

	if tiles[0].Rotation != 90 {
		t.Errorf("rotation should be 90, got %d", tiles[0].Rotation)
	}
}

func TestRotateLayout_180InvertsPositions(t *testing.T) {
	b := testBoard()

	tiles := []RenderedTile{
		{Tile: Tile{3, 2}, X: 100, Y: 200, Rotation: 0},
	}

	RotateLayout(tiles, b, b, 180)

	expectedX := b.ScreenWidth - 100
	expectedY := b.ScreenHeight - 200

	const eps = 1e-9
	if math.Abs(tiles[0].X-expectedX) > eps || math.Abs(tiles[0].Y-expectedY) > eps {
		t.Errorf("180° should mirror position: got (%.2f, %.2f), want (%.2f, %.2f)",
			tiles[0].X, tiles[0].Y, expectedX, expectedY)
	}

	if tiles[0].Rotation != 180 {
		t.Errorf("rotation should be 180, got %d", tiles[0].Rotation)
	}
}

func TestRotateLayout_RotationModulo(t *testing.T) {
	b := testBoard()

	tiles := []RenderedTile{
		{Tile: Tile{3, 2}, X: 400, Y: 300, Rotation: 270},
	}

	RotateLayout(tiles, b, b, 180)

	expected := (270 + 180) % 360
	if tiles[0].Rotation != expected {
		t.Errorf("rotation should be %d, got %d", expected, tiles[0].Rotation)
	}
}

func TestRotateLayout_BidirectionalChainAllAngles(t *testing.T) {
	b := tallTestBoard()

	chain := []PlacedTile{
		{Tile: Tile{3, 2}},
		{Tile: Tile{5, 3}},
		{Tile: Tile{5, 4}},
		{Tile: Tile{6, 4}},
		{Tile: Tile{6, 1}},
	}

	for _, angle := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("angle_%d", angle), func(t *testing.T) {
			engineCfg := b
			screenCfg := b
			if angle == 90 || angle == 270 {
				engineCfg.ScreenWidth, engineCfg.ScreenHeight = b.ScreenHeight, b.ScreenWidth
			}

			rendered := RenderBidirectionalChain(engineCfg, chain, 2)

			if angle != 0 {
				RotateLayout(rendered, engineCfg, screenCfg, angle)
			}

			for i, rt := range rendered {
				if rt.X < 0 || rt.X > screenCfg.ScreenWidth+screenCfg.TileLength {
					t.Errorf("angle=%d tile[%d] X=%.2f out of bounds [0, %.2f]",
						angle, i, rt.X, screenCfg.ScreenWidth+screenCfg.TileLength)
				}

				if rt.Y < 0 || rt.Y > screenCfg.ScreenHeight+screenCfg.TileLength {
					t.Errorf("angle=%d tile[%d] Y=%.2f out of bounds [0, %.2f]",
						angle, i, rt.Y, screenCfg.ScreenHeight+screenCfg.TileLength)
				}
			}
		})
	}
}
