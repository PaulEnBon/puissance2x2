package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"power4/game"
	"sync"
)

var (
	state = game.GameState{Next: "R", Mode: "exponentiel", Rows: 6, Cols: 7, WinLength: 4}
	mu    sync.Mutex
)

var (
	indexTmpl = template.Must(template.New("index.html").Funcs(template.FuncMap{
		"seq": func(a, b int) []int {
			s := make([]int, 0, b-a+1)
			for i := a; i <= b; i++ {
				s = append(s, i)
			}
			return s
		},
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
	}).ParseFiles("templates/index.html"))
	menuTmpl    = template.Must(template.New("menu.html").ParseFiles("templates/menu.html"))
	welcomeTmpl = template.Must(template.ParseFiles("templates/welcome.html"))
)

func resetBoard() {
	// En mode exponentiel, on augmente le plateau progressivement
	if state.Mode == "exponentiel" {
		// Chaque victoire ajoute +1 en lignes et colonnes
		state.Rows = 6 + (state.WinLength - 4)
		state.Cols = 7 + (state.WinLength - 4)

		// Limiter à 15x15 max (taille du tableau)
		if state.Rows > 15 {
			state.Rows = 15
		}
		if state.Cols > 15 {
			state.Cols = 15
		}
	}
	// En mode classique, on garde toujours 6x7
	if state.Mode == "classique" {
		state.Rows = 6
		state.Cols = 7
		state.WinLength = 4
	}

	// Nettoyer tout le plateau selon les dimensions actuelles
	for r := 0; r < state.Rows && r < 15; r++ {
		for c := 0; c < state.Cols && c < 15; c++ {
			state.Board[r][c] = ""
		}
	}
	state.Next = "R"
	state.Winner = ""
	state.Finished = false
}

// welcomeHandler affiche l'écran d'accueil avec l'image
func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	if err := welcomeTmpl.Execute(w, nil); err != nil {
		log.Printf("welcome template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// menuHandler shows the base menu to choose a mode
func menuHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	if err := menuTmpl.Execute(w, state); err != nil {
		log.Printf("menu template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// indexHandler renders the current game mode UI
func indexHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	if err := indexTmpl.Execute(w, state); err != nil {
		log.Printf("template execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func formPlayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	colStr := r.FormValue("column")
	log.Printf("Received column value: %q", colStr)

	var col int
	if _, err := fmt.Sscanf(colStr, "%d", &col); err != nil {
		log.Printf("Error parsing column %q: %v", colStr, err)
		http.Error(w, "Invalid column", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	log.Printf("Parsed column: %d, Cols: %d", col, state.Cols)

	if state.Finished || col < 0 || col >= state.Cols {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}
	placed := false
	// Chercher la première case vide depuis le bas (en utilisant state.Rows)
	for rr := state.Rows - 1; rr >= 0; rr-- {
		if state.Board[rr][col] == "" {
			state.Board[rr][col] = state.Next
			placed = true
			break
		}
	}
	if !placed {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}

	// Utiliser WinnerWithLength pour vérifier avec le bon nombre d'alignements
	if winner := game.WinnerWithLength(state.Board, state.Rows, state.Cols, state.WinLength); winner != "" {
		state.Winner = winner
		state.Finished = true
	} else {
		if state.Next == "R" {
			state.Next = "Y"
		} else {
			state.Next = "R"
		}
	}
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

func formResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	// En mode exponentiel, si on a gagné, on augmente le défi
	if state.Mode == "exponentiel" && state.Winner != "" {
		state.WinLength++
		log.Printf("Mode exponentiel: nouveau défi = %d alignés", state.WinLength)
	}
	// En mode classique, on ne change rien (toujours 6×7, 4 alignés)

	resetBoard()
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

// restartModeHandler remet complètement à zéro le mode exponentiel (retour à 6×7, puissance 4)
func restartModeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	// Réinitialiser complètement au départ
	state.WinLength = 4
	state.Rows = 6
	state.Cols = 7
	log.Printf("Mode exponentiel: réinitialisé à Puissance 4 (6×7)")

	resetBoard()
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

// setModeHandler changes the current mode and resets the board, then goes to /game
func setModeHandler(w http.ResponseWriter, r *http.Request) {
	mode := r.FormValue("mode")
	if mode == "" {
		mode = r.URL.Query().Get("mode")
	}
	mu.Lock()
	defer mu.Unlock()
	switch mode {
	case "exponentiel":
		state.Mode = "exponentiel"
		state.Rows, state.Cols, state.WinLength = 6, 7, 4
		log.Printf("Mode sélectionné: exponentiel (6×7, 4 alignés)")
	case "classique":
		state.Mode = "classique"
		state.Rows, state.Cols, state.WinLength = 6, 7, 4
		log.Printf("Mode sélectionné: classique (6×7, 4 alignés)")
	default:
		state.Mode = "exponentiel"
		state.Rows, state.Cols, state.WinLength = 6, 7, 4
		log.Printf("Mode par défaut: exponentiel (6×7, 4 alignés)")
	}
	for r := 0; r < 6; r++ {
		for c := 0; c < 7; c++ {
			state.Board[r][c] = ""
		}
	}
	state.Next = "R"
	state.Winner = ""
	state.Finished = false
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

func main() {
	resetBoard()
	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/menu", menuHandler)
	http.HandleFunc("/game", indexHandler)
	http.HandleFunc("/play", formPlayHandler)
	http.HandleFunc("/reset", formResetHandler)
	http.HandleFunc("/restart-mode", restartModeHandler)
	http.HandleFunc("/set-mode", setModeHandler)
	// Static assets
	fs := http.FileServer(http.Dir("templates"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	imgFs := http.FileServer(http.Dir("Images"))
	http.Handle("/images/", http.StripPrefix("/images/", imgFs))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Serveur lancé sur http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
