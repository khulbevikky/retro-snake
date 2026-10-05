package game

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// ---------------------------------------------------------------------------
// Virtual Console Hardware Geometry
// ---------------------------------------------------------------------------

const (
	baseW    = 360
	baseH    = 740
	cellSize = 10

	// Expanded High-Density LCD Arena (32 cols x 38 rows = 1,216 cells)
	arenaX0 = 20
	arenaX1 = 340
	arenaY0 = 76
	arenaY1 = 456

	gridCols = (arenaX1 - arenaX0) / cellSize // 32 columns
	gridRows = (arenaY1 - arenaY0) / cellSize // 38 rows

	tickInterval = 100 * time.Millisecond // 10 ticks-per-second arcade cadence
)

// ---------------------------------------------------------------------------
// Game States & Travel Directions
// ---------------------------------------------------------------------------

const (
	StateMenu = iota
	StatePlaying
	StatePaused
	StateGameOver
	StateHighScores
)

const (
	DirUp = iota
	DirDown
	DirLeft
	DirRight
)

// ---------------------------------------------------------------------------
// Color Palette (Vintage Handheld Hardware & Cyber Neon Accents)
// ---------------------------------------------------------------------------

var (
	colChassis        = color.NRGBA{0x2e, 0x2b, 0x36, 0xff} // console shell plastic chassis
	colFlash          = color.NRGBA{0x4d, 0x1a, 0x70, 0xff} // 1-frame chassis power flash
	colLCD            = color.NRGBA{0x1e, 0x1c, 0x24, 0xff} // deep charcoal purple screen
	colLCDBorder      = color.NRGBA{0x0d, 0x0c, 0x11, 0xff} // thick crisp LCD bezel
	colGridDot        = color.NRGBA{0x27, 0x25, 0x30, 0xff} // subtle retro dot-matrix pixel grid
	colCyan           = color.NRGBA{0x00, 0xe6, 0xff, 0xff} // electrifying Cyber Cyan
	colGold           = color.NRGBA{0xff, 0xd7, 0x00, 0xff} // electric Amber / Gold for high scores
	colOrange         = color.NRGBA{0xfd, 0x97, 0x1f, 0xff} // neon orange (food & head)
	colSnakeGreen     = color.NRGBA{0xa6, 0xe2, 0x2e, 0xff} // snake tail green
	colSnakeCore      = color.NRGBA{0x7e, 0xa8, 0x21, 0xff} // dark green inner chain core
	colWhite          = color.NRGBA{0xff, 0xff, 0xff, 0xff} // spark & eye white
	colBlack          = color.NRGBA{0x00, 0x00, 0x00, 0xff} // pupils & deep accents
	colBtnFace        = color.NRGBA{0x44, 0x40, 0x50, 0xff} // plastic utility button face
	colBtnFaceDown    = color.NRGBA{0x31, 0x2e, 0x3a, 0xff} // utility button pressed down
	colBtnHighlight   = color.NRGBA{0x5f, 0x5a, 0x6e, 0xff} // bevel top/left highlight
	colBtnShadow      = color.NRGBA{0x1b, 0x19, 0x21, 0xff} // bevel bottom/right shadow
	colDpadBody       = color.NRGBA{0x3a, 0x36, 0x44, 0xff} // pentagonal d-pad key body
	colDpadBevelLight = color.NRGBA{0x58, 0x53, 0x66, 0xff} // pentagonal key upper bevel
	colDpadBevelDark  = color.NRGBA{0x22, 0x20, 0x29, 0xff} // pentagonal key lower shadow
	colDpadPressed    = color.NRGBA{0x22, 0x20, 0x29, 0xff} // pentagonal key active pressed
	colDpadGlow       = color.NRGBA{0x00, 0xe6, 0xff, 0xbb} // active press neon glow
	colDpadHub        = color.NRGBA{0x26, 0x24, 0x2e, 0xff} // central pivot hub
	colDpadHubDot     = color.NRGBA{0x15, 0x14, 0x1a, 0xff} // recessed pivot dot
)

// ---------------------------------------------------------------------------
// Hardware Hitbox Casing
// ---------------------------------------------------------------------------

type RectBox struct {
	x0, y0, x1, y1 int
}

func (r RectBox) Contains(x, y int) bool {
	return x >= r.x0 && x < r.x1 && y >= r.y0 && y < r.y1
}

var (
	// Middle Utility Interface Buttons (generous ergonomic touch bounds)
	btnUtilLeft  = RectBox{30, 465, 172, 509}  // [ PAUSE ]
	btnUtilRight = RectBox{188, 465, 330, 509} // [ MENU ]
)

// ---------------------------------------------------------------------------
// Point Coordinate & High Score Storage
// ---------------------------------------------------------------------------

type Point struct {
	x, y int
}

type HighScore struct {
	Score int    `json:"score"`
	Date  string `json:"date"`
}

var defaultHighScores = []HighScore{
	{Score: 250, Date: "2026-09-01"},
	{Score: 200, Date: "2026-09-08"},
	{Score: 160, Date: "2026-09-15"},
	{Score: 130, Date: "2026-09-21"},
	{Score: 100, Date: "2026-09-27"},
	{Score: 80, Date: "2026-10-01"},
	{Score: 60, Date: "2026-10-02"},
	{Score: 40, Date: "2026-10-03"},
	{Score: 30, Date: "2026-10-04"},
	{Score: 20, Date: "2026-10-05"},
}

// ---------------------------------------------------------------------------
// Game Engine
// ---------------------------------------------------------------------------

type Game struct {
	state        int
	score        int
	highScores   []HighScore
	isNewRecord  bool
	snake        []Point
	dir          int // current travel direction
	nextDir      int // buffered next direction
	lastMovedDir int // direction from the most recent tick (prevents 180° turns)
	food         Point
	lastTick     time.Time
	flashUntil   time.Time
	rng          *rand.Rand

	// Menu navigation
	menuSelection int // currently selected menu item index

	// Input tracking (prevents touch bounce)
	activeTouches map[ebiten.TouchID]bool
	pressedBtn    int // visual feedback for button pressed

	// Scaled rendering buffers (zero allocation in Draw)
	scoreBuffer *ebiten.Image
	bestBuffer  *ebiten.Image
	titleBanner *ebiten.Image
	gameOverImg *ebiten.Image
	pausedImg   *ebiten.Image

	// Dynamic Bonus Food & Score Decay Engine
	hasBonusFood    bool
	bonusFood       Point
	bonusSpawnTime  time.Time
	bonusDuration   time.Duration
	bonusMaxPoints  int
	bonusPoints     int
	pelletsEaten    int
	floatScoreText  string
	floatScoreX     float32
	floatScoreY     float32
	floatScoreUntil time.Time
}

