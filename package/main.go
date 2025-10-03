package main

import (
	"html/template"
	"log"
	"net/http"
)

type PageData struct {
	Player string
	Board  [][]string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	data := PageData{
		Player: "Joueur 1",
		Board: [][]string{
			{"", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""},
		},
	}

	tmpl.Execute(w, data)
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Serveur lancé sur http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
