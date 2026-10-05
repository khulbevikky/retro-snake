//go:build android || ios

package main

import (
	_ "github.com/ebitengine/gomobile/app"

	"github.com/hajimehoshi/ebiten/v2/mobile"
)

func init() {
	mobile.SetGame(NewGame())
}

// Dummy forces gomobile/ebitenmobile to compile this package.
func Dummy() {}

