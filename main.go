package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"power4/game"
	"sync"

	"strings"

	"github.com/gorilla/websocket"
)

var (
	state          = game.GameState{Next: "R", Mode: "exponentiel", Rows: 6, Cols: 7, WinLength: 4}
	mu             sync.Mutex
	playerNames    = []string{"Joueur 1", "Joueur 2"} // Noms des joueurs
	doublePlayNext = false                            // Pour le booster "double-shot"
	blockedColumn  = -1                               // Colonne bloquée par le booster "block-column"
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
	menuTmpl        = template.Must(template.New("menu.html").ParseFiles("templates/menu.html"))
	welcomeTmpl     = template.Must(template.ParseFiles("templates/welcome.html"))
	playersTmpl     = template.Must(template.ParseFiles("templates/players.html"))
	multiplayerTmpl = template.Must(template.ParseFiles("templates/multiplayer.html"))
)

// WebSocket infrastructure
var (
	wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsClients  = map[*websocket.Conn]bool{}
	wsMu       sync.Mutex
)

func wsRegister(conn *websocket.Conn) {
	wsMu.Lock()
	wsClients[conn] = true
	wsMu.Unlock()
}

func wsUnregister(conn *websocket.Conn) {
	wsMu.Lock()
	delete(wsClients, conn)
	wsMu.Unlock()
	_ = conn.Close()
}

func wsBroadcast(v interface{}) {
	// Encode once
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("ws broadcast marshal error: %v", err)
		return
	}
	wsMu.Lock()
	defer wsMu.Unlock()
	for c := range wsClients {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("ws write error, removing client: %v", err)
			delete(wsClients, c)
			_ = c.Close()
		}
	}
}

// getLocalIP retourne l'adresse IP locale de la machine
// Priorités:
// 1) IPv4 privée RFC1918 (192.168.x.x, 10.x.x.x, 172.16-31.x.x) sur interface UP non loopback non-virtuelle
// 2) autre IPv4 non-APIPA (pas 169.254.x.x)
// 3) fallback "localhost"
func getLocalIP() string {
	// Try UDP dial heuristic to discover primary outbound IP
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		if la, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			ip := la.IP.To4()
			_ = conn.Close()
			if ip != nil && !(ip[0] == 169 && ip[1] == 254) && ip[3] != 0 && ip[3] != 255 {
				return ip.String()
			}
		} else {
			_ = conn.Close()
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "localhost"
	}

	// Helpers
	isPrivate := func(ip net.IP) bool {
		if ip == nil {
			return false
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return false
		}
		switch {
		case ip4[0] == 10:
			return true
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return true
		case ip4[0] == 192 && ip4[1] == 168:
			return true
		default:
			return false
		}
	}
	isAPIPA := func(ip net.IP) bool {
		ip4 := ip.To4()
		return ip4 != nil && ip4[0] == 169 && ip4[1] == 254
	}
	isVirtualName := func(name string) bool {
		n := strings.ToLower(name)
		for _, bad := range []string{"vmware", "virtualbox", "vEthernet", "hyper-v", "docker", "container", "loopback"} {
			if strings.Contains(n, strings.ToLower(bad)) {
				return true
			}
		}
		return false
	}

	var firstNonApipa net.IP
	// Pass 1: prefer private IPs
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		if isVirtualName(iface.Name) {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP.To4()
				if ip == nil {
					continue
				}
				// ignorer adresses réseau/broadcast
				if ip[3] == 0 || ip[3] == 255 {
					continue
				}
				if isPrivate(ip) {
					return ip.String()
				}
				if firstNonApipa == nil && !isAPIPA(ip) {
					firstNonApipa = ip
				}
			}
		}
	}
	if firstNonApipa != nil {
		return firstNonApipa.String()
	}
	// Fallback: try any non-loopback IPv4
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip := ipnet.IP.To4(); ip != nil {
					return ip.String()
				}
			}
		}
	}
	return "localhost"
}

// getLocalIPs retourne toutes les IPv4 privées valides des interfaces UP non loopback
func getLocalIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []string
	isPrivate := func(ip net.IP) bool {
		ip4 := ip.To4()
		if ip4 == nil {
			return false
		}
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}
	isVirtualName := func(name string) bool {
		n := strings.ToLower(name)
		for _, bad := range []string{"vmware", "virtualbox", "vethernet", "hyper-v", "docker", "container", "loopback"} {
			if strings.Contains(n, bad) {
				return true
			}
		}
		return false
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		if isVirtualName(iface.Name) {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP.To4()
				if ip == nil {
					continue
				}
				if ip[3] == 0 || ip[3] == 255 {
					continue
				}
				if isPrivate(ip) {
					ips = append(ips, ip.String())
				}
			}
		}
	}
	return ips
}

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
	// En mode turbo, on garde toujours 6x7 (comme classique pour l'instant)
	if state.Mode == "turbo" {
		state.Rows = 6
		state.Cols = 7
		state.WinLength = 4
	}
	// En mode multijoueur, on garde toujours 6x7
	if state.Mode == "multijoueur" {
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
	// Ne pas oublier: on considère ce reset comme un nouvel état
	state.Version++
}

