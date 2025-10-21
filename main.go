package main

import (
	"encoding/json"
	"html/template"
	"log"
	mrand "math/rand"
	"net/http"
	"os"
	"power4/game"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------- STRUCTURES ET VARIABLES GLOBALES ----------------

// TurboStateExtended : pour les boosters
type TurboStateExtended struct {
	game.GameState
	DoublePlayNext bool
	BlockedColumn  int
}

// États solo et multi (inchangés)
var (
	soloExponentielState  = game.GameState{Next: "R", Mode: "solo-exponentiel", Rows: 6, Cols: 7, WinLength: 4}
	soloClassiqueState    = game.GameState{Next: "R", Mode: "solo-classique", Rows: 6, Cols: 7, WinLength: 4}
	soloTurboState        = TurboStateExtended{GameState: game.GameState{Next: "R", Mode: "solo-turbo", Rows: 6, Cols: 7, WinLength: 4}, BlockedColumn: -1}
	multiExponentielState = game.GameState{Next: "R", Mode: "multi-exponentiel", Rows: 6, Cols: 7, WinLength: 4}
	multiClassiqueState   = game.GameState{Next: "R", Mode: "multi-classique", Rows: 6, Cols: 7, WinLength: 4}
	multiTurboState       = TurboStateExtended{GameState: game.GameState{Next: "R", Mode: "multi-turbo", Rows: 6, Cols: 7, WinLength: 4}, BlockedColumn: -1}

	muSoloExponentiel  sync.Mutex
	muSoloClassique    sync.Mutex
	muSoloTurbo        sync.Mutex
	muMultiExponentiel sync.Mutex
	muMultiClassique   sync.Mutex
	muMultiTurbo       sync.Mutex

	playerNames = []string{"Joueur 1", "Joueur 2"}
)

// Templates
var (
	indexTmpl = template.Must(template.New("index.html").Funcs(template.FuncMap{
		"seq": func(a, b int) []int {
			s := make([]int, b-a+1)
			for i := a; i <= b; i++ {
				s[i-a] = i
			}
			return s
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}).ParseFiles("templates/index.html"))
	menuTmpl        = template.Must(template.New("menu.html").ParseFiles("templates/menu.html"))
	welcomeTmpl     = template.Must(template.ParseFiles("templates/welcome.html"))
	playersTmpl     = template.Must(template.ParseFiles("templates/players.html"))
	multiplayerTmpl = template.Must(template.ParseFiles("templates/multiplayer.html"))
	wsUpgrader      = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsClients       = map[*websocket.Conn]bool{}
	wsMu            sync.Mutex
)

// ---------------- PARTIES AVEC CODES UNIQUES ----------------

type Party struct {
	Code           string
	CreatedAt      time.Time
	State          game.GameState
	Clients        map[*websocket.Conn]bool
	ClientTeam     map[*websocket.Conn]string // Stocke l'équipe de chaque client ('R' ou 'Y')
	DoublePlayNext bool                       // Pour le booster "double-shot"
	BlockedColumn  int                        // Colonne bloquée par le booster "block-column"
	Mu             sync.Mutex
}

var (
	parties   = make(map[string]*Party)
	partiesMu sync.Mutex
)

func generateCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[mrand.Intn(len(charset))]
	}
	return string(b)
}

func createPartyHandler(w http.ResponseWriter, r *http.Request) {
	partiesMu.Lock()
	defer partiesMu.Unlock()

	// Récupérer le mode depuis l'URL
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "solo-classique" // Mode par défaut
	}

	code := generateCode()
	newState := game.GameState{
		Rows: 6, Cols: 7, WinLength: 4,
		Next: "R", Mode: mode,
	}

	p := &Party{
		Code:          code,
		CreatedAt:     time.Now(),
		State:         newState,
		Clients:       make(map[*websocket.Conn]bool),
		ClientTeam:    make(map[*websocket.Conn]string),
		BlockedColumn: -1,
	}

	// Générer les cases boosters si mode turbo
	if strings.Contains(mode, "turbo") {
		generateBoosterCells(&p.State)
	}

	parties[code] = p

	log.Printf("✅ Nouvelle partie créée : %s (mode: %s)", code, mode)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
}