// Button visual press identifiers
const (
	pressNone = iota
	pressUtilLeft
	pressUtilRight
	pressDpadUp
	pressDpadDown
	pressDpadLeft
	pressDpadRight
	pressDpadCenter
)

// Pentagonal D-pad center pivot & enlarged touch dimensions
const (
	dpadCenterX     = 180
	dpadCenterY     = 620
	dpadRadius      = 92
	centerBtnRadius = 26
)

// NewGame constructs and initializes a new retro handheld Snake system.
func NewGame() *Game {
	g := &Game{
		state:         StateMenu,
		dir:           DirRight,
		nextDir:       DirRight,
		lastMovedDir:  DirRight,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		activeTouches: make(map[ebiten.TouchID]bool),
		scoreBuffer:   ebiten.NewImage(42, 13),
		bestBuffer:    ebiten.NewImage(42, 13),
	}

	g.loadHighScores()

	// Pre-render static UI titles
	g.titleBanner = renderPixelText("RETRO SNAKE", colSnakeGreen)
	g.gameOverImg = renderPixelText("GAME OVER", colOrange)
	g.pausedImg = renderPixelText("GAME PAUSED", colCyan)

	return g
}

func renderPixelText(str string, clr color.Color) *ebiten.Image {
	w := len(str) * 7
	h := 13
	img := ebiten.NewImage(w, h)
	text.Draw(img, str, basicfont.Face7x13, 0, 11, clr)
	return img
}

// ---------------------------------------------------------------------------
// High Scores Persistence
// ---------------------------------------------------------------------------

func getHighScoreFilePath() string {
	dir, err := os.UserConfigDir()
	if err == nil {
		appDir := filepath.Join(dir, "retro-snake")
		_ = os.MkdirAll(appDir, 0755)
		return filepath.Join(appDir, "highscores.json")
	}
	return "highscores.json"
}

func (g *Game) loadHighScores() {
	filePath := getHighScoreFilePath()
	data, err := os.ReadFile(filePath)
	if err == nil {
		var list []HighScore
		if json.Unmarshal(data, &list) == nil && len(list) > 0 {
			sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
			if len(list) > 10 {
				list = list[:10]
			}
			g.highScores = list
			return
		}
	}

	// Use default high scores and save initial file
	g.highScores = make([]HighScore, len(defaultHighScores))
	copy(g.highScores, defaultHighScores)
	g.saveHighScores()
}

func (g *Game) saveHighScores() {
	filePath := getHighScoreFilePath()
	data, err := json.MarshalIndent(g.highScores, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, data, 0644)
	}
}

func (g *Game) checkAndRecordHighScore() {
	if g.score <= 0 {
		return
	}

	dateStr := time.Now().Format("2006-01-02")
	qualifies := len(g.highScores) < 10 || g.score > g.highScores[len(g.highScores)-1].Score

	if qualifies {
		if len(g.highScores) > 0 && g.score > g.highScores[0].Score {
			g.isNewRecord = true
		}

		g.highScores = append(g.highScores, HighScore{Score: g.score, Date: dateStr})
		sort.Slice(g.highScores, func(i, j int) bool { return g.highScores[i].Score > g.highScores[j].Score })
		if len(g.highScores) > 10 {
			g.highScores = g.highScores[:10]
		}
		g.saveHighScores()
	}
}

func (g *Game) getBestScore() int {
	if len(g.highScores) > 0 {
		return g.highScores[0].Score
	}
	return 0
}

// ---------------------------------------------------------------------------
// Match Reset & Food Spawning
// ---------------------------------------------------------------------------

func (g *Game) resetMatch() {
	g.score = 0
	g.isNewRecord = false
	g.dir = DirRight
	g.nextDir = DirRight
	g.lastMovedDir = DirRight
	g.flashUntil = time.Time{}
	g.hasBonusFood = false
	g.pelletsEaten = 0
	g.floatScoreUntil = time.Time{}

	// Center snake horizontally & vertically inside the matte arena
	midCol := gridCols / 2
	midRow := gridRows / 2
	headX := arenaX0 + midCol*cellSize
	headY := arenaY0 + midRow*cellSize

	g.snake = []Point{
		{headX, headY},
		{headX - cellSize, headY},
		{headX - 2*cellSize, headY},
		{headX - 3*cellSize, headY},
		{headX - 4*cellSize, headY},
	}

	g.spawnFood()
	g.lastTick = time.Now()
	g.state = StatePlaying
}

func (g *Game) spawnFood() {
	if len(g.snake) >= gridCols*gridRows {
		return // Board full
	}

	for {
		col := g.rng.Intn(gridCols)
		row := g.rng.Intn(gridRows)
		pt := Point{
			x: arenaX0 + col*cellSize,
			y: arenaY0 + row*cellSize,
		}

		occupied := false
		for _, s := range g.snake {
			if s == pt {
				occupied = true
				break
			}
		}

		if !occupied {
			g.food = pt
			return
		}
	}
}

