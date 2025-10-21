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
	Code      string
	CreatedAt time.Time
	State     game.GameState
	Clients   map[*websocket.Conn]bool
	Mu        sync.Mutex
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

	code := generateCode()
	newState := game.GameState{
		Rows: 6, Cols: 7, WinLength: 4,
		Next: "R", Mode: "custom",
	}
	p := &Party{
		Code:      code,
		CreatedAt: time.Now(),
		State:     newState,
		Clients:   make(map[*websocket.Conn]bool),
	}
	parties[code] = p

	log.Printf("✅ Nouvelle partie créée : %s", code)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code})
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

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	p.Mu.Lock()
	p.Clients[conn] = true
	p.Mu.Unlock()

	_ = conn.WriteJSON(p.State)

	go func() {
		defer func() {
			p.Mu.Lock()
			delete(p.Clients, conn)
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
				handlePartyMove(p, col)
			}
		}
	}()
}

func handlePartyMove(p *Party, col int) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	if p.State.Finished || col < 0 || col >= p.State.Cols {
		return
	}

	for r := p.State.Rows - 1; r >= 0; r-- {
		if p.State.Board[r][col] == "" {
			p.State.Board[r][col] = p.State.Next
			break
		}
	}

	if winner := game.WinnerWithLength(p.State.Board, p.State.Rows, p.State.Cols, p.State.WinLength); winner != "" {
		p.State.Winner = winner
		p.State.Finished = true
	} else {
		if p.State.Next == "R" {
			p.State.Next = "Y"
		} else {
			p.State.Next = "R"
		}
	}
	p.State.Version++

	for c := range p.Clients {
		_ = c.WriteJSON(p.State)
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

// ---------------- MAIN ----------------

func main() {
	mrand.Seed(time.Now().UnixNano())

	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/players", playersHandler)
	http.HandleFunc("/set-players", setPlayersHandler)
	http.HandleFunc("/menu", menuHandler)
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
