package main

import (
	"testing"
	"time"
)

func TestBoundaryArrest(t *testing.T) {
	g := NewGame()
	g.resetMatch()

	// 1. Right boundary: arenaX1 = 340. Col 31 has x = 330.
	// When at x = 330 moving right, nextHead.x = 340.
	// nextHead.x + cellSize = 350 > 340 -> immediate StateGameOver!
	g.snake[0] = Point{x: 330, y: 250}
	g.dir = DirRight
	g.nextDir = DirRight
	g.lastMovedDir = DirRight

	g.tick()

	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver upon touching right boundary, got %v", g.state)
	}

	// 2. Left boundary: arenaX0 = 20. Col 0 has x = 20.
	// When at x = 20 moving left, nextHead.x = 10 < 20 -> immediate StateGameOver!
	g.resetMatch()
	g.snake[0] = Point{x: 20, y: 250}
	g.dir = DirLeft
	g.nextDir = DirLeft
	g.lastMovedDir = DirLeft

	g.tick()

	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver upon touching left boundary, got %v", g.state)
	}

	// 3. Top boundary: arenaY0 = 76. Row 0 has y = 76.
	// When at y = 76 moving up, nextHead.y = 66 < 76 -> immediate StateGameOver!
	g.resetMatch()
	g.snake[0] = Point{x: 180, y: 76}
	g.dir = DirUp
	g.nextDir = DirUp
	g.lastMovedDir = DirUp

	g.tick()

	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver upon touching top boundary, got %v", g.state)
	}

	// 4. Bottom boundary: arenaY1 = 456. Row 37 has y = 446.
	// When at y = 446 moving down, nextHead.y = 456.
	// nextHead.y + cellSize = 466 > 456 -> immediate StateGameOver!
	g.resetMatch()
	g.snake[0] = Point{x: 180, y: 446}
	g.dir = DirDown
	g.nextDir = DirDown
	g.lastMovedDir = DirDown

	g.tick()

	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver upon touching bottom boundary, got %v", g.state)
	}
}

func TestReversalPrevention(t *testing.T) {
	g := NewGame()
	g.resetMatch()

	// Moving Right -> attempting to turn Left must be rejected
	g.lastMovedDir = DirRight
	g.setDirection(DirLeft)
	if g.nextDir == DirLeft {
		t.Fatalf("reversal from Right to Left should have been blocked")
	}

	// Moving Up -> attempting to turn Down must be rejected
	g.lastMovedDir = DirUp
	g.setDirection(DirDown)
	if g.nextDir == DirDown {
		t.Fatalf("reversal from Up to Down should have been blocked")
	}

	// Moving Down -> attempting to turn Up must be rejected
	g.lastMovedDir = DirDown
	g.setDirection(DirUp)
	if g.nextDir == DirUp {
		t.Fatalf("reversal from Down to Up should have been blocked")
	}

	// Moving Left -> attempting to turn Right must be rejected
	g.lastMovedDir = DirLeft
	g.nextDir = DirUp
	g.setDirection(DirRight)
	if g.nextDir == DirRight {
		t.Fatalf("reversal from Left to Right should have been blocked")
	}

	// Rapid double buffering in single tick: Right -> Up (valid) -> Left (reversal of pending vs lastMoved)
	g.lastMovedDir = DirRight
	g.setDirection(DirUp)
	if g.nextDir != DirUp {
		t.Fatalf("turn Right to Up should succeed")
	}
	// Attempt to turn Left: Left is opposite to lastMovedDir (Right), must be blocked!
	g.setDirection(DirLeft)
	if g.nextDir == DirLeft {
		t.Fatalf("reversal to Left should have been blocked even when pending was Up")
	}
}

func TestSelfCollision(t *testing.T) {
	g := NewGame()
	g.resetMatch()

	g.snake = []Point{
		{180, 250},
		{180, 240},
		{170, 240},
		{170, 250},
	}
	g.dir = DirLeft
	g.nextDir = DirLeft
	g.lastMovedDir = DirLeft

	// Next step: (180-10, 250) = (170, 250), which collides with snake[3]
	g.tick()

	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver upon self-collision, got %v", g.state)
	}
}

func TestFoodConsumptionAndPowerFlash(t *testing.T) {
	g := NewGame()
	g.resetMatch()

	initialLen := len(g.snake)
	head := g.snake[0]
	g.food = Point{x: head.x + cellSize, y: head.y}
	g.dir = DirRight
	g.nextDir = DirRight
	g.lastMovedDir = DirRight

	g.tick()

	if g.score != 100 {
		t.Fatalf("expected score 100 after eating regular food, got %d", g.score)
	}
	if len(g.snake) != initialLen+1 {
		t.Fatalf("expected snake length %d, got %d", initialLen+1, len(g.snake))
	}
	if !time.Now().Before(g.flashUntil) {
		t.Fatalf("expected power flash flag to be active for 100ms")
	}
}

