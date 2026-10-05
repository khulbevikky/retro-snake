package mobile

import (
	"github.com/hajimehoshi/ebiten/v2/mobile"
	"retro-snake/game"
)

func init() {
	mobile.SetGame(game.NewGame())
}

// Dummy forces ebitenmobile to compile this package.
func Dummy() {}