func (g *Game) spawnBonusFood() {
	if len(g.snake) >= gridCols*gridRows-1 {
		return // Board full
	}

	for {
		col := g.rng.Intn(gridCols)
		row := g.rng.Intn(gridRows)
		pt := Point{
			x: arenaX0 + col*cellSize,
			y: arenaY0 + row*cellSize,
		}

		if pt == g.food {
			continue
		}

		occupied := false
		for _, s := range g.snake {
			if s == pt {
				occupied = true
				break
			}
		}

		if !occupied {
			g.bonusFood = pt
			g.hasBonusFood = true
			g.bonusSpawnTime = time.Now()
			g.bonusDuration = 6 * time.Second
			g.bonusMaxPoints = 1000
			g.bonusPoints = 1000
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Direction Validation (Strict 180-Degree Suicide Reversal Prevention)
// ---------------------------------------------------------------------------

func isOpposite(d1, d2 int) bool {
	return (d1 == DirUp && d2 == DirDown) ||
		(d1 == DirDown && d2 == DirUp) ||
		(d1 == DirLeft && d2 == DirRight) ||
		(d1 == DirRight && d2 == DirLeft)
}

func (g *Game) setDirection(newDir int) {
	// Must not reverse into the direction traveled in the last completed tick
	if !isOpposite(newDir, g.lastMovedDir) {
		g.nextDir = newDir
	}
}

// ---------------------------------------------------------------------------
// Pointer & Button Interaction
// ---------------------------------------------------------------------------

func (g *Game) handlePointerDown(x, y int) {
	// --- 1. Middle Utility Interface Buttons ---
	if btnUtilLeft.Contains(x, y) {
		g.pressedBtn = pressUtilLeft
		switch g.state {
		case StatePlaying:
			g.state = StatePaused
			g.menuSelection = 0
		case StatePaused, StateMenu, StateGameOver:
			g.triggerMenuAction()
		case StateHighScores:
			g.state = StateMenu
		}
		return
	}

	if btnUtilRight.Contains(x, y) {
		g.pressedBtn = pressUtilRight
		switch g.state {
		case StatePlaying:
			g.state = StatePaused
			g.menuSelection = 0
		case StatePaused, StateGameOver:
			g.state = StateMenu
			g.menuSelection = 0
		case StateMenu:
			g.state = StateHighScores
		case StateHighScores:
			g.resetMatch()
		}
		return
	}

	// --- 2. Interactive Menu Buttons (Inside Arena) ---
	switch g.state {
	case StateMenu:
		// Menu Options: [ NEW GAME ], [ VIEW HIGH SCORES ], [ EXIT ]
		if y >= 200 && y <= 240 && x >= 50 && x <= 310 {
			g.resetMatch()
			return
		}
		if y >= 255 && y <= 295 && x >= 50 && x <= 310 {
			g.state = StateHighScores
			return
		}
		if y >= 310 && y <= 350 && x >= 50 && x <= 310 {
			os.Exit(0)
			return
		}

	case StatePaused:
		// Paused Options: [ RESUME ], [ RESTART ], [ MAIN MENU ]
		if y >= 220 && y <= 260 && x >= 50 && x <= 310 {
			g.state = StatePlaying
			g.lastTick = time.Now()
			return
		}
		if y >= 275 && y <= 315 && x >= 50 && x <= 310 {
			g.resetMatch()
			return
		}
		if y >= 330 && y <= 370 && x >= 50 && x <= 310 {
			g.state = StateMenu
			g.menuSelection = 0
			return
		}

	case StateGameOver:
		// Game Over Options: [ PLAY AGAIN ], [ VIEW HIGH SCORES ], [ MAIN MENU ]
		if y >= 215 && y <= 255 && x >= 50 && x <= 310 {
			g.resetMatch()
			return
		}
		if y >= 270 && y <= 310 && x >= 50 && x <= 310 {
			g.state = StateHighScores
			return
		}
		if y >= 325 && y <= 365 && x >= 50 && x <= 310 {
			g.state = StateMenu
			g.menuSelection = 0
			return
		}

	case StateHighScores:
		// [ BACK TO MENU ] Button at Y: 395 to 435
		if y >= 395 && y <= 435 && x >= 60 && x <= 300 {
			g.state = StateMenu
			return
		}
	}

	// --- 3. Pentagonal D-PAD (Directional Touch Handling & Center OK Button) ---
	dx := float64(x - dpadCenterX)
	dy := float64(y - dpadCenterY)
	dist := math.Sqrt(dx*dx + dy*dy)

	// Central [OK] Button (generous radius <= 30 for thumb comfort on touchscreens)
	if dist <= 30 {
		g.pressedBtn = pressDpadCenter
		if g.state != StatePlaying {
			g.triggerMenuAction()
		}
		return
	}

	if dist > 30 && dist <= float64(dpadRadius+18) {
		var selectedDir int
		if math.Abs(dx) > math.Abs(dy) {
			if dx > 0 {
				selectedDir = DirRight
				g.pressedBtn = pressDpadRight
			} else {
				selectedDir = DirLeft
				g.pressedBtn = pressDpadLeft
			}
		} else {
			if dy > 0 {
				selectedDir = DirDown
				g.pressedBtn = pressDpadDown
			} else {
				selectedDir = DirUp
				g.pressedBtn = pressDpadUp
			}
		}

		// In gameplay: steer snake
		if g.state == StatePlaying {
			g.setDirection(selectedDir)
			return
		}

		// In menus: navigate options
		if selectedDir == DirUp {
			if g.menuSelection > 0 {
				g.menuSelection--
			}
		} else if selectedDir == DirDown {
			if g.menuSelection < 2 {
				g.menuSelection++
			}
		} else if selectedDir == DirRight {
			g.triggerMenuAction()
		}
	}
}

func (g *Game) triggerMenuAction() {
	switch g.state {
	case StateMenu:
		switch g.menuSelection {
		case 0:
			g.resetMatch()
		case 1:
			g.state = StateHighScores
		case 2:
			os.Exit(0)
		}
	case StatePaused:
		switch g.menuSelection {
		case 0:
			g.state = StatePlaying
			g.lastTick = time.Now()
		case 1:
			g.resetMatch()
		case 2:
			g.state = StateMenu
			g.menuSelection = 0
		}
	case StateGameOver:
		switch g.menuSelection {
		case 0:
			g.resetMatch()
		case 1:
			g.state = StateHighScores
		case 2:
			g.state = StateMenu
			g.menuSelection = 0
		}
	case StateHighScores:
		g.state = StateMenu
	}
}

// ---------------------------------------------------------------------------
// Engine Update Loop
// ---------------------------------------------------------------------------

func (g *Game) Update() error {
	// --- Touch Event Processing (ebiten.AppendTouchIDs & TouchPosition) ---
	currTouches := make(map[ebiten.TouchID]bool)
	touchIDs := ebiten.AppendTouchIDs(nil)

	hasAnyTouch := len(touchIDs) > 0
	for _, id := range touchIDs {
		currTouches[id] = true
		if !g.activeTouches[id] {
			tx, ty := ebiten.TouchPosition(id)
			g.handlePointerDown(tx, ty)
		}
	}
	g.activeTouches = currTouches

	// --- Mouse Click Fallback for Desktop Testing ---
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		g.handlePointerDown(mx, my)
	}

	// Release visual press highlights when no touches or mouse clicks active
	if !hasAnyTouch && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.pressedBtn = pressNone
	}

	// --- Keyboard Navigation ---
	if g.state == StatePlaying {
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW):
			g.pressedBtn = pressDpadUp
			g.setDirection(DirUp)
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS):
			g.pressedBtn = pressDpadDown
			g.setDirection(DirDown)
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA):
			g.pressedBtn = pressDpadLeft
			g.setDirection(DirLeft)
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD):
			g.pressedBtn = pressDpadRight
			g.setDirection(DirRight)
		}
	} else {
		// Menu Keyboard Controls
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			if g.menuSelection > 0 {
				g.menuSelection--
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			if g.menuSelection < 2 {
				g.menuSelection++
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyX) {
			g.pressedBtn = pressDpadCenter
			g.triggerMenuAction()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			if g.state == StateHighScores || g.state == StatePaused || g.state == StateGameOver {
				g.state = StateMenu
			}
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		if g.state == StatePlaying {
			g.state = StatePaused
			g.menuSelection = 0
		} else if g.state == StatePaused {
			g.state = StatePlaying
			g.lastTick = time.Now()
		}
	}

	// --- Dynamic Bonus Food Real-Time Decay & Expiry ---
	if g.state == StatePlaying && g.hasBonusFood {
		elapsed := time.Since(g.bonusSpawnTime)
		if elapsed >= g.bonusDuration {
			g.hasBonusFood = false
		} else {
			remRatio := 1.0 - elapsed.Seconds()/g.bonusDuration.Seconds()
			g.bonusPoints = 100 + int(900.0*remRatio)
		}
	}

	// --- Fixed 10 Ticks Per Second Movement Engine ---
	if g.state == StatePlaying {
		if time.Since(g.lastTick) >= tickInterval {
			g.tick()
			g.lastTick = time.Now()
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Arcade Step (Movement, Hitboxes, Food, & Juice)
// ---------------------------------------------------------------------------

func (g *Game) tick() {
	g.dir = g.nextDir
	g.lastMovedDir = g.dir
	head := g.snake[0]

	var nextHead Point
	switch g.dir {
	case DirUp:
		nextHead = Point{head.x, head.y - cellSize}
	case DirDown:
		nextHead = Point{head.x, head.y + cellSize}
	case DirLeft:
		nextHead = Point{head.x - cellSize, head.y}
	case DirRight:
		nextHead = Point{head.x + cellSize, head.y}
	}

	// --- Rigid Boundary Arrest (X: 20-340, Y: 76-456) ---
	if nextHead.x < arenaX0 || nextHead.x+cellSize > arenaX1 ||
		nextHead.y < arenaY0 || nextHead.y+cellSize > arenaY1 {
		g.state = StateGameOver
		g.checkAndRecordHighScore()
		g.menuSelection = 0
		return
	}

	// --- Self Collision Check ---
	for _, seg := range g.snake {
		if nextHead == seg {
			g.state = StateGameOver
			g.checkAndRecordHighScore()
			g.menuSelection = 0
			return
		}
	}

	// Create expanded body chain
	newSnake := make([]Point, len(g.snake)+1)
	newSnake[0] = nextHead
	copy(newSnake[1:], g.snake)

	ateRegularFood := false

	// --- 1. Regular Food Consumption (100 pts, grows snake, triggers power flash) ---
	if nextHead == g.food {
		ateRegularFood = true
		g.snake = newSnake
		g.score += 100
		g.pelletsEaten++
		g.flashUntil = time.Now().Add(tickInterval)
		g.spawnFood()

		// Spawn Bonus Food every 4 regular pellets
		if g.pelletsEaten%4 == 0 && !g.hasBonusFood {
			g.spawnBonusFood()
		}
	}

	// --- 2. Bonus Food Consumption (awards real-time decaying bonus score) ---
	if g.hasBonusFood && nextHead == g.bonusFood {
		g.score += g.bonusPoints
		g.floatScoreText = fmt.Sprintf("+%d", g.bonusPoints)
		g.floatScoreX = float32(g.bonusFood.x) - 10
		g.floatScoreY = float32(g.bonusFood.y) - 6
		g.floatScoreUntil = time.Now().Add(800 * time.Millisecond)
		g.flashUntil = time.Now().Add(tickInterval)
		g.hasBonusFood = false
	}

	// If regular food wasn't consumed, truncate tail
	if !ateRegularFood {
		g.snake = newSnake[:len(newSnake)-1]
	}
}

// ---------------------------------------------------------------------------
// Master Hardware Rendering
// ---------------------------------------------------------------------------

func (g *Game) Draw(screen *ebiten.Image) {
	// --- 1. Console Shell Plastic Chassis Fill ---
	chassisColor := colChassis
	if time.Now().Before(g.flashUntil) {
		chassisColor = colFlash
	}
	screen.Fill(chassisColor)

	// --- 2. Top HUD Header Panel (Score & All-Time High Score) ---
	g.drawTopHUD(screen)

	// --- 3. Matte Virtual Screen Window Arena ---
	const bezel = 4
	vector.DrawFilledRect(screen,
		float32(arenaX0-bezel), float32(arenaY0-bezel),
		float32(arenaX1-arenaX0+2*bezel), float32(arenaY1-arenaY0+2*bezel),
		colLCDBorder, false)

	vector.DrawFilledRect(screen,
		float32(arenaX0), float32(arenaY0),
		float32(arenaX1-arenaX0), float32(arenaY1-arenaY0),
		colLCD, false)

	// --- 4. Game Sprites (Active / Paused / Game Over) ---
	if g.state == StatePlaying || g.state == StatePaused || g.state == StateGameOver {
		g.drawFoodPellet(screen)
		if g.hasBonusFood {
			g.drawBonusFoodPellet(screen)
		}
		g.drawTailSegments(screen)
		g.drawHead(screen)
		g.drawFloatingScore(screen)
	}

	// --- 5. Interactive Screen Overlays (Menu, Paused, Game Over, High Scores) ---
	g.drawScreenOverlays(screen)

	// --- 6. Middle Utility Interface Buttons ---
	leftLabel := "PAUSE"
	rightLabel := "MENU"
	switch g.state {
	case StateMenu:
		leftLabel = "SELECT (OK)"
		rightLabel = "HIGH SCORES"
	case StateHighScores:
		leftLabel = "BACK"
		rightLabel = "PLAY NOW"
	case StatePaused:
		leftLabel = "RESUME"
		rightLabel = "MAIN MENU"
	case StateGameOver:
		leftLabel = "PLAY AGAIN"
		rightLabel = "MAIN MENU"
	case StatePlaying:
		leftLabel = "PAUSE"
		rightLabel = "MENU"
	}
	drawBeveledButton(screen, btnUtilLeft, leftLabel, g.pressedBtn == pressUtilLeft)
	drawBeveledButton(screen, btnUtilRight, rightLabel, g.pressedBtn == pressUtilRight)

	// --- 7. Sleek Pentagonal D-PAD ---
	g.drawPentagonalDPad(screen)
}

// ---------------------------------------------------------------------------
// Top HUD: Score & All-Time High Score
// ---------------------------------------------------------------------------

func (g *Game) drawTopHUD(screen *ebiten.Image) {
	// Left: "SCORE" & 6-digit score in Cyber Cyan (#00e6ff)
	text.Draw(screen, "SCORE", basicfont.Face7x13, 24, 25, colCyan)
	g.scoreBuffer.Clear()
	text.Draw(g.scoreBuffer, fmt.Sprintf("%06d", g.score), basicfont.Face7x13, 0, 11, colCyan)
	scoreOp := &ebiten.DrawImageOptions{}
	scoreOp.Filter = ebiten.FilterNearest
	scoreOp.GeoM.Scale(2, 2)
	scoreOp.GeoM.Translate(24, 32)
	screen.DrawImage(g.scoreBuffer, scoreOp)

	// Center: Live bonus status badge when active
	if g.hasBonusFood && (g.state == StatePlaying || g.state == StatePaused) {
		centerText(screen, "★ BONUS ★", 180, 25, colGold)
		bonusStr := fmt.Sprintf("+%d PTS", g.bonusPoints)
		centerText(screen, bonusStr, 180, 48, colCyan)
	}

	// Right: "HI-SCORE" & absolute all-time high score in Electric Gold (#ffd700)
	bestScore := g.getBestScore()
	text.Draw(screen, "HI-SCORE", basicfont.Face7x13, 246, 25, colGold)
	g.bestBuffer.Clear()
	text.Draw(g.bestBuffer, fmt.Sprintf("%06d", bestScore), basicfont.Face7x13, 0, 11, colGold)
	bestOp := &ebiten.DrawImageOptions{}
	bestOp.Filter = ebiten.FilterNearest
	bestOp.GeoM.Scale(2, 2)
	bestOp.GeoM.Translate(246, 32)
	screen.DrawImage(g.bestBuffer, bestOp)

	// Timer bar outside arena: right between the black upper border and below the scores
	if g.hasBonusFood && (g.state == StatePlaying || g.state == StatePaused) {
		g.drawBonusTimerBar(screen)
	} else {
		// Bezel decorative groove separating HUD and arena when bonus inactive
		vector.DrawFilledRect(screen, 20, 68, 320, 1, color.NRGBA{0x1b, 0x19, 0x21, 0xff}, false)
		vector.DrawFilledRect(screen, 20, 69, 320, 1, color.NRGBA{0x3e, 0x3a, 0x48, 0xff}, false)
	}
}

// ---------------------------------------------------------------------------
// Screen Overlays (Menu, High Scores, Paused, Game Over)
// ---------------------------------------------------------------------------

func (g *Game) drawScreenOverlays(screen *ebiten.Image) {
	centerX := float64(arenaX0+arenaX1) / 2

	switch g.state {
	case StateMenu:
		// Semi-transparent backdrop for menu readability
		vector.DrawFilledRect(screen, float32(arenaX0), float32(arenaY0), float32(arenaX1-arenaX0), float32(arenaY1-arenaY0), color.NRGBA{0x12, 0x11, 0x17, 0xdd}, false)

		// Title Banner
		drawScaledBanner(screen, g.titleBanner, centerX, 120, 2.8)
		centerText(screen, "★ VINTAGE HANDHELD EDITION ★", int(centerX), 155, colGold)

		// Menu Buttons
		drawMenuButton(screen, 70, 200, 220, 40, "NEW GAME", g.menuSelection == 0)
		drawMenuButton(screen, 70, 255, 220, 40, "VIEW HIGH SCORES", g.menuSelection == 1)
		drawMenuButton(screen, 70, 310, 220, 40, "EXIT", g.menuSelection == 2)

		centerText(screen, "NAVIGATE: D-PAD | ACCEPT: [OK]", int(centerX), 425, color.NRGBA{0x80, 0x7c, 0x92, 0xff})

	case StateHighScores:
		vector.DrawFilledRect(screen, float32(arenaX0), float32(arenaY0), float32(arenaX1-arenaX0), float32(arenaY1-arenaY0), color.NRGBA{0x12, 0x11, 0x17, 0xf6}, false)

		centerText(screen, "★ TOP 10 HIGH SCORES ★", int(centerX), 102, colGold)
		vector.DrawFilledRect(screen, float32(arenaX0+20), 114, float32(arenaX1-arenaX0-40), 1, colGold, false)

		// Table Header
		text.Draw(screen, "RANK", basicfont.Face7x13, arenaX0+30, 134, colCyan)
		text.Draw(screen, "SCORE", basicfont.Face7x13, arenaX0+120, 134, colCyan)
		text.Draw(screen, "DATE", basicfont.Face7x13, arenaX0+210, 134, colCyan)

		// Top 10 Entries
		for i := 0; i < 10 && i < len(g.highScores); i++ {
			yPos := 160 + i*22
			entry := g.highScores[i]

			rankClr := colWhite
			if i == 0 {
				rankClr = colGold
			} else if i == 1 {
				rankClr = color.NRGBA{0xdd, 0xdd, 0xdd, 0xff}
			} else if i == 2 {
				rankClr = color.NRGBA{0xcd, 0x7f, 0x32, 0xff}
			}

			rankStr := fmt.Sprintf("#%02d", i+1)
			scoreStr := fmt.Sprintf("%06d", entry.Score)

			text.Draw(screen, rankStr, basicfont.Face7x13, arenaX0+30, yPos, rankClr)
			text.Draw(screen, scoreStr, basicfont.Face7x13, arenaX0+120, yPos, colCyan)
			text.Draw(screen, entry.Date, basicfont.Face7x13, arenaX0+210, yPos, color.NRGBA{0xbb, 0xb8, 0xcc, 0xff})
		}

		// [ BACK TO MENU ] Button
		drawMenuButton(screen, 70, 400, 220, 36, "BACK TO MENU", true)

	case StatePaused:
		vector.DrawFilledRect(screen, float32(arenaX0), float32(arenaY0), float32(arenaX1-arenaX0), float32(arenaY1-arenaY0), color.NRGBA{0x12, 0x11, 0x17, 0xcc}, false)

		drawScaledBanner(screen, g.pausedImg, centerX, 140, 2.5)
		centerText(screen, fmt.Sprintf("CURRENT SCORE: %06d", g.score), int(centerX), 180, colCyan)

		drawMenuButton(screen, 70, 220, 220, 40, "RESUME", g.menuSelection == 0)
		drawMenuButton(screen, 70, 275, 220, 40, "RESTART", g.menuSelection == 1)
		drawMenuButton(screen, 70, 330, 220, 40, "MAIN MENU", g.menuSelection == 2)

	case StateGameOver:
		vector.DrawFilledRect(screen, float32(arenaX0), float32(arenaY0), float32(arenaX1-arenaX0), float32(arenaY1-arenaY0), color.NRGBA{0x12, 0x11, 0x17, 0xcc}, false)

		drawScaledBanner(screen, g.gameOverImg, centerX, 130, 2.8)
		centerText(screen, fmt.Sprintf("FINAL SCORE: %06d", g.score), int(centerX), 165, colCyan)

		if g.isNewRecord {
			centerText(screen, "★ NEW ALL-TIME RECORD! ★", int(centerX), 190, colGold)
		}

		drawMenuButton(screen, 70, 215, 220, 40, "PLAY AGAIN", g.menuSelection == 0)
		drawMenuButton(screen, 70, 270, 220, 40, "VIEW HIGH SCORES", g.menuSelection == 1)
		drawMenuButton(screen, 70, 325, 220, 40, "MAIN MENU", g.menuSelection == 2)
	}
}

func drawMenuButton(screen *ebiten.Image, x, y, w, h float32, label string, isSelected bool) {
	btnBg := color.NRGBA{0x2b, 0x28, 0x36, 0xff}
	borderClr := color.NRGBA{0x4a, 0x46, 0x5a, 0xff}
	labelClr := colWhite

	if isSelected {
		btnBg = color.NRGBA{0x36, 0x32, 0x45, 0xff}
		borderClr = colCyan
		labelClr = colCyan
	}

	vector.DrawFilledRect(screen, x, y, w, h, btnBg, false)
	vector.StrokeRect(screen, x, y, w, h, 2, borderClr, false)

	textW := len(label) * 7
	tx := int(x) + (int(w)-textW)/2
	ty := int(y) + int(h)/2 + 4

	if isSelected {
		text.Draw(screen, "▶", basicfont.Face7x13, tx-16, ty, colCyan)
		text.Draw(screen, "◀", basicfont.Face7x13, tx+textW+6, ty, colCyan)
	}
	text.Draw(screen, label, basicfont.Face7x13, tx, ty, labelClr)
}

func centerText(dst *ebiten.Image, str string, cx, cy int, clr color.Color) {
	tx := cx - len(str)*7/2
	text.Draw(dst, str, basicfont.Face7x13, tx, cy, clr)
}

func drawScaledBanner(screen *ebiten.Image, src *ebiten.Image, cx, cy, scale float64) {
	bounds := src.Bounds()
	w := float64(bounds.Dx()) * scale
	h := float64(bounds.Dy()) * scale

	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(cx-w/2, cy-h/2)
	screen.DrawImage(src, op)
}

// ---------------------------------------------------------------------------
// Sprite Geometric Layering
// ---------------------------------------------------------------------------

func (g *Game) drawFoodPellet(screen *ebiten.Image) {
	cx := float32(g.food.x) + 5
	cy := float32(g.food.y) + 5

	// Outer orange neon circle (#fd971f)
	vector.DrawFilledCircle(screen, cx, cy, 4, colOrange, true)
	// Glowing hot white center spark (#ffffff)
	vector.DrawFilledCircle(screen, cx, cy, 1.5, colWhite, true)
}

func (g *Game) drawBonusFoodPellet(screen *ebiten.Image) {
	cx := float32(g.bonusFood.x) + 5
	cy := float32(g.bonusFood.y) + 5

	// Pulsing energetic halo ring (visibly larger than regular food)
	phase := float64(time.Now().UnixNano()) / 1.2e8
	haloRadius := float32(7.5 + math.Sin(phase)*1.5)
	vector.StrokeCircle(screen, cx, cy, haloRadius, 1.2, colCyan, true)

	// Glowing Cyber Cyan disc (13px across, spans outside cell)
	vector.DrawFilledCircle(screen, cx, cy, 6.5, colCyan, true)
	// Electric Gold star core
	vector.DrawFilledCircle(screen, cx, cy, 3, colGold, true)
	// White-hot center spark
	vector.DrawFilledCircle(screen, cx, cy, 1.2, colWhite, true)
}

func (g *Game) drawBonusTimerBar(screen *ebiten.Image) {
	elapsed := time.Since(g.bonusSpawnTime)
	remRatio := 1.0 - elapsed.Seconds()/g.bonusDuration.Seconds()
	if remRatio < 0 {
		remRatio = 0
	}

	// Located outside arena: Y: 63 to 68, between the scores (Y: 58) and black upper bezel (Y: 72)
	barX := float32(arenaX0)
	barY := float32(63)
	barW := float32(arenaX1 - arenaX0)
	barH := float32(5)

	// Recessed console chassis trough
	vector.DrawFilledRect(screen, barX, barY, barW, barH, color.NRGBA{0x12, 0x10, 0x18, 0xff}, false)
	vector.StrokeRect(screen, barX, barY, barW, barH, 1, color.NRGBA{0x44, 0x3e, 0x54, 0xff}, false)

	// Progress bar fill (decaying Cyber Cyan)
	currentW := (barW - 2) * float32(remRatio)
	if currentW > 0 {
		vector.DrawFilledRect(screen, barX+1, barY+1, currentW, barH-2, colCyan, false)
		if currentW > 2 {
			// Leading bright spark edge
			vector.DrawFilledRect(screen, barX+currentW-1, barY+1, 2, barH-2, colWhite, false)
		}
	}
}

func (g *Game) drawFloatingScore(screen *ebiten.Image) {
	if time.Now().Before(g.floatScoreUntil) {
		rem := g.floatScoreUntil.Sub(time.Now()).Seconds()
		offsetY := float32((0.8 - rem) * 25.0) // drifts upward
		text.Draw(screen, g.floatScoreText, basicfont.Face7x13, int(g.floatScoreX), int(g.floatScoreY-offsetY), colCyan)
	}
}

func (g *Game) drawTailSegments(screen *ebiten.Image) {
	for i := 1; i < len(g.snake); i++ {
		seg := g.snake[i]
		// Outer green 8x8 pixel block inside 10x10 cell (#a6e22e)
		vector.DrawFilledRect(screen,
			float32(seg.x+1), float32(seg.y+1), 8, 8,
			colSnakeGreen, false)
		// Darker green inner core detail (#7ea821) forming physical chain loop
		vector.DrawFilledRect(screen,
			float32(seg.x+3), float32(seg.y+3), 4, 4,
			colSnakeCore, false)
	}
}

func (g *Game) drawHead(screen *ebiten.Image) {
	hd := g.snake[0]
	hx := float32(hd.x)
	hy := float32(hd.y)

	// Base 8x8 orange head block inside 10x10 cell (#fd971f)
	vector.DrawFilledRect(screen, hx+1, hy+1, 8, 8, colOrange, false)

	// Rotating tracking eyes facing current travel direction
	var eye1X, eye1Y, eye2X, eye2Y float32
	var pup1X, pup1Y, pup2X, pup2Y float32

	switch g.dir {
	case DirUp:
		eye1X, eye1Y = hx+1, hy+1
		eye2X, eye2Y = hx+6, hy+1
		pup1X, pup1Y = hx+1, hy+1
		pup2X, pup2Y = hx+6, hy+1
	case DirDown:
		eye1X, eye1Y = hx+1, hy+6
		eye2X, eye2Y = hx+6, hy+6
		pup1X, pup1Y = hx+1, hy+8
		pup2X, pup2Y = hx+6, hy+8
	case DirLeft:
		eye1X, eye1Y = hx+1, hy+1
		eye2X, eye2Y = hx+1, hy+6
		pup1X, pup1Y = hx+1, hy+1
		pup2X, pup2Y = hx+1, hy+6
	case DirRight:
		eye1X, eye1Y = hx+6, hy+1
		eye2X, eye2Y = hx+6, hy+6
		pup1X, pup1Y = hx+8, hy+1
		pup2X, pup2Y = hx+8, hy+6
	}

	vector.DrawFilledRect(screen, eye1X, eye1Y, 3, 3, colWhite, false)
	vector.DrawFilledRect(screen, eye2X, eye2Y, 3, 3, colWhite, false)
	vector.DrawFilledRect(screen, pup1X, pup1Y, 1, 1, colBlack, false)
	vector.DrawFilledRect(screen, pup2X, pup2Y, 1, 1, colBlack, false)
}

// ---------------------------------------------------------------------------
// Hardware Plastic Utility Buttons
// ---------------------------------------------------------------------------

func drawBeveledButton(screen *ebiten.Image, b RectBox, label string, isPressed bool) {
	x := float32(b.x0)
	y := float32(b.y0)
	w := float32(b.x1 - b.x0)
	h := float32(b.y1 - b.y0)

	btnFace := colBtnFace
	topBevel := colBtnHighlight
	botBevel := colBtnShadow

	if isPressed {
		btnFace = colBtnFaceDown
		topBevel = colBtnShadow
		botBevel = colBtnHighlight
	}

	vector.DrawFilledRect(screen, x, y, w, h, btnFace, false)
	vector.DrawFilledRect(screen, x, y, w, 2, topBevel, false)
	vector.DrawFilledRect(screen, x, y, 2, h, topBevel, false)
	vector.DrawFilledRect(screen, x, y+h-2, w, 2, botBevel, false)
	vector.DrawFilledRect(screen, x+w-2, y, 2, h, botBevel, false)

	textW := len(label) * 7
	tx := b.x0 + (int(w)-textW)/2
	ty := b.y0 + int(h)/2 + 4
	if isPressed {
		ty += 1
	}
	text.Draw(screen, label, basicfont.Face7x13, tx, ty, colWhite)
}

// ---------------------------------------------------------------------------
// Sleek Pentagonal D-PAD (Meeting with Pointy Heads in Middle)
// ---------------------------------------------------------------------------

func (g *Game) drawPentagonalDPad(screen *ebiten.Image) {
	cx := float32(dpadCenterX)
	cy := float32(dpadCenterY)

	// Draw outer bezel plate
	vector.DrawFilledCircle(screen, cx, cy, dpadRadius+8, color.NRGBA{0x23, 0x20, 0x2b, 0xff}, true)
	vector.StrokeCircle(screen, cx, cy, dpadRadius+8, 2, color.NRGBA{0x3b, 0x36, 0x46, 0xff}, true)

	// Draw 4 Pentagonal Keypads
	g.drawPentagonKey(screen, DirUp, g.pressedBtn == pressDpadUp)
	g.drawPentagonKey(screen, DirDown, g.pressedBtn == pressDpadDown)
	g.drawPentagonKey(screen, DirLeft, g.pressedBtn == pressDpadLeft)
	g.drawPentagonKey(screen, DirRight, g.pressedBtn == pressDpadRight)

	// Enlarged Central [OK] Button (radius 26, 52px across for easy mobile thumb tap)
	isCenterPressed := g.pressedBtn == pressDpadCenter
	hubBg := colDpadHub
	hubBorder := colDpadBevelLight
	hubTextClr := colCyan
	hubTextY := int(cy) + 5
	if isCenterPressed {
		hubBg = colDpadPressed
		hubBorder = colCyan
		hubTextClr = colWhite
		hubTextY += 1
	}

	vector.DrawFilledCircle(screen, cx, cy, float32(centerBtnRadius), hubBg, true)
	vector.StrokeCircle(screen, cx, cy, float32(centerBtnRadius), 2.5, hubBorder, true)
	text.Draw(screen, "OK", basicfont.Face7x13, int(cx)-7, hubTextY, hubTextClr)
}

// drawPentagonKey renders a 5-sided keycap with pointy tip directed at the center.
func (g *Game) drawPentagonKey(screen *ebiten.Image, dir int, isPressed bool) {
	cx := float32(dpadCenterX)
	cy := float32(dpadCenterY)

	var p0, p1, p2, p3, p4 [2]float32
	var arrowCX, arrowCY float32

	// Geometry: Pentagons with pointy tip meeting inward at radius 28 around enlarged OK button
	switch dir {
	case DirUp:
		p0 = [2]float32{cx, cy - 28}      // Pointy head meeting center button
		p1 = [2]float32{cx - 28, cy - 44} // Bottom-left
		p2 = [2]float32{cx - 40, cy - 88} // Top-left
		p3 = [2]float32{cx + 40, cy - 88} // Top-right
		p4 = [2]float32{cx + 28, cy - 44} // Bottom-right
		arrowCX, arrowCY = cx, cy-64

	case DirDown:
		p0 = [2]float32{cx, cy + 28}      // Pointy head meeting center button
		p1 = [2]float32{cx - 28, cy + 44} // Top-left
		p2 = [2]float32{cx - 40, cy + 88} // Bottom-left
		p3 = [2]float32{cx + 40, cy + 88} // Bottom-right
		p4 = [2]float32{cx + 28, cy + 44} // Top-right
		arrowCX, arrowCY = cx, cy+64

	case DirLeft:
		p0 = [2]float32{cx - 28, cy}      // Pointy head meeting center button
		p1 = [2]float32{cx - 44, cy - 28} // Top-right
		p2 = [2]float32{cx - 88, cy - 40} // Top-left
		p3 = [2]float32{cx - 88, cy + 40} // Bottom-left
		p4 = [2]float32{cx - 44, cy + 28} // Bottom-right
		arrowCX, arrowCY = cx-64, cy

	case DirRight:
		p0 = [2]float32{cx + 28, cy}      // Pointy head meeting center button
		p1 = [2]float32{cx + 44, cy - 28} // Top-left
		p2 = [2]float32{cx + 88, cy - 40} // Top-right
		p3 = [2]float32{cx + 88, cy + 40} // Bottom-right
		p4 = [2]float32{cx + 44, cy + 28} // Bottom-left
		arrowCX, arrowCY = cx+64, cy
	}

	// Press offset towards center
	if isPressed {
		var ox, oy float32
		switch dir {
		case DirUp:
			oy = 2
		case DirDown:
			oy = -2
		case DirLeft:
			ox = 2
		case DirRight:
			ox = -2
		}
		p0[0] += ox
		p0[1] += oy
		p1[0] += ox
		p1[1] += oy
		p2[0] += ox
		p2[1] += oy
		p3[0] += ox
		p3[1] += oy
		p4[0] += ox
		p4[1] += oy
		arrowCX += ox
		arrowCY += oy
	}

	// Build pentagon path
	path := &vector.Path{}
	path.MoveTo(p0[0], p0[1])
	path.LineTo(p1[0], p1[1])
	path.LineTo(p2[0], p2[1])
	path.LineTo(p3[0], p3[1])
	path.LineTo(p4[0], p4[1])
	path.Close()

	bodyClr := colDpadBody
	bevelClr := colDpadBevelLight
	if isPressed {
		bodyClr = colDpadPressed
		bevelClr = colCyan
	}

	dpFill := &vector.DrawPathOptions{AntiAlias: true}
	dpFill.ColorScale.ScaleWithColor(bodyClr)
	vector.FillPath(screen, path, nil, dpFill)

	dpStroke := &vector.DrawPathOptions{AntiAlias: true}
	dpStroke.ColorScale.ScaleWithColor(bevelClr)
	vector.StrokePath(screen, path, &vector.StrokeOptions{Width: 2}, dpStroke)

	// Draw embossed directional chevron arrow
	arrowClr := colWhite
	if isPressed {
		arrowClr = colCyan
	}
	drawEmbossedArrow(screen, dir, arrowCX, arrowCY, arrowClr)
}

func drawEmbossedArrow(dst *ebiten.Image, dir int, cx, cy float32, clr color.Color) {
	switch dir {
	case DirUp:
		for i := 0; i < 5; i++ {
			w := float32(2 + i*4)
			vector.DrawFilledRect(dst, cx-w/2, cy-9+float32(i*4), w, 3, clr, false)
		}
	case DirDown:
		for i := 0; i < 5; i++ {
			w := float32(18 - i*4)
			vector.DrawFilledRect(dst, cx-w/2, cy-9+float32(i*4), w, 3, clr, false)
		}
	case DirLeft:
		for i := 0; i < 5; i++ {
			h := float32(2 + i*4)
			vector.DrawFilledRect(dst, cx-9+float32(i*4), cy-h/2, 3, h, clr, false)
		}
	case DirRight:
		for i := 0; i < 5; i++ {
			h := float32(18 - i*4)
			vector.DrawFilledRect(dst, cx-9+float32(i*4), cy-h/2, 3, h, clr, false)
		}
	}
}

// ---------------------------------------------------------------------------
// Responsive Mobile Layout Engine
// ---------------------------------------------------------------------------

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		return baseW, baseH
	}

	targetH := outsideHeight * baseW / outsideWidth
	if targetH < baseH {
		targetH = baseH
	}

	return baseW, targetH
}