func TestBonusFoodSpawningAndDecay(t *testing.T) {
	g := NewGame()
	g.resetMatch()

	// 1. Verify bonus food is initially false
	if g.hasBonusFood {
		t.Fatalf("bonus food should not be active initially")
	}

	// 2. Feed snake 4 pellets to trigger bonus food spawn
	for i := 0; i < 4; i++ {
		head := g.snake[0]
		g.food = Point{x: head.x + cellSize, y: head.y}
		g.dir = DirRight
		g.nextDir = DirRight
		g.lastMovedDir = DirRight
		g.tick()
	}

	if !g.hasBonusFood {
		t.Fatalf("expected bonus food to spawn after 4 pellets")
	}
	if g.bonusPoints != 1000 {
		t.Fatalf("initial bonus points should be 1000, got %d", g.bonusPoints)
	}

	// 3. Test decay in Update()
	// Simulate 3 seconds elapsed (50% of 6s duration)
	g.bonusSpawnTime = time.Now().Add(-3 * time.Second)
	_ = g.Update()
	if g.bonusPoints >= 1000 || g.bonusPoints <= 100 {
		t.Fatalf("expected bonus points to decay between 100 and 1000, got %d", g.bonusPoints)
	}

	// 4. Test bonus food consumption
	head := g.snake[0]
	g.bonusFood = Point{x: head.x + cellSize, y: head.y}
	prevScore := g.score
	expectedAward := g.bonusPoints
	g.dir = DirRight
	g.nextDir = DirRight
	g.lastMovedDir = DirRight

	g.tick()

	if g.score != prevScore+expectedAward {
		t.Fatalf("expected score %d, got %d", prevScore+expectedAward, g.score)
	}
	if g.hasBonusFood {
		t.Fatalf("bonus food should be marked false after consumption")
	}
	if !time.Now().Before(g.floatScoreUntil) {
		t.Fatalf("floating score should be active after eating bonus food")
	}

	// 5. Test expiry when duration exceeded
	g.spawnBonusFood()
	g.bonusSpawnTime = time.Now().Add(-10 * time.Second)
	_ = g.Update()
	if g.hasBonusFood {
		t.Fatalf("bonus food should vanish after timer expires")
	}
}

func TestHighScorePersistenceAndTop10(t *testing.T) {
	g := NewGame()

	// Verify top 10 initial entries exist
	if len(g.highScores) != 10 {
		t.Fatalf("expected 10 high score entries, got %d", len(g.highScores))
	}

	best := g.getBestScore()
	if best <= 0 {
		t.Fatalf("expected positive best score, got %d", best)
	}

	// Record a score higher than all existing scores
	g.score = best + 50
	g.checkAndRecordHighScore()

	if g.getBestScore() != best+50 {
		t.Fatalf("expected new best score %d, got %d", best+50, g.getBestScore())
	}
	if !g.isNewRecord {
		t.Fatalf("expected isNewRecord flag to be true")
	}
	if len(g.highScores) != 10 {
		t.Fatalf("highScores list should strictly maintain top 10, got %d", len(g.highScores))
	}

	// Verify entries are sorted descending
	for i := 1; i < len(g.highScores); i++ {
		if g.highScores[i].Score > g.highScores[i-1].Score {
			t.Fatalf("highScores not sorted: index %d (%d) > index %d (%d)",
				i, g.highScores[i].Score, i-1, g.highScores[i-1].Score)
		}
	}
}