// welcomeHandler affiche l'écran d'accueil avec l'image
func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	if err := welcomeTmpl.Execute(w, nil); err != nil {
		log.Printf("welcome template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// playersHandler affiche le formulaire pour saisir les noms des joueurs
func playersHandler(w http.ResponseWriter, r *http.Request) {
	if err := playersTmpl.Execute(w, nil); err != nil {
		log.Printf("players template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// setPlayersHandler sauvegarde les noms des joueurs
func setPlayersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Récupérer les noms des joueurs
	player1 := r.FormValue("player1")
	player2 := r.FormValue("player2")

	if player1 != "" {
		playerNames[0] = player1
	}
	if player2 != "" {
		playerNames[1] = player2
	}

	log.Printf("Noms des joueurs sauvegardés: %v", playerNames)

	// Rediriger vers le menu de sélection de mode
	http.Redirect(w, r, "/menu", http.StatusSeeOther)
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

// multiplayerHandler affiche la page d'informations multijoueur
func multiplayerHandler(w http.ResponseWriter, r *http.Request) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	data := struct {
		LocalIP     string
		Port        string
		Player1Name string
		Player2Name string
	}{
		LocalIP:     getLocalIP(),
		Port:        port,
		Player1Name: playerNames[0],
		Player2Name: playerNames[1],
	}

	if err := multiplayerTmpl.Execute(w, data); err != nil {
		log.Printf("multiplayer template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// indexHandler renders the current game mode UI
func indexHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// Créer une structure pour passer à la fois le state et les noms des joueurs
	data := struct {
		game.GameState
		Player1Name   string
		Player2Name   string
		BlockedColumn int
	}{
		GameState:     state,
		Player1Name:   playerNames[0],
		Player2Name:   playerNames[1],
		BlockedColumn: blockedColumn,
	}

	if err := indexTmpl.Execute(w, data); err != nil {
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

	// Vérifier si la colonne est bloquée en mode turbo
	if state.Mode == "turbo" && col == blockedColumn {
		log.Printf("Colonne %d bloquée, coup impossible", col)
		blockedColumn = -1 // Débloquer après tentative
		http.Redirect(w, r, "/game", http.StatusSeeOther)
		return
	}

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
		// Changer de joueur sauf si double coup est actif
		if state.Mode == "turbo" && doublePlayNext {
			// Ne pas changer de joueur, désactiver le double coup
			doublePlayNext = false
			log.Printf("[Booster] Double coup utilisé, même joueur rejoue")
		} else {
			if state.Next == "R" {
				state.Next = "Y"
			} else {
				state.Next = "R"
			}
		}
	}
	// Incrémenter la version à chaque coup valide
	state.Version++
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

// nextLevelHandler passe au niveau suivant en mode exponentiel (augmente la difficulté)
func nextLevelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	// En mode exponentiel, on augmente le défi après une victoire
	if state.Mode == "exponentiel" && state.Winner != "" {
		state.WinLength++
		log.Printf("Mode exponentiel: niveau suivant = %d alignés", state.WinLength)
	}
	// En mode classique, on ne change rien

	resetBoard()
	state.Version++
	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

func formResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()

	// Reset complet : retour à la taille et difficulté initiale
	if state.Mode == "exponentiel" {
		state.WinLength = 4
		state.Rows = 6
		state.Cols = 7
		log.Printf("Mode exponentiel: réinitialisation complète à Puissance 4 (6×7)")
	}
	// En mode classique et turbo, on ne change rien (toujours 6×7, 4 alignés)
	if state.Mode == "turbo" {
		state.WinLength = 4
		state.Rows = 6
		state.Cols = 7
		log.Printf("Mode turbo: réinitialisation à Puissance 4 (6×7)")
	}

	resetBoard()
	state.Version++
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
	state.Version++
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
	case "turbo":
		state.Mode = "turbo"
		state.Rows, state.Cols, state.WinLength = 6, 7, 4
		log.Printf("Mode sélectionné: turbo (6×7, 4 alignés + boosters)")
	case "multijoueur":
		state.Mode = "multijoueur"
		state.Rows, state.Cols, state.WinLength = 6, 7, 4
		log.Printf("Mode sélectionné: multijoueur (6×7, 4 alignés en réseau)")
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
	// Changement de mode = nouvelle version
	state.Version++

	// Redirection spéciale pour le mode multijoueur
	if mode == "multijoueur" {
		http.Redirect(w, r, "/multiplayer", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/game", http.StatusSeeOther)
	}
}

// stateHandler renvoie des informations minimales sur l'état pour le polling
func stateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mu.Lock()
	defer mu.Unlock()

	resp := map[string]interface{}{
		"version":  state.Version,
		"finished": state.Finished,
		"winner":   state.Winner,
		"mode":     state.Mode,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// infoHandler renvoie l'IP locale et le port courant pour affichage dynamique
func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	primary := getLocalIP()
	candidates := getLocalIPs()
	// Si primary semble non utile, choisir une candidate si possible
	if (strings.HasPrefix(primary, "169.254.") || primary == "localhost") && len(candidates) > 0 {
		primary = candidates[0]
	}
	payload := map[string]interface{}{
		"localIP":    primary,
		"port":       port,
		"candidates": candidates,
		"hostHeader": r.Host,
	}
	log.Printf("/api/info -> primary=%s candidates=%v port=%s host=%s", primary, candidates, port, r.Host)
	_ = json.NewEncoder(w).Encode(payload)
}

// boosterActionHandler gère les actions des boosters en mode turbo
func boosterActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	action := r.FormValue("action")
	player := r.FormValue("player")

	log.Printf("Booster action: %s by player %s", action, player)

	switch action {
	case "double-shot":
		// Activer le double coup pour le prochain tour
		doublePlayNext = true
		log.Printf("[Booster] Double coup activé pour %s", player)

	case "remove-piece":
		// Retirer un pion adverse
		row := r.FormValue("row")
		col := r.FormValue("col")
		var rowInt, colInt int
		fmt.Sscanf(row, "%d", &rowInt)
		fmt.Sscanf(col, "%d", &colInt)

		if rowInt >= 0 && rowInt < state.Rows && colInt >= 0 && colInt < state.Cols {
			state.Board[rowInt][colInt] = ""
			log.Printf("[Booster] Pion retiré en (%d, %d)", rowInt, colInt)
		}

	case "block-column":
		// Bloquer une colonne
		col := r.FormValue("col")
		fmt.Sscanf(col, "%d", &blockedColumn)
		log.Printf("[Booster] Colonne %d bloquée", blockedColumn)

	case "swap-colors":
		// Échanger deux pions
		row1 := r.FormValue("row1")
		col1 := r.FormValue("col1")
		row2 := r.FormValue("row2")
		col2 := r.FormValue("col2")
		var r1, c1, r2, c2 int
		fmt.Sscanf(row1, "%d", &r1)
		fmt.Sscanf(col1, "%d", &c1)
		fmt.Sscanf(row2, "%d", &r2)
		fmt.Sscanf(col2, "%d", &c2)

		if r1 >= 0 && r1 < state.Rows && c1 >= 0 && c1 < state.Cols &&
			r2 >= 0 && r2 < state.Rows && c2 >= 0 && c2 < state.Cols {
			temp := state.Board[r1][c1]
			state.Board[r1][c1] = state.Board[r2][c2]
			state.Board[r2][c2] = temp
			log.Printf("[Booster] Pions échangés: (%d,%d) <-> (%d,%d)", r1, c1, r2, c2)
		}

	case "wildcard":
		// Placer un pion n'importe où
		row := r.FormValue("row")
		col := r.FormValue("col")
		var rowInt, colInt int
		fmt.Sscanf(row, "%d", &rowInt)
		fmt.Sscanf(col, "%d", &colInt)

		if rowInt >= 0 && rowInt < state.Rows && colInt >= 0 && colInt < state.Cols {
			state.Board[rowInt][colInt] = player
			log.Printf("[Booster] Joker placé en (%d, %d) par %s", rowInt, colInt, player)

			// Vérifier victoire
			if winner := game.WinnerWithLength(state.Board, state.Rows, state.Cols, state.WinLength); winner != "" {
				state.Winner = winner
				state.Finished = true
			} else {
				// Changer de joueur
				if state.Next == "R" {
					state.Next = "Y"
				} else {
					state.Next = "R"
				}
			}
		}
	}

	http.Redirect(w, r, "/game", http.StatusSeeOther)
}

func main() {
	resetBoard()
	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/players", playersHandler)
	http.HandleFunc("/set-players", setPlayersHandler)
	http.HandleFunc("/menu", menuHandler)
	http.HandleFunc("/multiplayer", multiplayerHandler)
	http.HandleFunc("/game", indexHandler)
	http.HandleFunc("/state", stateHandler)
	http.HandleFunc("/api/info", infoHandler)
	http.HandleFunc("/play", formPlayHandler)
	http.HandleFunc("/booster-action", boosterActionHandler)
	http.HandleFunc("/next-level", nextLevelHandler)
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
	localIP := getLocalIP()
	log.Printf("Serveur lancé sur http://localhost:%s", port)
	log.Printf("Adresse réseau local: http://%s:%s", localIP, port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
