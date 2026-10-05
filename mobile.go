//go:build android || ios

package main

import (
	"github.com/hajimehoshi/ebiten/v2/mobile"
)

func init() {
	mobile.SetGame(NewGame())
}

// Dummy is required because gomobile/ebitenmobile doesn't compile a package
// that doesn't include any exported functions.
func Dummy() {}