// generateBoosterCells génère aléatoirement des cases contenant des boosters
func generateBoosterCells(state *game.GameState) {
	// Utiliser un générateur aléatoire local (Go 1.20+)
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))

	// Liste des types de boosters disponibles
	boosterTypes := []string{
		"double-shot",
		"remove-piece",
		"block-column",
		"swap-colors",
		"wildcard",
	}

	// Nombre de boosters à placer (1-2 de chaque type)
	numBoostersPerType := 2

	// Créer une liste de toutes les positions possibles
	type Position struct {
		row, col int
	}
	var positions []Position
	for r := 0; r < state.Rows; r++ {
		for c := 0; c < state.Cols; c++ {
			positions = append(positions, Position{r, c})
		}
	}

	// Mélanger les positions
	rng.Shuffle(len(positions), func(i, j int) {
		positions[i], positions[j] = positions[j], positions[i]
	})

	// Placer les boosters
	posIndex := 0
	for _, boosterType := range boosterTypes {
		for i := 0; i < numBoostersPerType && posIndex < len(positions); i++ {
			pos := positions[posIndex]
			state.BoosterCells[pos.row][pos.col] = boosterType
			posIndex++
			log.Printf("[Boosters] Case booster placée en (%d,%d): %s", pos.row, pos.col, boosterType)
		}
	}
}

func joinPartyHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.URL.Query().Get("code"))
	partiesMu.Lock()
	_, exists := parties[code]
	partiesMu.Unlock()
	if !exists {
		http.Error(w, "Party not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "joined", "code": code})
	log.Printf("👥 Un joueur a rejoint la partie %s", code)
}

func wsPartyHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/ws/")
	partiesMu.Lock()
	p, exists := parties[code]
	partiesMu.Unlock()
	if !exists {
		http.Error(w, "Party not found", http.StatusNotFound)
		return
	}

	// Récupérer l'équipe depuis l'URL (query param)
	team := r.URL.Query().Get("team")
	if team == "" {
		team = "R" // Par défaut équipe rouge
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	p.Mu.Lock()
	p.Clients[conn] = true
	p.ClientTeam[conn] = team // Stocker l'équipe du client
	p.Mu.Unlock()

	_ = conn.WriteJSON(p.State)

	go func() {
		defer func() {
			p.Mu.Lock()
			delete(p.Clients, conn)
			delete(p.ClientTeam, conn)
			p.Mu.Unlock()
			conn.Close()
		}()

		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			if msg["type"] == "play" {
				col := int(msg["col"].(float64))
				handlePartyMove(p, conn, col)
			}
		}
	}()
}

