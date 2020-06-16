package main

import (
	"fmt"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel/text"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font/basicfont"
)

const maxParticles = 1600
const worldWidth = 1440.0
const worldHeight = 1080.0
const debug = false

var basicFont *text.Atlas

// Particles

type particle struct {
	origin      pixel.Vec
	orientation float64
	scale       pixel.Vec

	colour      pixel.RGBA
	duration    float64
	percentLife float64

	velocity         pixel.Vec
	lengthMultiplier float64
	particleType     string
}

func NewParticle(x float64, y float64, c pixel.RGBA, duration float64, scale pixel.Vec, theta float64, velocity pixel.Vec, lengthMultiplier float64, t string) particle {
	p := particle{}
	p.origin = pixel.V(x, y)
	p.colour = c
	p.duration = duration
	p.scale = scale
	p.orientation = theta
	p.velocity = velocity
	p.lengthMultiplier = lengthMultiplier
	p.particleType = t

	p.percentLife = 1.0
	return p
}

// GRID

type Vector3 struct {
	X float64
	Y float64
	Z float64
}

func v3zero() Vector3 {
	return Vector3{0.0, 0.0, 0.0}
}

func (a Vector3) Add(b Vector3) Vector3 {
	return Vector3{a.X + b.X, a.Y + b.Y, a.Z + b.Z}
}

func (a Vector3) Div(b float64) Vector3 {
	return Vector3{a.X / b, a.Y / b, a.Z / b}
}

func (a Vector3) Sub(b Vector3) Vector3 {
	return Vector3{a.X - b.X, a.Y - b.Y, a.Z - b.Z}
}

func (a Vector3) Mul(b float64) Vector3 {
	return Vector3{a.X * b, a.Y * b, a.Z * b}
}

func (a Vector3) Len() float64 {
	return math.Sqrt(a.LengthSquared())
}

func (a Vector3) LengthSquared() float64 {
	return a.X*a.X + a.Y*a.Y + a.Z*a.Z
}

func (a Vector3) ToVec2(screenSize pixel.Rect) pixel.Vec {
	// hard-coded perspective projection
	factor := (a.Z + 2000) / 2000
	return pixel.V(a.X, a.Y).Sub(screenSize.Max.Scaled(0.5)).Scaled(factor).Add(screenSize.Max.Scaled(0.5))
	// return pixel.V(a.X, a.Y)
}

func randomVector(magnitude float64) pixel.Vec {
	return pixel.V(rand.Float64()-0.5, rand.Float64()-0.5).Unit().Scaled(magnitude)
}

func HSVToColor(h float64, s float64, v float64) pixel.RGBA {
	if h == 0 && s == 0 {
		return pixel.RGBA{v, v, v, 1.0}
	}

	c := s * v
	x := c * (1 - math.Abs(math.Mod(h, 2)-1))
	m := v - c

	if h < 1 {
		return pixel.RGBA{c + m, x + m, m, 1.0}
	} else if h < 2 {
		return pixel.RGBA{x + m, c + m, m, 1.0}
	} else if h < 3 {
		return pixel.RGBA{m, c + m, x + m, 1.0}
	} else if h < 4 {
		return pixel.RGBA{m, x + m, c + m, 1.0}
	} else if h < 5 {
		return pixel.RGBA{x + m, m, c + m, 1.0}
	}

	return pixel.RGBA{c + m, m, x + m, 1.0}
}

type pointMass struct {
	origin       Vector3
	velocity     Vector3
	inverseMass  float64
	acceleration Vector3
	damping      float64
}

func NewPointMass(pos Vector3, invMass float64) *pointMass {
	mass := pointMass{}
	mass.origin = pos
	mass.acceleration = v3zero()
	mass.velocity = v3zero()
	mass.inverseMass = invMass
	mass.damping = 0.98
	return &mass
}

func (pm *pointMass) ApplyForce(force Vector3) {
	pm.acceleration = pm.acceleration.Add(force.Mul(pm.inverseMass))
}

func (pm *pointMass) IncreaseDamping(factor float64) {
	pm.damping *= factor
}

// // spring simulation using "symplectic Euler integration"
// via https://gamedevelopment.tutsplus.com/tutorials/make-a-neon-vector-shooter-in-xna-the-warping-grid--gamedev-9904
func (pm *pointMass) Update() {
	pm.velocity = pm.velocity.Add(pm.acceleration)
	pm.origin = pm.origin.Add(pm.velocity)
	pm.acceleration = v3zero()
	if pm.velocity.LengthSquared() < 0.001*0.001 {
		pm.velocity = v3zero()
	}

	pm.velocity = pm.velocity.Mul(pm.damping)
	pm.damping = 0.98
}

type spring struct {
	end1         *pointMass
	end2         *pointMass
	targetLength float64
	stiffness    float64
	damping      float64
}

func NewSpring(end1 *pointMass, end2 *pointMass, stiffness float64, damping float64) *spring {
	s := spring{
		end1:         end1,
		end2:         end2,
		stiffness:    stiffness,
		damping:      damping,
		targetLength: end1.origin.Sub(end2.origin).Len() * 0.95,
	}
	return &s
}

func (s *spring) Update() {
	x := s.end1.origin.Sub(s.end2.origin)
	len := x.Len()
	if len <= s.targetLength {
		return
	}

	x = x.Div(len).Mul(len - s.targetLength)
	dv := s.end2.velocity.Sub(s.end1.velocity)
	force := x.Mul(s.stiffness).Sub(
		dv.Mul(s.damping),
	)

	s.end1.ApplyForce(force.Mul(-1))
	s.end2.ApplyForce(force)
}

type grid struct {
	springs []*spring
	points  [][]*pointMass
}

func NewGrid(size pixel.Rect, spacing pixel.Vec) grid {
	g := grid{}

	// fmt.Printf(
	// 	"[NewGrid] size: [%f, %f], [%f, %f], spacing: [%f, %f]\n",
	// 	size.Min.X, size.Min.Y, size.Max.X, size.Max.Y, spacing.X, spacing.Y,
	// )
	numCols := int(size.W()/spacing.X) + 1
	numRows := int(size.H()/spacing.Y) + 1

	g.points = make([][]*pointMass, numCols)
	for c := range g.points {
		g.points[c] = make([]*pointMass, numRows)
	}

	// these fixed points will be used to anchor the grid to fixed positions on the screen
	fixedPoints := make([][]*pointMass, numCols)
	for c := range fixedPoints {
		fixedPoints[c] = make([]*pointMass, numRows)
	}

	// create the point masses
	column, row := 0, 0
	for y := size.Min.Y; y <= size.Max.Y; y += spacing.Y {
		for x := size.Min.X; x <= size.Max.X; x += spacing.X {
			g.points[column][row] = NewPointMass(
				Vector3{x, y, 0.0}, 1.0,
			)
			fixedPoints[column][row] = NewPointMass(
				Vector3{x, y, 0.0}, 0.0,
			)
			column++
		}
		row++
		column = 0
	}

	// link the point masses together with springs
	g.springs = make([]*spring, numRows*numCols)
	for y := 0; y < numRows; y++ {
		for x := 0; x < numCols; x++ {
			if x == 0 || y == 0 || x == (numCols-1) || y == (numRows-1) {
				// anchor the border of the grid
				g.springs = append(
					g.springs,
					NewSpring(fixedPoints[x][y], g.points[x][y], 0.1, 0.1),
				)
			} else if x%3 == 0 && y%3 == 0 {
				// loosely anchor 1/9th of the point masses
				g.springs = append(
					g.springs,
					NewSpring(fixedPoints[x][y], g.points[x][y], 0.002, 0.002),
				)
			}

			stiffness := 0.28
			damping := 0.06

			if x > 0 {
				g.springs = append(
					g.springs,
					NewSpring(
						g.points[x-1][y], g.points[x][y], stiffness, damping,
					),
				)
			}
			if y > 0 {
				g.springs = append(
					g.springs,
					NewSpring(
						g.points[x][y-1], g.points[x][y], stiffness, damping,
					),
				)
			}
		}
	}
	return g
}

