package power4

func Winner(board [][]string, rows, cols, winLength int) string {
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			p := board[r][c]
			if p == "" {
				continue
			}
			// horizontal
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
			// vertical
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
			// diagonal droite
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
			// diagonal gauche
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
