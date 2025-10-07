package power4

// Winner retourne le jeton ("R" ou "Y") gagnant s'il existe, sinon chaîne vide.
func Winner(board [6][7]string) string {
	for r := 0; r < 6; r++ {
		for c := 0; c < 7; c++ {
			p := board[r][c]
			if p == "" {
				continue
			}
			// horizontal
			if c+3 < 7 && board[r][c+1] == p && board[r][c+2] == p && board[r][c+3] == p {
				return p
			}
			// vertical
			if r+3 < 6 && board[r+1][c] == p && board[r+2][c] == p && board[r+3][c] == p {
				return p
			}
			// diag ↘
			if r+3 < 6 && c+3 < 7 && board[r+1][c+1] == p && board[r+2][c+2] == p && board[r+3][c+3] == p {
				return p
			}
			// diag ↙
			if r+3 < 6 && c-3 >= 0 && board[r+1][c-1] == p && board[r+2][c-2] == p && board[r+3][c-3] == p {
				return p
			}
		}
	}
	return ""
}