func (g *grid) Update() {
	for _, s := range g.springs {
		if s != nil {
			s.Update()
		}
	}

	for _, col := range g.points {
		for _, point := range col {
			if point != nil {
				point.Update()
			}
		}
	}
}

func (g *grid) ApplyDirectedForce(force Vector3, origin Vector3, radius float64) {
	for _, col := range g.points {
		for _, point := range col {
			if origin.Sub(point.origin).LengthSquared() < radius*radius {
				point.ApplyForce(
					force.Mul(10).Div(origin.Sub(point.origin).Len() + 10),
				)
			}
		}
	}
}

func (g *grid) ApplyImplosiveForce(force float64, origin Vector3, radius float64) {
	for _, col := range g.points {
		for _, point := range col {
			dist2 := origin.Sub(point.origin).LengthSquared()
			if dist2 < radius*radius {
				forceMultiplier := origin.Sub(point.origin).Mul(10 * force).Div(100 + dist2)
				point.ApplyForce(forceMultiplier)
				point.IncreaseDamping(0.6)
			}
		}
	}
}

func (g *grid) ApplyTightImplosiveForce(force float64, origin Vector3, radius float64) {
	for _, col := range g.points {
		for _, point := range col {
			dist2 := origin.Sub(point.origin).LengthSquared()
			if dist2 < radius*radius {
				forceMultiplier := origin.Sub(point.origin).Mul(100 * force).Div(10000 + dist2)
				point.ApplyForce(forceMultiplier)
				point.IncreaseDamping((0.6))
			}
		}
	}
}

func (g *grid) ApplyExplosiveForce(force float64, origin Vector3, radius float64) {
	for _, col := range g.points {
		for _, point := range col {
			dist2 := origin.Sub(point.origin).LengthSquared()
			if dist2 < radius*radius {
				forceMultiplier := point.origin.Sub(origin).Mul(100 * force).Div(10000 + dist2)
				point.ApplyForce(forceMultiplier)
				point.IncreaseDamping((0.6))
			}
		}
	}
}

// ENTITIES

// For now just using a god entity struct, honestly this is probably fine
type entityData struct {
	target     pixel.Vec
	origin     pixel.Vec
	velocity   pixel.Vec
	speed      float64
	radius     float64
	born       time.Time
	death      time.Time
	expiry     time.Time
	alive      bool
	entityType string
	text       *text.Text
	hp         int

	// sounds
	spawnSound *beep.Buffer

	// enemy data
	bounty       int
	bountyText   string
	killedPlayer bool
}

type bullet struct {
	data     entityData
	velocity pixel.Vec
}

func NewEntity(x float64, y float64, size float64, speed float64, entityType string) *entityData {
	p := new(entityData)
	p.target = pixel.V(0.0, 1.0)
	p.origin = pixel.V(x, y)
	p.radius = size / 2.0
	p.speed = speed
	p.hp = 1
	p.alive = true
	p.entityType = entityType
	p.born = time.Now()
	p.text = text.New(pixel.V(0, 0), basicFont)
	return p
}

func NewFollower(x float64, y float64) *entityData {
	e := NewEntity(x, y, 50.0, 240, "follower")
	e.target = pixel.V(1.0, 1.0)
	e.spawnSound = spawnBuffer
	e.bounty = 50
	return e
}

func NewWanderer(x float64, y float64) *entityData {
	w := NewEntity(x, y, 40.0, 100, "wanderer")
	w.spawnSound = spawnBuffer4
	w.bounty = 25
	return w
}

func NewDodger(x float64, y float64) *entityData {
	w := NewEntity(x, y, 50.0, 240, "dodger")
	w.spawnSound = spawnBuffer2
	w.bounty = 100
	return w
}

func NewPinkSquare(x float64, y float64) *entityData {
	w := NewEntity(x, y, 50.0, 260, "pink")
	w.spawnSound = spawnBuffer5
	w.bounty = 100
	return w
}

func NewPinkPleb(x float64, y float64) *entityData {
	w := NewEntity(x, y, 30.0, 220, "pinkpleb")
	// w.spawnSound = spawnBuffer4
	w.bounty = 75
	return w
}

func NewBlackHole(x float64, y float64) *entityData {
	b := NewEntity(x, y, 75.0, 0.0, "blackhole")
	b.bounty = 150
	b.hp = 10
	return b
}

func NewBullet(x float64, y float64, speed float64, target pixel.Vec) *bullet {
	b := new(bullet)
	b.data = *NewEntity(x, y, 3, speed, "bullet")
	b.data.target = target
	b.velocity = target.Scaled(speed)
	return b
}

// STATE

type wavedata struct {
	waveDuration float64
	waveStart    time.Time
	waveEnd      time.Time
	lastSpawn    time.Time
}

type weapondata struct {
	fireRate float64
	fireMode string
}

type gamedata struct {
	lives           int
	bombs           int
	scoreMultiplier int
	entities        []entityData
	bullets         []bullet
	particles       []particle
	spawns          int
	spawnCount      int
	scoreSinceBorn  int
	player          entityData
	weapon          weapondata
	waves           []wavedata

	score             int
	lifeReward        int
	bombReward        int
	multiplierReward  int
	waveFreq          float64
	weaponUpgradeFreq float64
	spawnFreq         float64
	spawning          bool

	lastSpawn         time.Time
	lastBullet        time.Time
	lastBomb          time.Time
	lastWeaponUpgrade time.Time
}

type game struct {
	state string
	data  gamedata
	grid  grid
}

func NewWaveData() *wavedata {
	waveData := new(wavedata)
	waveData.waveDuration = 5
	waveData.waveStart = time.Time{}
	waveData.waveEnd = time.Now()
	return waveData
}

func NewWeaponData() *weapondata {
	weaponData := new(weapondata)
	weaponData.fireRate = 0.15
	weaponData.fireMode = "normal"

	return weaponData
}

func NewBurstWeapon() *weapondata {
	weaponData := new(weapondata)
	weaponData.fireRate = 0.18
	weaponData.fireMode = "burst"

	return weaponData
}

