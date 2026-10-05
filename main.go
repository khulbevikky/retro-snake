package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"retro-snake/game"
)

func main() {
	ebiten.SetWindowSize(360, 740)
	ebiten.SetWindowTitle("Retro Snake Handheld")

	g := game.NewGame()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
