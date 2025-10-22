package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	mrand "math/rand"
	"net/http"
	"os"
<<<<<<< HEAD
	_ "os/exec" // Gardé pour usage futur
=======
>>>>>>> 74104140c5ab87bed33754a1b0ce65a7ae112c8c
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
			msgType, _ := msg["type"].(string)
			switch msgType {
			case "play":
				col := int(msg["col"].(float64))
				handlePartyMove(p, conn, col)
			case "booster":
				handleBoosterAction(p, conn, msg)
			}
		}
	}()
}

func handlePartyMove(p *Party, conn *websocket.Conn, col int) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	// En mode solo, on autorise les deux joueurs à jouer
	// En mode multi, on vérifie que c'est bien le tour du joueur
	playerTeam := p.ClientTeam[conn]
	isSoloMode := strings.Contains(p.State.Mode, "solo")

	if !isSoloMode && playerTeam != "" && playerTeam != p.State.Next {
		// En mode multijoueur, vérifier que c'est le bon tour
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

		// Ajouter le booster à la collection du joueur
		newBooster := game.Booster{
			Type: boosterType,
			Used: false,
		}
		if p.State.Next == "R" {
			p.State.BoostersR = append(p.State.BoostersR, newBooster)
		} else {
			p.State.BoostersY = append(p.State.BoostersY, newBooster)
		}

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

func handleBoosterAction(p *Party, conn *websocket.Conn, msg map[string]interface{}) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	action, _ := msg["action"].(string)
	player, _ := msg["player"].(string)
	indexFloat, _ := msg["index"].(float64)
	index := int(indexFloat)

	log.Printf("[Booster] Action reçue: %s de joueur %s (index %d)", action, player, index)

	// Vérifier que c'est bien le tour du joueur
	if player != p.State.Next {
		errorMsg := map[string]interface{}{
			"type":    "error",
			"message": "Ce n'est pas votre tour!",
		}
		_ = conn.WriteJSON(errorMsg)
		return
	}

	// Traiter selon l'action
	switch action {
	case "double-shot":
		// Activer le double coup pour le prochain tour
		p.DoublePlayNext = true

		// Marquer le booster comme utilisé
		if player == "R" && index >= 0 && index < len(p.State.BoostersR) {
			p.State.BoostersR[index].Used = true
		} else if player == "Y" && index >= 0 && index < len(p.State.BoostersY) {
			p.State.BoostersY[index].Used = true
		}

		log.Printf("[Booster] Double coup activé pour joueur %s", player)

		// Envoyer confirmation
		for c := range p.Clients {
			_ = c.WriteJSON(map[string]interface{}{
				"type":    "state",
				"state":   p.State,
				"blocked": p.BlockedColumn,
			})
		}

	case "block-column":
		// Le client doit envoyer la colonne à bloquer dans un message suivant
		// Pour l'instant, on marque juste le booster comme utilisé
		if player == "R" && index >= 0 && index < len(p.State.BoostersR) {
			p.State.BoostersR[index].Used = true
		} else if player == "Y" && index >= 0 && index < len(p.State.BoostersY) {
			p.State.BoostersY[index].Used = true
		}

		log.Printf("[Booster] Bloqueur activé pour joueur %s (en attente de sélection)", player)

	default:
		log.Printf("[Booster] Action non implémentée: %s", action)
	}
}

func boosterActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lire le body brut pour debug
	bodyBytes, _ := io.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	log.Printf("[Booster] Body brut reçu: %s", bodyString)

	// Recréer le body pour ParseForm
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Parser le formulaire
	if err := r.ParseForm(); err != nil {
		log.Printf("[Booster] Erreur ParseForm: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Erreur de parsing",
		})
		return
	}

	action := r.FormValue("action")
	player := r.FormValue("player")
	code := r.FormValue("code")

	log.Printf("[Booster] Action reçue - action: '%s', player: '%s', code: '%s'", action, player, code)
	log.Printf("[Booster] Tous les FormValues: %v", r.Form)

	if code == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Code de partie manquant",
		})
		return
	}

	partiesMu.Lock()
	p, exists := parties[code]
	partiesMu.Unlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Partie introuvable",
		})
		return
	}

	p.Mu.Lock()
	defer p.Mu.Unlock()

	switch action {
	case "double-shot":
		// Activer le double coup pour le joueur
		p.DoublePlayNext = true
		log.Printf("[Booster] Double coup activé pour joueur %s dans partie %s", player, code)

		// Notifier tous les clients
		for c := range p.Clients {
			_ = c.WriteJSON(map[string]interface{}{
				"type":    "state",
				"state":   p.State,
				"message": "Double coup activé!",
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Double coup activé",
		})

	case "remove-piece":
		// Retirer un pion
		rowStr := r.FormValue("row")
		colStr := r.FormValue("col")
		log.Printf("[Booster] Retrait de pion à (%s, %s) par joueur %s", rowStr, colStr, player)

		// Convertir et retirer le pion
		var row, col int
		fmt.Sscanf(rowStr, "%d", &row)
		fmt.Sscanf(colStr, "%d", &col)

		if row >= 0 && row < p.State.Rows && col >= 0 && col < p.State.Cols {
			p.State.Board[row][col] = ""
			p.State.Version++
			log.Printf("[Booster] Pion retiré à (%d, %d)", row, col)
		}

		// Notifier tous les clients
		for c := range p.Clients {
			_ = c.WriteJSON(map[string]interface{}{
				"type":  "state",
				"state": p.State,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Pion retiré",
		})

	case "block-column":
		// Bloquer une colonne
		colStr := r.FormValue("col")
		log.Printf("[Booster] Blocage de la colonne %s par joueur %s", colStr, player)

		var col int
		fmt.Sscanf(colStr, "%d", &col)

		p.BlockedColumn = col
		p.State.Version++
		log.Printf("[Booster] Colonne %d bloquée", col)

		// Notifier tous les clients
		for c := range p.Clients {
			_ = c.WriteJSON(map[string]interface{}{
				"type":    "state",
				"state":   p.State,
				"blocked": p.BlockedColumn,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Colonne bloquée",
		})

	case "swap-colors":
		// Échanger deux pions
		row1Str := r.FormValue("row1")
		col1Str := r.FormValue("col1")
		row2Str := r.FormValue("row2")
		col2Str := r.FormValue("col2")
		log.Printf("[Booster] Échange de pions (%s,%s) <-> (%s,%s) par joueur %s", row1Str, col1Str, row2Str, col2Str, player)

		var row1, col1, row2, col2 int
		fmt.Sscanf(row1Str, "%d", &row1)
		fmt.Sscanf(col1Str, "%d", &col1)
		fmt.Sscanf(row2Str, "%d", &row2)
		fmt.Sscanf(col2Str, "%d", &col2)

		// Échanger les pions
		if row1 >= 0 && row1 < p.State.Rows && col1 >= 0 && col1 < p.State.Cols &&
			row2 >= 0 && row2 < p.State.Rows && col2 >= 0 && col2 < p.State.Cols {
			p.State.Board[row1][col1], p.State.Board[row2][col2] = p.State.Board[row2][col2], p.State.Board[row1][col1]
			p.State.Version++
			log.Printf("[Booster] Pions échangés entre (%d,%d) et (%d,%d)", row1, col1, row2, col2)
		}

		// Notifier tous les clients
		for c := range p.Clients {
			_ = c.WriteJSON(map[string]interface{}{
				"type":  "state",
				"state": p.State,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Pions échangés",
		})

	case "wildcard":
		// Placer un pion n'importe où
		rowStr := r.FormValue("row")
		colStr := r.FormValue("col")
		log.Printf("[Booster] Placement joker à (%s, %s) par joueur %s", rowStr, colStr, player)

		var row, col int
		fmt.Sscanf(rowStr, "%d", &row)
		fmt.Sscanf(colStr, "%d", &col)

		// Placer le pion du joueur actuel
		if row >= 0 && row < p.State.Rows && col >= 0 && col < p.State.Cols && p.State.Board[row][col] == "" {
			p.State.Board[row][col] = player
			p.State.Version++
			log.Printf("[Booster] Joker placé à (%d,%d) pour joueur %s", row, col, player)

			// Vérifier victoire
			winner := game.WinnerWithLength(p.State.Board, p.State.Rows, p.State.Cols, p.State.WinLength)
			if winner != "" {
				p.State.Winner = winner
				p.State.Finished = true
				log.Printf("[Booster] Victoire détectée pour joueur %s après joker!", winner)
			} else {
				// Changer de joueur
				if p.State.Next == "R" {
					p.State.Next = "Y"
				} else {
					p.State.Next = "R"
				}
			}
		}

		// Notifier tous les clients
		for c := range p.Clients {
			_ = c.WriteJSON(map[string]interface{}{
				"type":  "state",
				"state": p.State,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Joker placé",
		})

	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Action inconnue",
		})
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
		IsTurbo       bool
	}{
		GameState:     p.State,
		Player1Name:   playerNames[0],
		Player2Name:   playerNames[1],
		BlockedColumn: p.BlockedColumn,
		Code:          code,
		IsTurbo:       strings.Contains(p.State.Mode, "turbo"),
	}

	// Debug: afficher les boosters dans le template
	hasBooster := false
	for r := 0; r < p.State.Rows; r++ {
		for c := 0; c < p.State.Cols; c++ {
			if p.State.BoosterCells[r][c] != "" {
				hasBooster = true
				log.Printf("[Template Debug] Booster à (%d,%d): %s", r, c, p.State.BoosterCells[r][c])
			}
		}
	}
	if !hasBooster {
		log.Printf("[Template Debug] AUCUN booster dans le state pour partie %s", code)
	}

	if err := indexTmpl.Execute(w, data); err != nil {
		log.Printf("template execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

<<<<<<< HEAD
// launchEasterEggHandler lance le jeu ESPERSOUL2 comme Easter Egg
// NOTE: Le lancement automatique est désactivé car il fait crasher le serveur
// L'utilisateur doit télécharger le jeu via /download/espersoul2
func launchEasterEggHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	log.Println("🎮 EASTER EGG ACTIVÉ!")

	// Définir le chemin vers le jeu
	gamePath := "epp4\\ESPERSOUL2"
	exePath := gamePath + "\\ESPERSOUL2.exe"

	// Vérifier si l'exécutable existe
	if _, err := os.Stat(exePath); err == nil {
		log.Println("✅ ESPERSOUL2.exe trouvé - téléchargement disponible")
		// Retourner succès mais indiquer qu'il faut télécharger
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"message":  "🎮 ESPERSOUL2 disponible au téléchargement!",
			"method":   "download",
			"download": "/download/espersoul2",
		})
		return
	}

	log.Println("⚠️ ESPERSOUL2.exe non trouvé")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "Jeu non disponible",
		"method":  "none",
	})
}

