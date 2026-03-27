package handler

// TileInfo descreve quantas pedras cada jogador compra e o que sobra para compra durante o jogo.
type TileInfo struct {
	TilesEach  int
	TotalTiles int
	Reserve    int
	SetType    string // "duplo-6" ou "duplo-9"
}

// CalcTiles retorna a distribuição de pedras por jogador para o Pontinho.
//
// Todos os jogos usam duplo-6 (28 pedras).
// Distribuição: máximo de pedras por jogador mantendo ≥7 de reserva para comprar.
//
//   2 → 9 cada, 10 reserva
//   3 → 7 cada,  7 reserva
//   4 → 5 cada,  8 reserva
//   5 → 4 cada,  8 reserva
//   6 → 3 cada, 10 reserva
//   7 → 3 cada,  7 reserva
//   8 → 2 cada, 12 reserva
//   9 → 2 cada, 10 reserva
//  10 → 2 cada,  8 reserva
func CalcTiles(playerCount int) TileInfo {
	const d6 = 28

	switch playerCount {
	case 2:
		return TileInfo{9, d6, d6 - 9*2, "duplo-6"}  // 10 para comprar
	case 3:
		return TileInfo{7, d6, d6 - 7*3, "duplo-6"}  //  7 para comprar
	case 4:
		return TileInfo{5, d6, d6 - 5*4, "duplo-6"}  //  8 para comprar
	case 5:
		return TileInfo{4, d6, d6 - 4*5, "duplo-6"}  //  8 para comprar
	case 6:
		return TileInfo{3, d6, d6 - 3*6, "duplo-6"}  // 10 para comprar
	case 7:
		return TileInfo{3, d6, d6 - 3*7, "duplo-6"}  //  7 para comprar
	case 8:
		return TileInfo{2, d6, d6 - 2*8, "duplo-6"}  // 12 para comprar
	case 9:
		return TileInfo{2, d6, d6 - 2*9, "duplo-6"}  // 10 para comprar
	case 10:
		return TileInfo{2, d6, d6 - 2*10, "duplo-6"} //  8 para comprar
	default:
		if playerCount <= 0 {
			return TileInfo{0, 0, 0, "—"}
		}
		each := 2
		reserve := d6 - each*playerCount
		if reserve < 0 {
			reserve = 0
		}
		return TileInfo{each, d6, reserve, "duplo-6"}
	}
}