func TestMenuNavigationAndTransitions(t *testing.T) {
	g := NewGame()
	if g.state != StateMenu {
		t.Fatalf("initial state should be StateMenu")
	}

	// 1. Direct Touch on [ NEW GAME ] (Y: 200-240, X: 50-310)
	g.handlePointerDown(180, 220)
	if g.state != StatePlaying {
		t.Fatalf("expected StatePlaying after tapping NEW GAME, got %v", g.state)
	}

	// 2. Tap [ PAUSE ] button (btnUtilLeft: X: 40-170, Y: 472-512)
	g.handlePointerDown(100, 490)
	if g.state != StatePaused {
		t.Fatalf("expected StatePaused after PAUSE, got %v", g.state)
	}

	// 3. Tap [ RESUME ] option inside arena (Y: 220-260, X: 50-310)
	g.handlePointerDown(180, 240)
	if g.state != StatePlaying {
		t.Fatalf("expected StatePlaying after tapping RESUME, got %v", g.state)
	}

	// 4. Tap [ MENU ] utility button (btnUtilRight: X: 190-320, Y: 472-512) -> returns to menu
	g.handlePointerDown(250, 490)
	// From playing, utility right button pauses/opens menu
	if g.state != StatePaused && g.state != StateMenu {
		t.Fatalf("expected StatePaused or StateMenu, got %v", g.state)
	}

	// 5. Test [ VIEW HIGH SCORES ] from Menu
	g.state = StateMenu
	g.handlePointerDown(180, 275) // Y: 255-295 is VIEW HIGH SCORES
	if g.state != StateHighScores {
		t.Fatalf("expected StateHighScores, got %v", g.state)
	}

	// 6. Tap [ BACK TO MENU ] in High Scores screen (Y: 395-435)
	g.handlePointerDown(180, 415)
	if g.state != StateMenu {
		t.Fatalf("expected StateMenu after tapping BACK, got %v", g.state)
	}
}

func TestPentagonalDPadDirectionalAngles(t *testing.T) {
	g := NewGame()
	g.resetMatch()

	// D-pad center is at (180, 615).
	// Test UP touch (180, 560) -> dy = -55
	g.handlePointerDown(180, 560)
	if g.nextDir != DirUp {
		t.Fatalf("expected DirUp from D-pad UP touch, got %v", g.nextDir)
	}

	// Test DOWN touch (180, 670) -> dy = +55
	g.lastMovedDir = DirLeft // neutral direction
	g.handlePointerDown(180, 670)
	if g.nextDir != DirDown {
		t.Fatalf("expected DirDown from D-pad DOWN touch, got %v", g.nextDir)
	}

	// Test LEFT touch (125, 615) -> dx = -55
	g.lastMovedDir = DirUp // neutral direction
	g.handlePointerDown(125, 615)
	if g.nextDir != DirLeft {
		t.Fatalf("expected DirLeft from D-pad LEFT touch, got %v", g.nextDir)
	}

	// Test RIGHT touch (235, 615) -> dx = +55
	g.lastMovedDir = DirUp // neutral direction
	g.handlePointerDown(235, 615)
	if g.nextDir != DirRight {
		t.Fatalf("expected DirRight from D-pad RIGHT touch, got %v", g.nextDir)
	}
}

func TestResponsiveLayout(t *testing.T) {
	g := NewGame()

	// Base 360x740 window
	w, h := g.Layout(360, 740)
	if w != 360 || h != 740 {
		t.Fatalf("expected 360x740, got %dx%d", w, h)
	}

	// Tall 1080x2400 mobile phone (20:9)
	w, h = g.Layout(1080, 2400)
	if w != 360 {
		t.Fatalf("expected width 360, got %d", w)
	}
	if h != 800 {
		t.Fatalf("expected height 800 for 20:9 phone, got %d", h)
	}
}

func TestCenterOKButton(t *testing.T) {
	g := NewGame()
	if g.state != StateMenu {
		t.Fatalf("expected StateMenu")
	}

	// Menu option 0 is "NEW GAME".
	// Pressing the center of the D-pad (180, 615) should trigger the [OK] action!
	g.handlePointerDown(180, 615)
	if g.state != StatePlaying {
		t.Fatalf("expected StatePlaying after pressing center OK button, got %v", g.state)
	}

	// Return to menu
	g.state = StateMenu
	g.menuSelection = 0

	// Navigate down to option 1: "VIEW HIGH SCORES" via D-pad DOWN touch (180, 670)
	g.handlePointerDown(180, 670)
	if g.menuSelection != 1 {
		t.Fatalf("expected menuSelection 1, got %d", g.menuSelection)
	}

	// Press center [OK] button (180, 615) to confirm
	g.handlePointerDown(180, 615)
	if g.state != StateHighScores {
		t.Fatalf("expected StateHighScores after pressing OK on option 1, got %v", g.state)
	}

	// In High Scores, pressing center [OK] returns to Menu
	g.handlePointerDown(180, 615)
	if g.state != StateMenu {
		t.Fatalf("expected StateMenu after pressing OK in HighScores, got %v", g.state)
	}

	// Test left utility button as [SELECT (OK)] (btnUtilLeft: 100, 490)
	g.menuSelection = 0
	g.handlePointerDown(100, 490)
	if g.state != StatePlaying {
		t.Fatalf("expected StatePlaying after pressing SELECT (OK) utility button, got %v", g.state)
	}
}
