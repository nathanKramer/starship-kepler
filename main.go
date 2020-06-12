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

const worldWidth = 1200
const worldHeight = 800
const debug = false

var basicFont *text.Atlas

// ENTITIES

// For now just using a god entity struct, honestly this is probably fine
type entityData struct {
	target     pixel.Vec
	born       time.Time
	death      time.Time
	expiry     time.Time
	rect       pixel.Rect
	alive      bool
	speed      float64
	entityType string
	text       *text.Text

	// sounds
	spawnSound *beep.Buffer

	// enemy data
	bounty     int
	bountyText string
}

type bullet struct {
	data     entityData
	velocity pixel.Vec
}

func NewEntity(x float64, y float64, size float64, speed float64, entityType string) *entityData {
	p := new(entityData)
	p.target = pixel.V(0.0, 1.0)
	p.rect = pixel.R(x-(size/2), y-(size/2), x+(size/2), y+(size/2))
	p.speed = speed
	p.alive = true
	p.entityType = entityType
	p.born = time.Now()
	p.text = text.New(pixel.V(0, 0), basicFont)
	return p
}

func NewFollower(x float64, y float64) *entityData {
	e := NewEntity(x, y, 50.0, 120, "follower")
	e.target = pixel.V(1.0, 1.0)
	e.spawnSound = spawnBuffer
	e.bounty = 50
	return e
}

func NewWanderer(x float64, y float64) *entityData {
	w := NewEntity(x, y, 40.0, 40, "wanderer")
	w.spawnSound = spawnBuffer4
	w.bounty = 25
	return w
}

func NewDodger(x float64, y float64) *entityData {
	w := NewEntity(x, y, 50.0, 140, "dodger")
	w.spawnSound = spawnBuffer2
	w.bounty = 100
	return w
}

func NewPinkSquare(x float64, y float64) *entityData {
	w := NewEntity(x, y, 50.0, 140, "pink")
	w.spawnSound = spawnBuffer5
	w.bounty = 100
	return w
}

func NewPinkPleb(x float64, y float64) *entityData {
	w := NewEntity(x, y, 30.0, 180, "pinkpleb")
	// w.spawnSound = spawnBuffer4
	w.bounty = 75
	return w
}

func NewBlackHole(x float64, y float64) *entityData {
	b := NewEntity(x, y, 75.0, 0.0, "blackhole")
	b.bounty = 150
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

	lastSpawn         time.Time
	lastBullet        time.Time
	lastBomb          time.Time
	lastWeaponUpgrade time.Time
}

type game struct {
	state string
	data  gamedata
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
	gameData.player = *NewEntity(0.0, 0.0, 50, 320, "player")
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

	return game
}

