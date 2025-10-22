package game

type Booster struct {
	Type string `json:"type"` // "double-shot", "remove-piece", etc.
	Used bool   `json:"used"` // true si déjà utilisé
}

type GameState struct {
	Board        [15][15]string `json:"board"`        // Taille max pour supporter la croissance du mode exponentiel
	BoosterCells [15][15]string `json:"boosterCells"` // Cases spéciales contenant des boosters ("double-shot", "remove-piece", etc.)
	BoostersR    []Booster      `json:"boostersR"`    // Boosters collectés par le joueur Rouge
	BoostersY    []Booster      `json:"boostersY"`    // Boosters collectés par le joueur Jaune
	Next         string         `json:"next"`
	Winner       string         `json:"winner"`
	Finished     bool           `json:"finished"`
	Mode         string         `json:"mode"`
	Rows         int            `json:"rows"`
	Cols         int            `json:"cols"`
	WinLength    int            `json:"winLength"`
	Version      int            `json:"version"`
}
