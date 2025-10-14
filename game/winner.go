package game

// Winner retourne le jeton ("R" ou "Y") gagnant s'il existe, sinon une chaîne vide.
// Le plateau est un tableau 15x15 de chaînes ("R", "Y" ou "").
func Winner(board [15][15]string) string {
	return WinnerWithLength(board, 6, 7, 4)
}

// WinnerWithLength vérifie s'il y a un gagnant avec un nombre d'alignements spécifique
func WinnerWithLength(board [15][15]string, rows, cols, winLength int) string {
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			p := board[r][c]
			if p == "" {
				continue
			}
			// horizontal →
			if c+winLength-1 < cols {
				win := true
				for i := 1; i < winLength; i++ {
					if board[r][c+i] != p {
						win = false
						break
					}
				}
				if win {
					return p
				}
			}
			// vertical ↓
			if r+winLength-1 < rows {
				win := true
				for i := 1; i < winLength; i++ {
					if board[r+i][c] != p {
						win = false
						break
					}
				}
				if win {
					return p
				}
			}
			// diag ↘
			if r+winLength-1 < rows && c+winLength-1 < cols {
				win := true
				for i := 1; i < winLength; i++ {
					if board[r+i][c+i] != p {
						win = false
						break
					}
				}
				if win {
					return p
				}
			}
			// diag ↙
			if r+winLength-1 < rows && c-winLength+1 >= 0 {
				win := true
				for i := 1; i < winLength; i++ {
					if board[r+i][c-i] != p {
						win = false
						break
					}
				}
				if win {
					return p
				}
			}
		}
	}
	return ""
}