func NewConicWeapon() *weapondata {
	weaponData := new(weapondata)
	weaponData.fireRate = 0.1
	weaponData.fireMode = "conic"

	return weaponData
}

func NewGameData() *gamedata {
	gameData := new(gamedata)
	gameData.lives = 3
	gameData.bombs = 3
	gameData.scoreMultiplier = 1
	gameData.entities = make([]entityData, 0, 100)
	gameData.bullets = make([]bullet, 0, 100)
	gameData.particles = make([]particle, 0, maxParticles)
	gameData.player = *NewEntity(0.0, 0.0, 50, 400, "player")
	gameData.spawns = 0
	gameData.spawnCount = 1
	gameData.scoreSinceBorn = 0
	gameData.weapon = *NewWeaponData()

	gameData.score = 0
	gameData.multiplierReward = 2000
	gameData.lifeReward = 100000
	gameData.bombReward = 100000
	gameData.waveFreq = 30
	gameData.weaponUpgradeFreq = 30
	gameData.spawnFreq = 1.5
	gameData.spawning = true
	gameData.lastSpawn = time.Now()
	gameData.lastBullet = time.Now()
	gameData.lastBomb = time.Now()
	gameData.lastWeaponUpgrade = time.Time{}

	return gameData
}

func NewGame() *game {
	game := new(game)
	game.state = "starting"
	game.data = *NewGameData()

	maxGridPoints := 1024.0
	buffer := 256.0
	gridSpacing := math.Sqrt(worldWidth * worldHeight / maxGridPoints)
	game.grid = NewGrid(
		pixel.R(
			-worldWidth/2-buffer,
			-worldHeight/2-buffer,
			worldWidth/2+buffer,
			worldHeight/2+buffer,
		),
		pixel.V(
			gridSpacing,
			gridSpacing,
		),
	)

	return game
}

func (data *gamedata) respawnPlayer() {
	data.entities = make([]entityData, 0, 100)
	data.bullets = make([]bullet, 0, 100)
	data.player = *NewEntity(0.0, 0.0, 50, 400, "player")
	data.scoreMultiplier = 1
	data.scoreSinceBorn = 0
}

func (game *game) respawnPlayer() {
	game.data.respawnPlayer()
	game.data.multiplierReward = 2000
}

func thumbstickVector(win *pixelgl.Window, joystick pixelgl.Joystick, axisX pixelgl.GamepadAxis, axisY pixelgl.GamepadAxis) pixel.Vec {
	v := pixel.V(0.0, 0.0)
	if win.JoystickPresent(joystick) {
		x := win.JoystickAxis(pixelgl.Joystick(pixelgl.Joystick1), axisX)
		y := win.JoystickAxis(pixelgl.Joystick(pixelgl.Joystick1), axisY) * -1

		if math.Abs(x) < 0.2 {
			x = 0
		}
		if math.Abs(y) < 0.2 {
			y = 0
		}

		v = pixel.V(x, y)
	}
	return v
}

func drawBullet(bullet *entityData, d *imdraw.IMDraw) {
	d.Push(
		pixel.V(1.0, 6.0),
		pixel.V(1.0, -6.0),
		pixel.V(-1.0, 6.0),
		pixel.V(-1.0, -6.0),
	)
	d.Rectangle(3)
}

func drawShip(d *imdraw.IMDraw) {
	weight := 3.0
	outline := 8.0
	p := pixel.ZV.Add(pixel.V(0.0, -15.0))
	pInner := p.Add(pixel.V(0, outline))
	l1 := p.Add(pixel.V(-10.0, -5.0))
	l1Inner := l1.Add(pixel.V(0, outline))
	r1 := p.Add(pixel.V(10.0, -5.0))
	r1Inner := r1.Add(pixel.V(0.0, outline))
	d.Push(p, l1)
	d.Line(weight)
	d.Push(pInner, l1Inner)
	d.Line(weight)
	d.Push(p, r1)
	d.Line(weight)
	d.Push(pInner, r1Inner)
	d.Line(weight)

	l2 := l1.Add(pixel.V(-15, 20))
	l2Inner := l2.Add(pixel.V(outline, 0.0))
	r2 := r1.Add(pixel.V(15, 20))
	r2Inner := r2.Add(pixel.V(-outline, 0.0))
	d.Push(l1, l2)
	d.Line(weight)
	d.Push(l1Inner, l2Inner)
	d.Line(weight)
	d.Push(r1, r2)
	d.Line(weight)
	d.Push(r1Inner, r2Inner)
	d.Line(weight)

	l3 := l2.Add(pixel.V(15, 25))
	r3 := r2.Add(pixel.V(-15, 25))
	d.Push(l2, l3)
	d.Line(weight)
	d.Push(r2, r3)
	d.Line(weight)
	d.Push(l2Inner, l3)
	d.Line(weight)
	d.Push(r2Inner, r3)
	d.Line(weight)
}

func (e *entityData) Circle() pixel.Circle {
	return pixel.C(e.origin, e.radius)
}

func outOfWorld(v pixel.Vec) bool {
	return v.X < -worldWidth/2 || v.X > worldWidth/2 || v.Y < -worldHeight/2 || v.Y > worldHeight/2
}

func withinWorld(v pixel.Vec) bool {
	return !outOfWorld(v)
}

func enforceWorldBoundary(v *pixel.Vec) { // this code seems dumb, TODO: find some api call that does it
	minX := -(worldWidth / 2.0)
	minY := -(worldHeight / 2.0)
	maxX := (worldWidth / 2.)
	maxY := (worldHeight / 2.0)
	if v.X < minX {
		v.X = minX
	} else if v.X > maxX {
		v.X = maxX
	}
	if v.Y < minY {
		v.Y = minY
	} else if v.Y > maxY {
		v.Y = maxY
	}
}

func (p *entityData) enforceWorldBoundary() { // this code seems dumb, TODO: find some api call that does it
	minX := -(worldWidth / 2.0) + p.radius
	minY := -(worldHeight / 2.0) + p.radius
	maxX := (worldWidth / 2.) - p.radius
	maxY := (worldHeight / 2.0) - p.radius
	if p.origin.X < minX {
		p.origin.X = minX
	} else if p.origin.X > maxX {
		p.origin.X = maxX
	}
	if p.origin.Y < minY {
		p.origin.Y = minY
	} else if p.origin.Y > maxY {
		p.origin.Y = maxY
	}
}

func init() {
	rand.Seed(time.Now().Unix())
}

