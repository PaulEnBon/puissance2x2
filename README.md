# 🎮 Puissance 4 en Go

Un jeu complet de **Puissance 4** développé en **Go** avec une interface **HTML/CSS/JS** et un mode **multijoueur en ligne** grâce aux **WebSockets**.

---

## 🚀 Fonctionnalités

- 🧠 **Trois modes Solo**
  - 📈 **Exponentiel** → le plateau grandit à chaque victoire.
  - 🎯 **Classique** → le Puissance 4 traditionnel.
  - ⚡ **Turbo** → avec des boosters (double coup, suppression de pion, blocage de colonne...).

- 👥 **Trois modes Multijoueur**
  - Exponentiel, Classique et Turbo disponibles pour jouer à deux sur le même réseau ou en ligne.

- 🧩 **Parties personnalisées**
  - Crée une partie et partage un **code unique** avec un ami.
  - Rejoins une partie existante avec ce code.
  - Synchronisation en **temps réel** grâce à WebSocket.

- 💻 **Interface moderne**
  - Design sombre, fluide et responsive.
  - Menus intuitifs et animations légères.

---

## 🛠️ Installation et lancement local

### 1️⃣ Cloner le projet
```bash
git clone https://github.com/PaulEnBon/puissance2x2.git
cd puissance4-go

2️⃣ Installer les dépendances

Assure-toi d’avoir Go
 installé (version 1.20 ou supérieure).

Installe le module WebSocket :

go get github.com/gorilla/websocket

3️⃣ Lancer le serveur
go run main.go

Le serveur démarre sur :

http://localhost:8080

Si tu veux accéder au jeux directement clique ici ⬇️
"https://puissance2x2.onrender.com"