//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"power4"
	"sync"
)

// GameState représente l'état courant d'une partie de Puissance 4.
type GameState struct {
	Board     [][]string `json:"board"`
	Rows      int        `json:"rows"`
	Cols      int        `json:"cols"`
	WinLength int        `json:"winLength"`
	Next      string     `json:"next"`
	Winner    string     `json:"winner"`
	Finished  bool       `json:"finished"`
}

var (
	state = GameState{
		Rows:      6,
		Cols:      7,
		WinLength: 4,
		Next:      "R",
	}
	mu sync.Mutex
	// Templates
	welcomeTmpl = template.Must(template.ParseFiles("templates/welcome.html"))
	gameTmpl    = template.Must(template.New("index.html").Funcs(template.FuncMap{
		"seq": func(a, b int) []int {
			s := make([]int, 0, b-a+1)
			for i := a; i <= b; i++ {
				s = append(s, i)
			}
			return s
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}).ParseFiles("templates/index.html"))
)

// resetBoard réinitialise le plateau et l'état de partie.
func resetBoard() {
	// Créer un nouveau plateau avec les dimensions actuelles
	state.Board = make([][]string, state.Rows)
	for r := 0; r < state.Rows; r++ {
		state.Board[r] = make([]string, state.Cols)
	}
	state.Next = "R"
	state.Winner = ""
	state.Finished = false
}

// expandBoard augmente la taille du plateau de 1 case et le nombre de pions à aligner de 1.
func expandBoard() {
	state.Rows++
	state.Cols++
	state.WinLength++
	resetBoard()
}

// welcomeHandler affiche l'écran d'accueil
func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	if err := welcomeTmpl.Execute(w, nil); err != nil {
		log.Printf("welcome template execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// indexHandler rend la page principale (jeu).
// IMPORTANT (bug initial corrigé): Dans le template, à l'intérieur des {{range}}, le point (.) change de valeur.
// Pour accéder au plateau on doit ancrer le contexte racine avec $. Exemple utilisé:  {{ $cell := index $.Board $r $c }}
// Sans cela certaines cases n'étaient pas évaluées correctement et pouvaient ne pas s'afficher.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	if err := gameTmpl.Execute(w, state); err != nil {
		log.Printf("template execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// formPlayHandler gère le dépôt d'un jeton dans une colonne via le formulaire.
func formPlayHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Printf("ParseForm error: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Debug logging
	log.Printf("Received POST /play - Form values: %v", r.Form)

	colStr := r.FormValue("column")
	if colStr == "" {
		log.Printf("Missing 'column' parameter in form")
		http.Error(w, "Bad Request: missing column parameter", http.StatusBadRequest)
		return
	}
	var col int
	if _, err := fmt.Sscanf(colStr, "%d", &col); err != nil {
		log.Printf("Invalid column value: %s", colStr)
		http.Error(w, "Invalid column", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if state.Finished {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}
	if col < 0 || col >= state.Cols {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}

	placed := false
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

	if winner := power4.Winner(state.Board, state.Rows, state.Cols, state.WinLength); winner != "" {
		state.Winner = winner
		state.Finished = true
		// Agrandir la grille pour la prochaine partie
		expandBoard()
	} else {
		if state.Next == "R" {
			state.Next = "Y"
		} else {
			state.Next = "R"
		}
	}
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

// formResetHandler remet la partie à zéro.
func formResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	// Réinitialiser aux dimensions de base
	state.Rows = 6
	state.Cols = 7
	state.WinLength = 4
	resetBoard()
	mu.Unlock()
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

func main() {
	resetBoard()

	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/game", indexHandler)
	http.HandleFunc("/play", formPlayHandler)
	http.HandleFunc("/reset", formResetHandler)

	// Fichiers statiques (CSS, JS)
	fs := http.FileServer(http.Dir("templates"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Images statiques
	imgFs := http.FileServer(http.Dir("Images"))
	http.Handle("/images/", http.StripPrefix("/images/", imgFs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Serveur lancé sur http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