func handlePartyMove(p *Party, conn *websocket.Conn, col int) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	// Vérifier que c'est bien le tour du joueur qui fait le mouvement
	playerTeam := p.ClientTeam[conn]
	if playerTeam != p.State.Next {
		// Envoyer un message d'erreur au client
		errorMsg := map[string]interface{}{
			"type":    "error",
			"message": "Ce n'est pas votre tour!",
		}
		_ = conn.WriteJSON(errorMsg)
		return
	}

	// Vérifier si la colonne est bloquée en mode turbo
	if strings.Contains(p.State.Mode, "turbo") && col == p.BlockedColumn {
		log.Printf("Colonne %d bloquée, coup impossible", col)
		p.BlockedColumn = -1 // Débloquer après tentative
		errorMsg := map[string]interface{}{
			"type":    "error",
			"message": "Cette colonne est bloquée!",
		}
		_ = conn.WriteJSON(errorMsg)
		return
	}

	if p.State.Finished || col < 0 || col >= p.State.Cols {
		return
	}

	placedRow := -1
	for r := p.State.Rows - 1; r >= 0; r-- {
		if p.State.Board[r][col] == "" {
			p.State.Board[r][col] = p.State.Next
			placedRow = r
			break
		}
	}

	if placedRow == -1 {
		return // Colonne pleine
	}

	// Vérifier si le joueur a placé son pion sur une case booster
	boosterObtained := ""
	playerWhoGotBooster := ""
	if strings.Contains(p.State.Mode, "turbo") && p.State.BoosterCells[placedRow][col] != "" {
		boosterType := p.State.BoosterCells[placedRow][col]
		log.Printf("[Booster] Joueur %s a récupéré un booster: %s en (%d,%d)", p.State.Next, boosterType, placedRow, col)

		boosterObtained = boosterType
		playerWhoGotBooster = p.State.Next
		p.State.BoosterCells[placedRow][col] = "" // Retirer le booster de la grille
	}

	if winner := game.WinnerWithLength(p.State.Board, p.State.Rows, p.State.Cols, p.State.WinLength); winner != "" {
		p.State.Winner = winner
		p.State.Finished = true
	} else {
		// Changer de joueur sauf si double coup est actif
		if strings.Contains(p.State.Mode, "turbo") && p.DoublePlayNext {
			// Ne pas changer de joueur, désactiver le double coup
			p.DoublePlayNext = false
			log.Printf("[Booster] Double coup utilisé, même joueur rejoue")
		} else {
			if p.State.Next == "R" {
				p.State.Next = "Y"
			} else {
				p.State.Next = "R"
			}
		}
	}
	p.State.Version++

	// Envoyer la mise à jour avec le booster éventuel
	for c := range p.Clients {
		response := map[string]interface{}{
			"type":    "state",
			"state":   p.State,
			"blocked": p.BlockedColumn,
		}
		if boosterObtained != "" && c == conn {
			response["booster"] = boosterObtained
			response["player"] = playerWhoGotBooster
		}
		_ = c.WriteJSON(response)
	}
}

// ---------------- HANDLERS CLASSIQUES (inchangés) ----------------

func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	if err := welcomeTmpl.Execute(w, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func playersHandler(w http.ResponseWriter, r *http.Request) {
	if err := playersTmpl.Execute(w, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func setPlayersHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	player1 := r.FormValue("player1")
	player2 := r.FormValue("player2")
	if player1 != "" {
		playerNames[0] = player1
	}
	if player2 != "" {
		playerNames[1] = player2
	}
	http.Redirect(w, r, "/menu", http.StatusSeeOther)
}

func menuHandler(w http.ResponseWriter, r *http.Request) {
	if err := menuTmpl.Execute(w, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	partiesMu.Lock()
	p, exists := parties[code]
	partiesMu.Unlock()

	if !exists {
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	p.Mu.Lock()
	defer p.Mu.Unlock()

	data := struct {
		game.GameState
		Player1Name   string
		Player2Name   string
		BlockedColumn int
		Code          string
	}{
		GameState:     p.State,
		Player1Name:   playerNames[0],
		Player2Name:   playerNames[1],
		BlockedColumn: p.BlockedColumn,
		Code:          code,
	}

	if err := indexTmpl.Execute(w, data); err != nil {
		log.Printf("template execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ---------------- MAIN ----------------

func main() {
	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/players", playersHandler)
	http.HandleFunc("/set-players", setPlayersHandler)
	http.HandleFunc("/menu", menuHandler)
	http.HandleFunc("/game", gameHandler)
	http.HandleFunc("/api/party/create", createPartyHandler)
	http.HandleFunc("/api/party/join", joinPartyHandler)
	http.HandleFunc("/ws/", wsPartyHandler)

	// Fichiers statiques
	fs := http.FileServer(http.Dir("templates"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	imgFs := http.FileServer(http.Dir("Images"))
	http.Handle("/images/", http.StripPrefix("/images/", imgFs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("✅ Serveur démarré sur : http://localhost:%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
