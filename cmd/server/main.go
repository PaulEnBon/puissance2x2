package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	power4 "power4/power4"
	"sync"
)

type GameState struct {
	Board    [6][7]string `json:"board"`
	Next     string       `json:"next"`
	Winner   string       `json:"winner"`
	Finished bool         `json:"finished"`
}

var (
	state = GameState{Next: "R"}
	mu    sync.Mutex
	tmpl  = template.Must(template.New("index.html").Funcs(template.FuncMap{
		"seq": func(a, b int) []int {
			s := make([]int, 0, b-a+1)
			for i := a; i <= b; i++ {
				s = append(s, i)
			}
			return s
		},
	}).ParseFiles("templates/index.html"))
)

func resetBoard() {
	for r := 0; r < 6; r++ {
		for c := 0; c < 7; c++ {
			state.Board[r][c] = ""
		}
	}
	state.Next = "R"
	state.Winner = ""
	state.Finished = false
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	if err := tmpl.Execute(w, state); err != nil {
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
	var col int
	if _, err := fmt.Sscanf(r.FormValue("column"), "%d", &col); err != nil {
		http.Error(w, "Invalid column", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if state.Finished || col < 0 || col > 6 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	placed := false
	for rr := 5; rr >= 0; rr-- {
		if state.Board[rr][col] == "" {
			state.Board[rr][col] = state.Next
			placed = true
			break
		}
	}
	if !placed {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if winner := power4.Winner(state.Board); winner != "" {
		state.Winner = winner
		state.Finished = true
	} else {
		if state.Next == "R" {
			state.Next = "Y"
		} else {
			state.Next = "R"
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func formResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	resetBoard()
	mu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func main() {
	resetBoard()
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/play", formPlayHandler)
	http.HandleFunc("/reset", formResetHandler)
	fs := http.FileServer(http.Dir("templates"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Serveur lancé sur http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