// resist the urge to refactor. just write a game, don't worry about clean code.
func run() {
	monitor := pixelgl.PrimaryMonitor()
	width, height := monitor.Size()
	cfg := pixelgl.WindowConfig{
		Title:  "Euclidean Combat",
		Bounds: pixel.R(0, 0, width, height),
		// Bounds: pixel.R(0, 0, 1024, 768),
		Monitor:   monitor,
		Maximized: true,
		VSync:     true,
	}

	if debug {
		cfg.Bounds = pixel.R(0, 0, 1024, 768)
		cfg.Maximized = false
		cfg.Monitor = nil
	}

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	// bgColor := color.RGBA{0x16, 0x16, 0x16, 0xff}
	// Draw targets

	mapRect := imdraw.New(nil)
	mapRect.Color = color.RGBA{0x64, 0x64, 0xff, 0xbb}
	mapRect.Push(
		pixel.V(-(worldWidth/2), (worldHeight/2)),
		pixel.V((worldWidth/2), (worldHeight/2)),
	)
	mapRect.Push(
		pixel.V(-(worldWidth/2), -(worldHeight/2)),
		pixel.V((worldWidth/2), -(worldHeight/2)),
	)
	mapRect.Rectangle(4)

	canvas := pixelgl.NewCanvas(pixel.R(-cfg.Bounds.W()/2, -cfg.Bounds.H()/2, cfg.Bounds.W()/2, cfg.Bounds.H()/2))
	imd := imdraw.New(nil)
	playerDraw := imdraw.New(nil)
	bulletDraw := imdraw.New(nil)
	particleDraw := imdraw.New(nil)
	tmpTarget := imdraw.New(nil)
	camPos := pixel.ZV

	// Game initialization
	last := time.Now()

	// Fonts
	basicFont = text.NewAtlas(basicfont.Face7x13, text.ASCII)
	gameOverTxt := text.New(pixel.V(0, 0), basicFont)
	pausedTxt := text.New(pixel.V(0, 0), basicFont)
	scoreTxt := text.New(pixel.V(-(win.Bounds().W()/2)+120, (win.Bounds().H()/2)-50), basicFont)
	livesTxt := text.New(pixel.V(0.0, (win.Bounds().H()/2)-50), basicFont)

	// Input initialization
	currJoystick := pixelgl.Joystick1
	for i := pixelgl.Joystick1; i <= pixelgl.JoystickLast; i++ {
		if win.JoystickPresent(i) {
			currJoystick = i
			fmt.Printf("Joystick Connected: %d\n", i)
			break
		}
	}

	game := NewGame()

	// precache player draw
	drawShip(playerDraw)
	playMusic()

	for !win.Closed() {
		// update
		dt := time.Since(last).Seconds()
		last = time.Now()

		player := &game.data.player

		// lerp the camera position towards the player
		camPos = pixel.Lerp(
			camPos,
			player.origin,
			1-math.Pow(1.0/128, dt),
		)
		cam := pixel.IM.Moved(camPos.Scaled(-1))
		canvas.SetMatrix(cam)

		if game.state == "paused" {
			if win.Pressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
				game.state = "playing"
			}
		}

		if game.state == "starting" {
			game.data = *NewGameData()
			game.state = "playing"
		}

		if game.state == "playing" {
			if !player.alive {
				game.respawnPlayer()
				game.grid.ApplyDirectedForce(Vector3{0.0, 0.0, 5000.0}, Vector3{player.origin.X, player.origin.Y, 0.0}, 50)
			}

			if win.Pressed(pixelgl.KeyP) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonStart) {
				game.state = "paused"
				pausedTxt.Clear()
				line := "Paused."

				pausedTxt.Dot.X -= (pausedTxt.BoundsOf(line).W() / 2)
				fmt.Fprintln(pausedTxt, line)
			}

			// player controls
			direction := pixel.ZV
			player.velocity = pixel.ZV
			if win.Pressed(pixelgl.KeyLeft) || win.Pressed(pixelgl.KeyA) {
				direction = direction.Add(pixel.V(-1, 0))
			}
			if win.Pressed(pixelgl.KeyUp) || win.Pressed(pixelgl.KeyW) {
				direction = direction.Add(pixel.V(0, 1))
			}
			if win.Pressed(pixelgl.KeyRight) || win.Pressed(pixelgl.KeyD) {
				direction = direction.Add(pixel.V(1, 0))
			}
			if win.Pressed(pixelgl.KeyDown) || win.Pressed(pixelgl.KeyS) {
				direction = direction.Add(pixel.V(0, -1))
			}

			if win.JoystickPresent(currJoystick) {
				moveVec := thumbstickVector(
					win,
					currJoystick,
					pixelgl.AxisLeftX,
					pixelgl.AxisLeftY,
				)
				direction = direction.Add(moveVec)
			}

			if direction.Len() > 0.5 {
				targetDt := (pixel.Lerp(
					player.target,
					direction,
					1-math.Pow(1.0/512, dt),
				))
				player.target = targetDt
				player.velocity = direction.Unit().Scaled(player.speed)
			}

			aim := thumbstickVector(win, currJoystick, pixelgl.AxisRightX, pixelgl.AxisRightY)
			// if !win.Pressed(pixelgl.KeySpace) && aim.Len() == 0 {

			// }
			timeSinceBullet := last.Sub(game.data.lastBullet).Seconds()
			timeSinceAbleToShoot := timeSinceBullet - game.data.weapon.fireRate

			if timeSinceAbleToShoot >= 0 {
				if win.Pressed(pixelgl.KeySpace) {
					scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) * (canvas.Bounds().W() / win.Bounds().W())
					scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) * (canvas.Bounds().H() / win.Bounds().H())
					mp := pixel.V(scaledX, scaledY).Add(camPos)
					aim = player.origin.To(mp)
				}

				if aim.Len() > 0 {
					// fmt.Printf("Bullet spawned %s", time.Now().String())
					if game.data.weapon.fireMode == "conic" {
						rad := math.Atan2(aim.Unit().Y, aim.Unit().X)
						ang1 := rad + (10 * (2 * math.Pi) / 360)
						ang2 := rad - (10 * (2 * math.Pi) / 360)
						ang1Vec := pixel.V(math.Cos(ang1), math.Sin(ang1))
						ang2Vec := pixel.V(math.Cos(ang2), math.Sin(ang2))

						leftB := NewBullet(
							player.origin.X,
							player.origin.Y,
							1200, ang1Vec,
						)
						b := NewBullet(
							player.origin.X,
							player.origin.Y,
							1200,
							aim.Unit(),
						)
						rightB := NewBullet(
							player.origin.X,
							player.origin.Y,
							1200,
							ang2Vec,
						)
						game.data.bullets = append(game.data.bullets, *leftB, *b, *rightB)
					} else if game.data.weapon.fireMode == "burst" {
						bulletCount := 5
						width := 40.0
						for i := 0; i < bulletCount; i++ {
							bPos := pixel.V(
								25.0,
								-(width/2)+(float64(i)*(width/float64(bulletCount))),
							).Rotated(
								aim.Angle(),
							).Add(player.origin)

							b := NewBullet(
								bPos.X,
								bPos.Y,
								1100,
								aim.Unit(),
							)
							game.data.bullets = append(game.data.bullets, *b)
						}
					} else {
						b1Pos := pixel.V(5.0, 5.0).Rotated(aim.Angle()).Add(player.origin)
						b2Pos := pixel.V(5.0, -5.0).Rotated(aim.Angle()).Add(player.origin)
						b1 := NewBullet(
							b1Pos.X,
							b1Pos.Y,
							1100,
							aim.Unit(),
						)
						b2 := NewBullet(
							b2Pos.X,
							b2Pos.Y,
							1100,
							aim.Unit(),
						)
						game.data.bullets = append(game.data.bullets, *b1, *b2)
					}

					overflow := timeSinceAbleToShoot * 1000
					if overflow > game.data.weapon.fireRate {
						overflow = 0
					}
					game.data.lastBullet = last
					shot := shotBuffer.Streamer(0, shotBuffer.Len())
					volume := &effects.Volume{
						Streamer: shot,
						Base:     10,
						Volume:   -1.2,
						Silent:   false,
					}
					speaker.Play(volume)
				}
			}

			// set velocities
			for i, e := range game.data.entities {
				if !e.alive {
					continue
				}
				dir := e.origin.To(player.origin).Unit()
				if e.entityType == "wanderer" {
					if e.target.Len() == 0 || e.origin.To(e.target).Len() < 0.2 {
						e.target = pixel.V(
							rand.Float64()*400,
							rand.Float64()*400,
						).Add(e.origin)
					}
					dir = e.origin.To(e.target).Unit()
				} else if e.entityType == "dodger" {
					// https://gamedev.stackexchange.com/questions/109513/how-to-find-if-an-object-is-facing-another-object-given-position-and-direction-a
					// todo, tidy up and put somewhere
					currentlyDodgingDist := -1.0
					for _, b := range game.data.bullets {
						if (b == bullet{}) || !b.data.alive {
							continue
						}
						entToBullet := e.origin.Sub(b.data.origin).Unit()
						if entToBullet.Len() > 500 {
							continue
						}

						facing := entToBullet.Dot(b.data.target.Unit())

						isClosest := (currentlyDodgingDist == -1.0 || entToBullet.Len() < currentlyDodgingDist)
						if facing > 0.0 && facing > 0.80 && facing < 0.95 && isClosest { // if it's basically dead on, they'll die.
							currentlyDodgingDist = entToBullet.Len()

							rad := math.Atan2(entToBullet.Unit().Y, entToBullet.Unit().X)
							dodge1 := rad + (90 * (2 * math.Pi) / 360)
							dodge2 := rad - (90 * (2 * math.Pi) / 360)
							dodge1Worth := math.Abs(b.data.target.Unit().Angle() - dodge1)
							dodge2Worth := math.Abs(b.data.target.Unit().Angle() - dodge2)
							dodgeDirection := dodge1
							if dodge2Worth > dodge1Worth {
								dodgeDirection = dodge2
							}
							dodgeVec := pixel.V(math.Cos(dodgeDirection), math.Sin(dodgeDirection))
							dir = dodgeVec.Scaled(2)
						}
					}
				}
				e.velocity = dir.Scaled(e.speed)
				game.data.entities[i] = e
			}

			for i, b := range game.data.bullets {
				b.data.origin = b.data.origin.Add(b.velocity.Scaled(dt))
				if game.data.weapon.fireMode == "burst" {
					game.grid.ApplyExplosiveForce(b.velocity.Scaled(dt).Len()*2, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 80.0)
				} else {
					game.grid.ApplyExplosiveForce(b.velocity.Scaled(dt).Len()*0.6, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 40.0)
				}

				game.data.bullets[i] = b
			}

			// Process blackholes
			// I don't really care about efficiency atm
			for bID, b := range game.data.entities {
				if !b.alive || b.entityType != "blackhole" {
					continue
				}
				maxForce := 2.0

				game.grid.ApplyImplosiveForce(20, Vector3{b.origin.X, b.origin.Y, 0.0}, 150)

				dist := player.origin.Sub(b.origin)
				length := dist.Len()
				if length <= 250.0 {
					force := pixel.Lerp(
						dist.Scaled(maxForce),
						pixel.ZV,
						length/250.0,
					)
					player.velocity = player.velocity.Sub(force)
				}

				for pID, p := range game.data.particles {
					if p == (particle{}) {
						continue
					}

					dist := b.origin.Sub(p.origin)
					length := dist.Len()

					n := dist.Unit()
					p.velocity = p.velocity.Add(n.Scaled(10000.0 / ((length * length) + 10000.0)))

					if length < 400 {
						p.velocity = p.velocity.Add(pixel.Vec{n.Y, -n.X}.Scaled(45 / (length + 250.0)))
					}
					game.data.particles[pID] = p
				}

				for bulletID, bul := range game.data.bullets {
					if bul.data.alive {
						dist := bul.data.origin.Sub(b.origin)
						length := dist.Len()
						if length > 250.0 {
							continue
						}
						bul.velocity = bul.velocity.Add(dist.Scaled(0.2))
						game.data.bullets[bulletID] = bul
					}
				}

				for eID, e := range game.data.entities {
					if e.alive && eID != bID {
						dist := b.origin.Sub(e.origin)
						length := dist.Len()
						if length > 250.0 {
							continue
						}
						force := pixel.Lerp(
							dist.Scaled(maxForce),
							pixel.ZV,
							length/250.0,
						)
						e.velocity = e.velocity.Add(force)

						intersection := pixel.C(b.origin, b.radius+8.0).Intersect(e.Circle())
						if intersection.Radius > 0 {
							e.alive = false
						}
						game.data.entities[eID] = e
					}
				}
			}

			game.grid.Update()

			// Apply velocities
			player.origin = player.origin.Add(player.velocity.Scaled(dt))

			for i, e := range game.data.entities {
				if !e.alive {
					continue
				}

				e.origin = e.origin.Add(e.velocity.Scaled(dt))
				e.enforceWorldBoundary()
				game.data.entities[i] = e
			}

			// check for collisions
			player.enforceWorldBoundary()

			for id, a := range game.data.entities {
				for id2, b := range game.data.entities {
					if id == id2 {
						continue
					}

					intersection := a.Circle().Intersect(b.Circle())
					if intersection.Radius > 0 {
						a.origin = a.origin.Add(
							b.origin.To(a.origin).Unit().Scaled(intersection.Radius),
						)
						game.data.entities[id] = a
					}
				}
			}

			entsToAdd := make([]entityData, 0, 100)
			for bID, b := range game.data.bullets {
				if b.data.alive {
					for eID, e := range game.data.entities {
						if e.alive && b.data.Circle().Intersect(e.Circle()).Radius > 0 {
							b.data.alive = false
							game.data.bullets[bID] = b

							e.hp -= 1
							if e.hp <= 0 {
								e.alive = false
								e.death = last
								e.expiry = last.Add(time.Millisecond * 300)
								reward := e.bounty * game.data.scoreMultiplier
								e.bountyText = fmt.Sprintf("%d", reward)

								// Draw particles
								hue1 := rand.Float64() * 6.0
								hue2 := math.Mod(hue1+rand.Float64()*1.5, 6.0)
								for i := 0; i < 120; i++ {
									speed := 18 * (1.0 - (1.0 / ((rand.Float64() * 10.0) + 1.0)))
									t := rand.Float64()
									hue := (hue1 * (1.0 - t)) + (hue2 * t)

									p := NewParticle(
										e.origin.X,
										e.origin.Y,
										HSVToColor(hue, 0.5, 1.0),
										64,
										pixel.V(1.5, 1.5),
										0.0,
										randomVector(speed),
										1.8,
										"enemy",
									)

									if len(game.data.particles) < maxParticles {
										game.data.particles = append(game.data.particles, p)
									} else {
										game.data.particles[len(game.data.particles)%maxParticles] = p
									}
								}

								// on kill
								if e.entityType == "pink" {
									// spawn 3 mini plebs
									for i := 0; i < 3; i++ {
										pos := pixel.V(
											rand.Float64()*100,
											rand.Float64()*100,
										).Add(e.origin)
										pleb := *NewPinkPleb(pos.X, pos.Y)
										entsToAdd = append(entsToAdd, pleb)
									}
									// spawnSound := entsToAdd[0].spawnSound.Streamer(0, entsToAdd[0].spawnSound.Len())
									// speaker.Play(spawnSound)
								}

								game.data.score += reward
								game.data.scoreSinceBorn += reward
							} else {
								// still alive
								// if black hole, emit sound particles to indicate damage
								if e.entityType == "blackhole" {
									hue1 := 1.0
									hue2 := hue1 + (rand.Float64() * 0.5) - 0.25
									for i := 0; i < 32; i++ {
										speed := 16 * (1.0 - (1.0 / ((rand.Float64() * 8.0) + 1.0)))
										t := rand.Float64()
										hue := (hue1 * (1.0 - t)) + (hue2 * t)

										p := NewParticle(
											e.origin.X,
											e.origin.Y,
											HSVToColor(hue, 0.5, 1.0),
											64,
											pixel.V(1.0, 1.0),
											0.0,
											randomVector(speed),
											1.0,
											"enemy",
										)

										game.data.particles = append(game.data.particles, p)
									}

								}
							}
							game.data.entities[eID] = e
							break
						}
					}
					if !pixel.R(-worldWidth/2, -worldHeight/2, worldWidth/2, worldHeight/2).Contains(b.data.origin) {

						// explode bullets when they hit the edge
						for i := 0; i < 30; i++ {
							p := NewParticle(b.data.origin.X, b.data.origin.Y, pixel.ToRGBA(colornames.Lightblue), 32, pixel.Vec{1.0, 1.0}, 0.0, randomVector(5.0), 1.0, "bullet")
							game.data.particles = append(game.data.particles, p)
						}

						b.data.alive = false
						game.data.bullets[bID] = b
					}
				}
			}

			game.data.entities = append(game.data.entities, entsToAdd...)

			for eID, e := range game.data.entities {
				if e.alive && e.Circle().Intersect(player.Circle()).Radius > 0 {
					e.killedPlayer = true
					game.data.entities[eID] = e
					game.data.lives -= 1
					player.alive = false
					player.death = last

					for i := 0; i < 1200; i++ {
						speed := 24.0 * (1.0 - 1/((rand.Float64()*32.0)+1))
						p := NewParticle(player.origin.X, player.origin.Y, pixel.ToRGBA(colornames.Lightyellow), 100, pixel.Vec{1.5, 1.5}, 0.0, randomVector(speed), 2.5, "player")
						game.data.particles = append(game.data.particles, p)
					}

					if game.data.lives == 0 {
						game.state = "game_over"

						gameOverTxt.Clear()
						lines := []string{
							"Game Over.",
							"Score: " + fmt.Sprintf("%d", game.data.score),
							"Press enter to restart",
						}
						for _, line := range lines {
							gameOverTxt.Dot.X -= (gameOverTxt.BoundsOf(line).W() / 2)
							fmt.Fprintln(gameOverTxt, line)
						}
					}
				}
			}

			// check for bomb here for now
			bombPressed := win.Pressed(pixelgl.KeyR) || win.JoystickAxis(currJoystick, pixelgl.AxisRightTrigger) > 0.1
			if game.data.bombs > 0 && bombPressed && last.Sub(game.data.lastBomb).Seconds() > 3.0 {
				game.grid.ApplyExplosiveForce(256.0, Vector3{player.origin.X, player.origin.Y, 0.0}, 800)
				sound := bombBuffer.Streamer(0, bombBuffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   0.7,
					Silent:   false,
				}
				speaker.Play(volume)
				game.data.lastBomb = time.Now()

				game.data.bombs -= 1
				for eID, e := range game.data.entities {
					e.alive = false
					e.death = last
					e.expiry = last
					game.data.entities[eID] = e
				}
			}

			for pID, p := range game.data.particles {
				if p != (particle{}) {
					p.origin = p.origin.Add(p.velocity)

					minX := -worldWidth / 2
					minY := -worldHeight / 2
					maxX := worldWidth / 2
					maxY := worldHeight / 2
					// collide with the edges of the screen
					if p.origin.X < minX {
						p.origin.X = minX
						p.velocity.X = math.Abs(p.velocity.X)
					} else if p.origin.X > maxX {
						p.origin.X = maxX
						p.velocity.X = -math.Abs(p.velocity.X)
					}
					if p.origin.Y < minY {
						p.origin.Y = minY
						p.velocity.Y = math.Abs(p.velocity.Y)
					} else if p.origin.Y > maxY {
						p.origin.Y = maxY
						p.velocity.Y = -math.Abs(p.velocity.Y)
					}

					p.orientation = p.velocity.Angle()

					p.percentLife -= 1.0 / p.duration

					speed := p.velocity.Len()
					alpha := math.Min(1, math.Min(p.percentLife*2, speed*1.0))
					alpha *= alpha
					p.colour.A = alpha
					p.scale.X = p.lengthMultiplier * math.Min(math.Min(1.0, 0.2*speed+0.1), alpha)

					if math.Abs(p.velocity.X)+math.Abs(p.velocity.Y) < 0.00000001 {
						p.velocity = pixel.ZV
					}

					p.velocity = p.velocity.Scaled(0.97)

					game.data.particles[pID] = p
				}
			}

			// kill particles
			newParticles := make([]particle, 0, maxParticles)
			for _, p := range game.data.particles {
				if p.percentLife > 0 {
					newParticles = append(newParticles, p)
				}
			}
			game.data.particles = newParticles

			// kill entities
			newEntities := make([]entityData, 0, 100)
			for _, e := range game.data.entities {
				if e.alive || (e.expiry != time.Time{} && last.Before(e.expiry)) {
					newEntities = append(newEntities, e)
				}
			}
			game.data.entities = newEntities

			// kill bullets
			newBullets := make([]bullet, 100)
			for _, b := range game.data.bullets {
				if b.data.alive || b.data.born.After(time.Now().Add(time.Duration(-10)*time.Second)) {
					newBullets = append(newBullets, b)
				}
			}
			game.data.bullets = newBullets

			// spawn entities

			// ambient spawns
			if last.Sub(game.data.lastSpawn).Seconds() > game.data.spawnFreq && game.data.spawning {
				// spawn
				spawns := make([]entityData, 0, game.data.spawnCount)
				for i := 0; i < game.data.spawnCount; i++ {
					pos := pixel.V(
						float64(rand.Intn(worldWidth)-worldWidth/2),
						float64(rand.Intn(worldHeight)-worldHeight/2),
					)
					for pos.Sub(player.origin).Len() < 300 {
						pos = pixel.V(
							float64(rand.Intn(worldWidth)-worldWidth/2),
							float64(rand.Intn(worldHeight)-worldHeight/2),
						)
					}

					var enemy entityData
					r := rand.Float64()
					if r < 0.4 {
						enemy = *NewFollower(
							pos.X,
							pos.Y,
						)
						// spawnSound := enemy.spawnSound.Streamer(0, enemy.spawnSound.Len())
						// speaker.Play(spawnSound)
					} else if r < 0.6 {
						enemy = *NewWanderer(
							pos.X,
							pos.Y,
						)
						// spawnSound := enemy.spawnSound.Streamer(0, enemy.spawnSound.Len())
						// speaker.Play(spawnSound)
					} else if r < 0.9 {
						enemy = *NewPinkSquare(
							pos.X,
							pos.Y,
						)
						// spawnSound := enemy.spawnSound.Streamer(0, enemy.spawnSound.Len())
						// volume := &effects.Volume{
						// 	Streamer: spawnSound,
						// 	Base:     10,
						// 	Volume:   0.2,
						// 	Silent:   false,
						// }
						// speaker.Play(volume)
					} else if r < 0.95 {
						enemy = *NewDodger(
							pos.X,
							pos.Y,
						)
						// spawnSound := enemy.spawnSound.Streamer(0, enemy.spawnSound.Len())
						// speaker.Play(spawnSound)
					} else {
						enemy = *NewBlackHole(
							pos.X,
							pos.Y,
						)
						// spawnSound := enemy.spawnSound.Streamer(0, enemy.spawnSound.Len())
						// speaker.Play(spawnSound)
					}

					spawns = append(spawns, enemy)
				}

				playback := map[string]bool{}
				for _, e := range spawns {
					if playback[e.entityType] || e.spawnSound == nil {
						continue
					}
					playback[e.entityType] = true

					spawnSound := e.spawnSound.Streamer(0, e.spawnSound.Len())
					volume := &effects.Volume{
						Streamer: spawnSound,
						Base:     10,
						Volume:   -0.6,
						Silent:   false,
					}
					speaker.Play(volume)
				}
				game.data.entities = append(game.data.entities, spawns...)
				game.data.spawns += 1
				game.data.lastSpawn = time.Now()
				if game.data.spawns%20 == 0 && game.data.spawnCount < 4 {
					game.data.spawnCount += 1
				}

				if game.data.spawns%10 == 0 && game.data.spawnFreq > 0.6 {
					game.data.spawnFreq -= 0.1
				}
			}

			// wave management
			// if (waveStart == time.Time{}) && last.Sub(waveEnd).Seconds() >= waveFreq { // or the
			// 	// Start the next wave
			// 	fmt.Printf("[WaveStart] %s\n", time.Now().String())
			// 	waveStart = time.Now()
			// }

			// if (waveStart != time.Time{}) && last.Sub(waveStart).Seconds() >= waveDuration { // If a wave has ended
			// 	// End the wave
			// 	fmt.Printf("[WaveEnd] %s\n", time.Now().String())
			// 	waveEnd = time.Now()
			// 	waveStart = time.Time{}
			// }

			// if last.Sub(waveStart).Seconds() < waveDuration {
			// 	// Continue wave
			// 	// TODO make these data driven
			// 	// waves would have spawn points, and spawn counts, and probably durations and stuff
			// 	// hardcoded for now :D

			// 	if last.Sub(lastWaveSpawn).Seconds() > 0.2 {

			// 		// 4 spawn points
			// 		points := [4]pixel.Vec{
			// 			pixel.V(-(worldWidth/2)+50, -(worldHeight/2)+50),
			// 			pixel.V(-(worldWidth/2)+50, (worldHeight/2)-50),
			// 			pixel.V((worldWidth/2)-50, -(worldHeight/2)+50),
			// 			pixel.V((worldWidth/2)-50, (worldHeight/2)-50),
			// 		}

			// 		for _, p := range points {
			// 			enemy := NewEntity(
			// 				p.X,
			// 				p.Y,
			// 				50,
			// 				130,
			// 			)
			// 			entities = append(entities, *enemy)
			// 			spawns += 1
			// 		}
			// 		spawnSound := spawnBuffer.Streamer(0, spawnBuffer.Len())
			// 		speaker.Play(spawnSound)
			// 		lastWaveSpawn = time.Now()
			// 	}
			// }

			// adjust game rules

			if game.data.score >= game.data.lifeReward {
				game.data.lifeReward += 100000
				game.data.lives += 1
				sound := lifeBuffer.Streamer(0, lifeBuffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   -0.9,
					Silent:   false,
				}
				speaker.Play(volume)
			}

			if game.data.score >= game.data.bombReward {
				game.data.bombReward += 100000
				game.data.bombs += 1
			}

			if game.data.scoreSinceBorn >= game.data.multiplierReward && game.data.scoreMultiplier < 10 {
				game.data.scoreMultiplier += 1
				game.data.multiplierReward *= 2
				buffer := multiplierSounds[game.data.scoreMultiplier]
				sound := buffer.Streamer(0, buffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   -0.6,
					Silent:   false,
				}
				speaker.Play(volume)
			}

			timeToUpgrade := game.data.score >= 10000 && game.data.lastWeaponUpgrade == time.Time{}
			if timeToUpgrade || (game.data.lastWeaponUpgrade != time.Time{} && last.Sub(game.data.lastWeaponUpgrade).Seconds() >= game.data.weaponUpgradeFreq) {
				fmt.Printf("[UpgradingWeapon]\n")
				game.data.lastWeaponUpgrade = time.Now()
				switch rand.Intn(2) {
				case 0:
					game.data.weapon = *NewBurstWeapon()
				case 1:
					game.data.weapon = *NewConicWeapon()
				}
			}

		} else {
			if win.Pressed(pixelgl.KeyEnter) || win.JoystickPressed(currJoystick, pixelgl.ButtonA) {
				game.state = "starting"
			}
		}

		// draw
		imd.Clear()

		if game.state == "playing" {

			// Draw the grid effect
			// TODO, extract?
			// Add catmullrom splines?
			{
				width := len(game.grid.points)
				height := len(game.grid.points[0])
				imd.SetColorMask(pixel.Alpha(0.4))
				imd.Color = color.RGBA{30, 30, 139, 255} // The alpha component here doesn't seem to be respected :/

				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						left, up := pixel.ZV, pixel.ZV
						p := game.grid.points[x][y].origin.ToVec2(cfg.Bounds)

						// fmt.Printf("Drawing point %f %f\n", p.X, p.Y)
						if x > 0 {
							left = game.grid.points[x-1][y].origin.ToVec2(cfg.Bounds)
							if withinWorld(p) || withinWorld(left) {
								// It's possible that one but not the other point is brought in from out of the world boundary
								// If being brought in from out of the world, render right on the border
								enforceWorldBoundary(&p)
								enforceWorldBoundary(&left)
								thickness := 1.0
								if y%3 == 1 {
									thickness = 2.0
								}
								imd.Push(left, p)
								imd.Line(thickness)
							}
						}
						if y > 0 {
							up = game.grid.points[x][y-1].origin.ToVec2(cfg.Bounds)
							if withinWorld(p) || withinWorld(up) {
								// It's possible that one but not the other point is brought in from out of the world boundary
								// If being brought in from out of the world, render right on the border
								enforceWorldBoundary(&p)
								enforceWorldBoundary(&up)
								thickness := 1.0
								if y%3 == 1 {
									thickness = 3.0
								}
								imd.Push(up, p)
								imd.Line(thickness)
							}
						}

						if x > 0 && y > 0 {
							upLeft := game.grid.points[x-1][y-1].origin.ToVec2(cfg.Bounds)
							p1, p2 := upLeft.Add(up).Scaled(0.5), left.Add(p).Scaled(0.5)

							if withinWorld(p1) || withinWorld(p2) {
								enforceWorldBoundary(&p1)
								enforceWorldBoundary(&p2)
								imd.Push(p1, p2)
								imd.Line(1.0)
							}

							p3, p4 := upLeft.Add(left).Scaled(0.5), up.Add(p).Scaled(0.5)

							if withinWorld(p3) || withinWorld(p4) {
								enforceWorldBoundary(&p3)
								enforceWorldBoundary(&p4)
								imd.Push(p3, p4)
								imd.Line(1.0)
							}
						}
					}
				}
			}

			// draw particles
			for _, p := range game.data.particles {
				particleDraw.Clear()
				if p != (particle{}) {
					defaultSize := pixel.V(8, 2)
					pModel := defaultSize.ScaledXY(p.scale)
					particleDraw.Color = p.colour
					particleDraw.SetColorMask(pixel.RGBA{1.0, 1.0, 1.0, p.colour.A})
					particleDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, p.orientation).Moved(p.origin))
					particleDraw.Push(pixel.V(-pModel.X/2, 0.0), pixel.V(pModel.X/2, 0.0))
					particleDraw.Line(pModel.Y)
					particleDraw.Draw(imd)
				}
			}

			// draw player
			imd.Color = colornames.White
			imd.SetColorMask(pixel.Alpha(1))
			d := imdraw.New(nil)
			if debug {
				d.Color = colornames.Lightgreen
				d.Push(player.origin)
				d.Circle(player.radius, 2)
			}
			d.SetMatrix(pixel.IM.Rotated(pixel.ZV, player.target.Angle()-math.Pi/2).Moved(player.origin))
			playerDraw.Draw(d)
			d.Draw(imd)

			// imd.Push(player.rect.Min, player.rect.Max)
			// imd.Rectangle(2)

			// draw enemies
			imd.Color = colornames.Lightskyblue
			for _, e := range game.data.entities {
				if e.alive {
					if e.entityType == "wanderer" {
						imd.Color = colornames.Purple
						imd.Push(e.origin)
						imd.Circle(20, 2)
					} else if e.entityType == "blackhole" {
						imd.Color = colornames.Red
						imd.Push(e.origin)
						imd.Circle(e.radius, 2)
					} else {
						tmpTarget.Clear()
						tmpTarget.SetMatrix(pixel.IM.Rotated(e.origin, e.target.Angle()))
						weight := 2.0
						if e.entityType == "follower" {
							tmpTarget.Color = colornames.Lightskyblue
						} else if e.entityType == "pink" || e.entityType == "pinkpleb" {
							weight = 4.0
							tmpTarget.Color = colornames.Lightpink
						} else if e.entityType == "dodger" {
							weight = 4.0
							tmpTarget.Color = colornames.Limegreen
						}
						tmpTarget.Push(pixel.V(e.origin.X-e.radius, e.origin.Y-e.radius), pixel.V(e.origin.X+e.radius, e.origin.Y+e.radius))
						tmpTarget.Rectangle(weight)
						tmpTarget.Draw(imd)
					}

					if debug {
						imd.Color = colornames.Green
						imd.Push(e.origin)
						imd.Circle(e.radius, 2)
					}
				}
			}

			bulletDraw.Color = colornames.Lightgoldenrodyellow
			for _, b := range game.data.bullets {
				if b.data.alive {
					bulletDraw.Clear()
					bulletDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, b.data.target.Angle()-math.Pi/2).Moved(b.data.origin))
					drawBullet(&b.data, bulletDraw)
					bulletDraw.Draw(imd)
				}
			}
		}

		canvas.Clear(colornames.Black)
		mapRect.Draw(canvas)
		imd.Draw(canvas)

		if game.state == "playing" {
			for _, e := range game.data.entities {
				if (!e.alive && e.death != time.Time{}) {
					// fmt.Print("[DrawBounty]")
					// Draw the bounty
					e.text.Clear()
					e.text.Orig = e.origin
					e.text.Dot = e.origin

					text := fmt.Sprintf("%d", e.bounty*game.data.scoreMultiplier)
					e.text.Dot.X -= (e.text.BoundsOf(text).W() / 2)
					fmt.Fprintf(e.text, "%s", text)
					e.text.Color = colornames.White

					growth := (0.5 - (float64(e.expiry.Sub(last).Milliseconds()) / 300.0))
					e.text.Draw(
						canvas,
						pixel.IM.Scaled(e.text.Orig, 2.0+growth),
					)
				}
			}
		}

		// stretch the canvas to the window
		win.Clear(colornames.White)
		win.SetMatrix(pixel.IM.ScaledXY(pixel.ZV,
			pixel.V(
				win.Bounds().W()/canvas.Bounds().W(),
				win.Bounds().H()/canvas.Bounds().H(),
			),
		).Moved(win.Bounds().Center()))
		canvas.Draw(win, pixel.IM.Moved(canvas.Bounds().Center()))

		if game.state == "playing" {
			scoreTxt.Clear()
			txt := "Score: %d\n"
			scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(scoreTxt, txt, game.data.score)
			txt = "X%d"
			scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(scoreTxt, txt, game.data.scoreMultiplier)
			scoreTxt.Draw(
				win,
				pixel.IM.Scaled(scoreTxt.Orig, 2),
			)

			livesTxt.Clear()
			txt = "Lives: %d\n"
			livesTxt.Dot.X -= (livesTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(livesTxt, txt, game.data.lives)
			txt = "Bombs: %d"
			livesTxt.Dot.X -= (livesTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(livesTxt, txt, game.data.bombs)
			livesTxt.Draw(
				win,
				pixel.IM.Scaled(livesTxt.Orig, 2),
			)
		} else if game.state == "paused" {
			pausedTxt.Draw(
				win,
				pixel.IM.Scaled(
					pausedTxt.Orig,
					5,
				),
			)
		} else if game.state == "game_over" {
			gameOverTxt.Draw(
				win,
				pixel.IM.Scaled(
					gameOverTxt.Orig,
					4,
				),
			)
		}

		if win.Pressed(pixelgl.KeyEscape) {
			win.SetClosed(true)
		}

		win.Update()
	}
}

func main() {
	pixelgl.Run(run)
}