// downloadEasterEggHandler permet de télécharger le jeu ESPERSOUL2
func downloadEasterEggHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("💾 Demande de téléchargement ESPERSOUL2...")

	// Chemin vers le dossier du jeu
	gamePath := "epp4/ESPERSOUL2"

	// Vérifier si le dossier existe
	if _, err := os.Stat(gamePath); os.IsNotExist(err) {
		log.Printf("❌ Dossier ESPERSOUL2 non trouvé: %s", gamePath)
		http.Error(w, "Jeu non disponible", http.StatusNotFound)
		return
	}

	// Vérifier si l'exécutable existe
	exePath := gamePath + "/ESPERSOUL2.exe"
	if _, err := os.Stat(exePath); err == nil {
		// Envoyer l'exécutable
		log.Printf("✅ Envoi de l'exécutable: %s", exePath)
		w.Header().Set("Content-Disposition", "attachment; filename=ESPERSOUL2.exe")
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, exePath)
		return
	}

	// Si pas d'exe, informer l'utilisateur
	log.Println("⚠️ Aucun exécutable trouvé")
	http.Error(w, "Exécutable non disponible. Le jeu doit être compilé localement.", http.StatusNotFound)
}

=======
>>>>>>> 74104140c5ab87bed33754a1b0ce65a7ae112c8c
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
	http.HandleFunc("/booster-action", boosterActionHandler)

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
