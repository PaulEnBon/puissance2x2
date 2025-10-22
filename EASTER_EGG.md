# 🎮 EASTER EGG - CODE KONAMI

## Comment activer l'Easter Egg ESPERSOUL2

### 🕹️ Séquence secrète :
Pendant une partie de Puissance 4, tapez cette séquence sur votre clavier :

```
↑ ↑ ↓ ↓ A B A B
```

C'est-à-dire :
1. **Flèche Haut** (2 fois)
2. **Flèche Bas** (2 fois)  
3. **Touche A**
4. **Touche B**
5. **Touche A**
6. **Touche B**

### ✨ Que se passe-t-il ?

Lorsque vous entrez correctement la séquence :

1. 🎨 Un message épique apparaît au centre de l'écran
2. 🚀 Le jeu **ESPERSOUL2** se lance automatiquement dans une nouvelle fenêtre
3. ✅ Une notification de succès s'affiche en haut à droite
4. 🎮 Vous pouvez jouer aux deux jeux en même temps !

### 🎯 Détails techniques

Le code Konami détecte la séquence de touches en temps réel et envoie une requête au serveur via l'API `/api/launch-easteregg`.

Le serveur lance ensuite le jeu ESPERSOUL2 de deux façons :
- Si `ESPERSOUL2.exe` existe → Lance l'exécutable directement
- Sinon → Compile et lance via `go run main.go` dans une nouvelle fenêtre de terminal

### 📁 Structure

```
puissance2x2/
├── main.go                    # Serveur avec API Easter Egg
├── epp4/
│   └── ESPERSOUL2/
│       ├── main.go           # Jeu ESPERSOUL2
│       ├── ESPERSOUL2.exe    # Exécutable (si disponible)
│       └── ...               # Autres fichiers du jeu
└── templates/
    └── index.html            # Code Konami JavaScript
```

### 🎲 Comment jouer

1. Lancez le serveur : `go run main.go`
2. Ouvrez votre navigateur : `http://localhost:8080`
3. Commencez une partie de Puissance 4
4. Tapez le code Konami : ↑ ↑ ↓ ↓ A B A B
5. ESPERSOUL2 se lance ! 🎉

### 🐛 Dépannage

Si l'Easter Egg ne se lance pas :
- Vérifiez que le dossier `epp4/ESPERSOUL2/` existe
- Vérifiez que `main.go` est présent dans ce dossier
- Consultez les logs du serveur pour voir les messages d'erreur
- Assurez-vous que Go est installé et dans votre PATH

### 🎊 Enjoy!

Ce code secret rend hommage au célèbre **Code Konami** utilisé dans de nombreux jeux vidéo classiques !
