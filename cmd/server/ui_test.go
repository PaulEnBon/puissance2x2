package main

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestTemplateRenders42Cells s'assure que le plateau (6x7) génère bien 42 cellules (<td>)
func TestTemplateRenders42Cells(t *testing.T) {
	tmpl := template.Must(template.New("index.html").Funcs(template.FuncMap{
		"seq": func(a, b int) []int {
			s := make([]int, 0, b-a+1)
			for i := a; i <= b; i++ {
				s = append(s, i)
			}
			return s
		},
	}).ParseFiles("templates/index.html"))
	st := GameState{Next: "R"} // plateau vide
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, st); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}
	html := buf.String()
	td := strings.Count(html, "<td>")
	if td != 42 {
		t.Fatalf("expected 42 <td> elements, got %d", td)
	}
}