func (data *gamedata) respawnPlayer() {
	data.entities = make([]entityData, 100)
	data.bullets = make([]bullet, 100)
	data.player = *NewEntity(0.0, 0.0, 50, 280, "player")
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

func (p *entityData) enforceWorldBoundary() { // this code seems dumb, TODO: find some api call that does it
	w := p.rect.W()
	h := p.rect.H()
	minX := -(worldWidth / 2.0)
	minY := -(worldHeight / 2.0)
	maxX := (worldWidth / 2.)
	maxY := (worldHeight / 2.0)
	if p.rect.Min.X < minX {
		p.rect.Min.X = minX
		p.rect.Max.X = minX + w
	} else if p.rect.Max.X > maxX {
		p.rect.Min.X = maxX - w
		p.rect.Max.X = maxX
	}
	if p.rect.Min.Y < minY {
		p.rect.Min.Y = minY
		p.rect.Max.Y = minY + h
	} else if p.rect.Max.Y > maxY {
		p.rect.Max.Y = maxY
		p.rect.Min.Y = maxY - h
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
			player.rect.Center(),
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
			}

			if win.Pressed(pixelgl.KeyP) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonStart) {
				game.state = "paused"
				pausedTxt.Clear()
				line := "Paused."

				pausedTxt.Dot.X -= (pausedTxt.BoundsOf(line).W() / 2)
				fmt.Fprintln(pausedTxt, line)
			}

			// player controls
			direction := pixel.V(0.0, 0.0)
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
				player.rect = player.rect.Moved(direction.Scaled(player.speed * dt))
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

					// fmt.Printf(
					// 	"[Player] X: %f, Y: %f	[Mouse] X: %f, Y: %f\n",
					// 	player.rect.Center().X,
					// 	player.rect.Center().Y,
					// 	scaledX,
					// 	scaledY,
					// )

					aim = player.rect.Center().To(mp)
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
							player.rect.Center().X,
							player.rect.Center().Y,
							1500, ang1Vec,
						)
						b := NewBullet(
							player.rect.Center().X,
							player.rect.Center().Y,
							1500,
							aim.Unit(),
						)
						rightB := NewBullet(
							player.rect.Center().X,
							player.rect.Center().Y,
							1500,
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
							).Add(player.rect.Center())

							b := NewBullet(
								bPos.X,
								bPos.Y,
								900,
								aim.Unit(),
							)
							game.data.bullets = append(game.data.bullets, *b)
						}
					} else {
						b1Pos := pixel.V(5.0, 5.0).Rotated(aim.Angle()).Add(player.rect.Center())
						b2Pos := pixel.V(5.0, -5.0).Rotated(aim.Angle()).Add(player.rect.Center())
						b1 := NewBullet(
							b1Pos.X,
							b1Pos.Y,
							1200,
							aim.Unit(),
						)
						b2 := NewBullet(
							b2Pos.X,
							b2Pos.Y,
							1200,
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

			// move enemies
			for i, e := range game.data.entities {
				if !e.alive {
					continue
				}
				dir := e.rect.Center().To(player.rect.Center()).Unit()
				if e.entityType == "wanderer" {
					if e.target.Len() == 0 || e.rect.Center().To(e.target).Len() < 0.2 {
						e.target = pixel.V(
							rand.Float64()*400,
							rand.Float64()*400,
						).Add(e.rect.Center())
					}
					dir = e.rect.Center().To(e.target).Unit()
				} else if e.entityType == "dodger" {
					// https://gamedev.stackexchange.com/questions/109513/how-to-find-if-an-object-is-facing-another-object-given-position-and-direction-a
					// todo, tidy up and put somewhere
					currentlyDodgingDist := -1.0
					for _, b := range game.data.bullets {
						if (b == bullet{}) || !b.data.alive {
							continue
						}
						entToBullet := e.rect.Center().Sub(b.data.rect.Center()).Unit()
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
							dir = dodgeVec.Scaled(3)
						}
					}
				}
				scaled := dir.Scaled(e.speed * dt)
				e.rect = e.rect.Moved(scaled)
				e.enforceWorldBoundary()
				game.data.entities[i] = e
			}

			for i, b := range game.data.bullets {
				b.data.rect = b.data.rect.Moved(b.velocity.Scaled(dt))
				game.data.bullets[i] = b
			}

			// check for collisions
			player.enforceWorldBoundary()

			for id, a := range game.data.entities {
				for id2, b := range game.data.entities {
					if id == id2 {
						continue
					}

					aCircle := pixel.C(a.rect.Center(), a.rect.W()/2)
					bCircle := pixel.C(b.rect.Center(), b.rect.W()/2)
					intersection := aCircle.Intersect(bCircle)
					if intersection.Radius > 0 {
						a.rect = a.rect.Moved(
							b.rect.Center().To(a.rect.Center()).Unit().Scaled(intersection.Radius / 2),
						)
						game.data.entities[id] = a
					}
				}
			}

			entsToAdd := make([]entityData, 0, 100)
			for bID, b := range game.data.bullets {
				if b.data.rect.W() > 0 && b.data.alive {
					for eID, e := range game.data.entities {
						if e.rect.W() > 0 && e.alive && b.data.rect.Intersects(e.rect) {
							b.data.alive = false
							e.alive = false
							e.death = last
							e.expiry = last.Add(time.Millisecond * 300)
							reward := e.bounty * game.data.scoreMultiplier
							e.bountyText = fmt.Sprintf("%d", reward)

							// on kill
							if e.entityType == "pink" {
								// spawn 3 mini plebs
								for i := 0; i < 3; i++ {
									pos := pixel.V(
										rand.Float64()*100,
										rand.Float64()*100,
									).Add(e.rect.Center())
									pleb := *NewPinkPleb(pos.X, pos.Y)
									entsToAdd = append(entsToAdd, pleb)
								}
								// spawnSound := entsToAdd[0].spawnSound.Streamer(0, entsToAdd[0].spawnSound.Len())
								// speaker.Play(spawnSound)
							}

							game.data.score += reward
							game.data.scoreSinceBorn += reward
							game.data.bullets[bID] = b
							game.data.entities[eID] = e
							break
						}
					}
					if !pixel.R(-worldWidth/2, -worldHeight/2, worldWidth/2, worldHeight/2).Contains(b.data.rect.Center()) {
						b.data.alive = false
						game.data.bullets[bID] = b
					}
				}
			}

			game.data.entities = append(game.data.entities, entsToAdd...)

			for _, e := range game.data.entities {
				if e.alive && e.rect.W() > 0 && e.rect.Intersects(player.rect) {
					game.data.lives -= 1
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
					} else {
						player.alive = false
					}
				}
			}

			// check for bomb here for now
			bombPressed := win.Pressed(pixelgl.KeyR) || win.JoystickAxis(currJoystick, pixelgl.AxisRightTrigger) > 0.1
			if game.data.bombs > 0 && bombPressed && last.Sub(game.data.lastBomb).Seconds() > 3.0 {
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
			if last.Sub(game.data.lastSpawn).Seconds() > game.data.spawnFreq {
				// spawn
				spawns := make([]entityData, 0, game.data.spawnCount)
				for i := 0; i < game.data.spawnCount; i++ {
					pos := pixel.V(
						float64(rand.Intn(worldWidth)-worldWidth/2),
						float64(rand.Intn(worldHeight)-worldHeight/2),
					)
					for pos.Sub(player.rect.Center()).Len() < 300 {
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
			// draw player
			imd.Color = colornames.White
			d := imdraw.New(nil)
			if debug {
				d.Color = colornames.Lightgreen
				d.Push(player.rect.Min, player.rect.Max)
				d.Rectangle(2)
			}
			d.SetMatrix(pixel.IM.Rotated(pixel.ZV, player.target.Angle()-math.Pi/2).Moved(player.rect.Center()))
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
						imd.Push(e.rect.Center())
						imd.Circle(20, 2)
					} else if e.entityType == "blackhole" {
						imd.Color = colornames.Red
						imd.Push(e.rect.Center())
						imd.Circle(37, 2)
					} else {
						tmpTarget.Clear()
						tmpTarget.SetMatrix(pixel.IM.Rotated(e.rect.Center(), e.target.Angle()))
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
						tmpTarget.Push(e.rect.Min, e.rect.Max)
						tmpTarget.Rectangle(weight)
						tmpTarget.Draw(imd)
					}
				}
			}

			bulletDraw.Color = colornames.Lightgoldenrodyellow
			for _, b := range game.data.bullets {
				if b.data.alive {
					bulletDraw.Clear()
					bulletDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, b.data.target.Angle()-math.Pi/2).Moved(b.data.rect.Center()))
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
					e.text.Orig = e.rect.Center()
					e.text.Dot = e.rect.Center()

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
