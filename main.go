package main

import (
	"fmt"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"time"

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

// ENTITIES

type entityData struct {
	target     pixel.Vec
	born       time.Time
	rect       pixel.Rect
	alive      bool
	speed      float64
	entityType string
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
	return p
}

func NewWanderer(x float64, y float64) *entityData {
	w := new(entityData)
	size := 40.0
	w.rect = pixel.R(x-(size/2), y-(size/2), x+(size/2), y+(size/2))
	w.speed = 40
	w.alive = true
	w.entityType = "wanderer"
	w.born = time.Now()
	return w
}

func NewPinkSquare(x float64, y float64) *entityData {
	w := new(entityData)
	size := 40.0
	w.rect = pixel.R(x-(size/2), y-(size/2), x+(size/2), y+(size/2))
	w.speed = 140
	w.alive = true
	w.entityType = "pink"
	w.born = time.Now()
	return w
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
	fireRate     float64
	fireMode     string
	bulletsFired int
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
	weaponData.bulletsFired = 0

	return weaponData
}

func NewBurstWeapon() *weapondata {
	weaponData := new(weapondata)
	weaponData.fireRate = 0.2
	weaponData.fireMode = "burst"
	weaponData.bulletsFired = 0

	return weaponData
}

func NewConicWeapon() *weapondata {
	weaponData := new(weapondata)
	weaponData.fireRate = 0.15
	weaponData.fireMode = "conic"
	weaponData.bulletsFired = 0

	return weaponData
}

func NewGameData() *gamedata {
	gameData := new(gamedata)
	gameData.lives = 3
	gameData.bombs = 3
	gameData.scoreMultiplier = 1
	gameData.entities = make([]entityData, 100)
	gameData.bullets = make([]bullet, 100)
	gameData.player = *NewEntity(0.0, 0.0, 50, 280, "player")
	gameData.spawns = 0
	gameData.spawnCount = 1
	gameData.scoreSinceBorn = 0
	gameData.weapon = *NewWeaponData()

	gameData.score = 0
	gameData.multiplierReward = 500
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
	game.data.multiplierReward = 500
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
	weight := 2.0
	outline := 8.0
	p := pixel.ZV.Add(pixel.V(0.0, -20.0))
	pInner := p.Add(pixel.V(0, outline))
	l1 := p.Add(pixel.V(-10.0, -10.0))
	l1Inner := l1.Add(pixel.V(0, outline))
	r1 := p.Add(pixel.V(10.0, -10.0))
	r1Inner := r1.Add(pixel.V(0.0, outline))
	d.Push(p, l1)
	d.Line(weight)
	d.Push(pInner, l1Inner)
	d.Line(weight)
	d.Push(p, r1)
	d.Line(weight)
	d.Push(pInner, r1Inner)
	d.Line(weight)

	l2 := l1.Add(pixel.V(-15, 15))
	l2Inner := l2.Add(pixel.V(outline, 0.0))
	r2 := r1.Add(pixel.V(15, 15))
	r2Inner := r2.Add(pixel.V(-outline, 0.0))
	d.Push(l1, l2)
	d.Line(weight)
	d.Push(l1Inner, l2Inner)
	d.Line(weight)
	d.Push(r1, r2)
	d.Line(weight)
	d.Push(r1Inner, r2Inner)
	d.Line(weight)

	l3 := l2.Add(pixel.V(15, 30))
	r3 := r2.Add(pixel.V(-15, 30))
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
	// monitor := pixelgl.PrimaryMonitor()
	// width, height := monitor.Size()
	cfg := pixelgl.WindowConfig{
		Title: "Euclidean Combat",
		// Bounds: pixel.R(0, 0, width, height/),
		Bounds: pixel.R(0, 0, 1024, 768),
		// Monitor:   monitor,
		// Maximized: true,
		VSync: true,
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
	camPos := pixel.ZV

	// Game initialization
	last := time.Now()

	// Fonts
	atlas := text.NewAtlas(basicfont.Face7x13, text.ASCII)
	gameOverTxt := text.New(pixel.V(0, 0), atlas)
	pausedTxt := text.New(pixel.V(0, 0), atlas)
	scoreTxt := text.New(pixel.V(-(win.Bounds().W()/2)+120, (win.Bounds().H()/2)-50), atlas)
	livesTxt := text.New(pixel.V(0.0, (win.Bounds().H()/2)-50), atlas)

	// Input initialization
	currJoystick := pixelgl.Joystick1
	for i := pixelgl.Joystick1; i <= pixelgl.JoystickLast; i++ {
		if win.JoystickPresent(i) {
			currJoystick = i
			fmt.Printf("Joystick Connected: %d", i)
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
			if last.Sub(game.data.lastBullet).Seconds() > game.data.weapon.fireRate {
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
						spread := 0.20
						for i := 0; i < bulletCount; i++ {
							bPos := pixel.V(
								5.0,
								-width+(float64(i)*(width/float64(bulletCount))),
							).Rotated(
								aim.Angle(),
							).Add(player.rect.Center())

							b := NewBullet(
								bPos.X,
								bPos.Y,
								900,
								aim.Add(pixel.V((rand.Float64()*spread)-(spread/2), (rand.Float64()*spread)-(spread/2))).Unit(),
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
					game.data.lastBullet = time.Now()
					game.data.weapon.bulletsFired += 1
					shot := shotBuffer.Streamer(0, shotBuffer.Len())
					volume := &effects.Volume{
						Streamer: shot,
						Base:     10,
						Volume:   -0.7,
						Silent:   false,
					}
					speaker.Play(volume)
				} else {
					game.data.weapon.bulletsFired = 0
				}
			}

			// move enemies
			for i, e := range game.data.entities {
				dir := e.rect.Center().To(player.rect.Center())
				if e.entityType == "wanderer" {
					if e.target.Len() == 0 || e.rect.Center().To(e.target).Len() < 0.2 {
						e.target = pixel.V(
							rand.Float64()*400,
							rand.Float64()*400,
						).Add(e.rect.Center())
					}
					dir = e.rect.Center().To(e.target)
				}
				scaled := dir.Unit().Scaled(e.speed * dt)
				e.rect = e.rect.Moved(scaled)
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

			for bID, b := range game.data.bullets {
				if b.data.rect.W() > 0 && b.data.alive {
					for eID, e := range game.data.entities {
						if e.rect.W() > 0 {
							if b.data.rect.Intersects(e.rect) {
								b.data.alive = false
								e.alive = false
								game.data.score += 50 * game.data.scoreMultiplier
								game.data.scoreSinceBorn += 50 * game.data.scoreMultiplier
								game.data.bullets[bID] = b
								game.data.entities[eID] = e
								break
							}
							if !pixel.R(-worldWidth/2, -worldHeight/2, worldWidth/2, worldHeight/2).Contains(b.data.rect.Center()) {
								b.data.alive = false
								game.data.bullets[bID] = b
							}
						}
					}
				}
			}

			for _, e := range game.data.entities {
				if e.alive && e.rect.W() > 0 {
					if e.rect.Intersects(player.rect) {
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
					game.data.entities[eID] = e
				}
			}

			// kill entities
			newEntities := make([]entityData, 100)
			for _, e := range game.data.entities {
				if e.alive {
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
					if r < 0.5 {
						enemy = *NewEntity(
							pos.X,
							pos.Y,
							40,
							120,
							"follower",
						)
					} else if r < 0.8 {
						enemy = *NewWanderer(
							pos.X,
							pos.Y,
						)
					} else {
						enemy = *NewPinkSquare(
							pos.X,
							pos.Y,
						)
					}

					game.data.entities = append(game.data.entities, enemy)
					game.data.spawns += 1
				}

				spawnSound := spawnBuffer.Streamer(0, spawnBuffer.Len())
				speaker.Play(spawnSound)
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
				// sound := multiplierBuffer.Streamer(0, multiplierBuffer.Len())
				// volume := &effects.Volume{
				// 	Streamer: sound,
				// 	Base:     10,
				// 	Volume:   -0.9,
				// 	Silent:   false,
				// }
				// speaker.Play(volume)
				game.data.scoreMultiplier += 1
				game.data.multiplierReward *= 2
			}

			timeToUpgrade := game.data.score >= 10000 && game.data.lastWeaponUpgrade == time.Time{}
			if timeToUpgrade || (game.data.lastWeaponUpgrade != time.Time{} && last.Sub(game.data.lastWeaponUpgrade).Seconds() >= game.data.weaponUpgradeFreq) {
				fmt.Printf("[UpgradingWeapon]")
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
					} else if e.entityType == "follower" {
						imd.Color = colornames.Lightskyblue
						imd.Push(e.rect.Min, e.rect.Max)
						imd.Rectangle(2)
					} else if e.entityType == "pink" {
						imd.Color = colornames.Lightpink
						imd.Push(e.rect.Min, e.rect.Max)
						imd.Rectangle(4)
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
