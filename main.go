package main

import (
	"fmt"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
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

type entityData struct {
	orientation pixel.Vec
	born        time.Time
	rect        pixel.Rect
	alive       bool
	speed       float64
}

type bullet struct {
	data     entityData
	velocity pixel.Vec
}

func NewEntity(x float64, y float64, size float64, speed float64) *entityData {
	p := new(entityData)
	p.orientation = pixel.V(0.0, 1.0)
	p.rect = pixel.R(x-(size/2), y-(size/2), x+(size/2), y+(size/2))
	p.speed = speed
	p.alive = true
	p.born = time.Now()
	return p
}

func NewBullet(x float64, y float64, speed float64, target pixel.Vec) *bullet {
	b := new(bullet)
	b.data = *NewEntity(x, y, 3, speed)
	b.data.orientation = target
	b.velocity = target.Scaled(speed)
	return b
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
	p1 := pixel.ZV.Add(pixel.V(0.0, 5.0))
	p2 := p1.Add(pixel.V(5.0, -10.0))
	p3 := p1.Add(pixel.V(-5.0, -10.0))
	d.Push(p1, p2, pixel.ZV, p3)
	d.Polygon(1)
}

func (player *entityData) draw(d *imdraw.IMDraw) {
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

	// SOUNDS

	music, _ := os.Open("sound/music.mp3")
	shootSound, _ := os.Open("sound/shoot.mp3")
	spawnSound, _ := os.Open("sound/spawn.mp3")
	lifeSound, _ := os.Open("sound/life.mp3")
	multiplierSound, _ := os.Open("sound/multiplierbonus.mp3")
	bombSound, _ := os.Open("sound/usebomb.mp3")

	musicStreamer, musicFormat, err := mp3.Decode(music)
	if err != nil {
		panic(err)
	}
	defer musicStreamer.Close()

	shootStreamer, shootFormat, err := mp3.Decode(shootSound)
	if err != nil {
		panic(err)
	}
	shotBuffer := beep.NewBuffer(shootFormat)
	shotBuffer.Append(shootStreamer)
	shootStreamer.Close()

	spawnStreamer, spawnFormat, err := mp3.Decode(spawnSound)
	if err != nil {
		panic(err)
	}
	spawnBuffer := beep.NewBuffer(spawnFormat)
	spawnBuffer.Append(spawnStreamer)
	spawnStreamer.Close()

	lifeStreamer, lifeFormat, err := mp3.Decode(lifeSound)
	if err != nil {
		panic(err)
	}
	lifeBuffer := beep.NewBuffer(lifeFormat)
	lifeBuffer.Append(lifeStreamer)
	lifeStreamer.Close()

	multiplierStreamer, multiplierFormat, err := mp3.Decode(multiplierSound)
	if err != nil {
		panic(err)
	}
	multiplierBuffer := beep.NewBuffer(multiplierFormat)
	multiplierBuffer.Append(multiplierStreamer)
	multiplierStreamer.Close()

	bombStreamer, bombFormat, err := mp3.Decode(bombSound)
	if err != nil {
		panic(err)
	}
	bombBuffer := beep.NewBuffer(bombFormat)
	bombBuffer.Append(bombStreamer)
	bombStreamer.Close()

	speaker.Init(musicFormat.SampleRate, musicFormat.SampleRate.N(time.Second/10))
	speaker.Play(musicStreamer)

	// END SOUNDS

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
	last := time.Now()
	lastSpawn := time.Now()
	lastWaveSpawn := time.Now()
	lastBullet := time.Now()
	lastBomb := time.Now()

	rand.Seed(time.Now().Unix())
	atlas := text.NewAtlas(basicfont.Face7x13, text.ASCII)
	gameOverTxt := text.New(pixel.V(0, 0), atlas)
	pausedTxt := text.New(pixel.V(0, 0), atlas)
	scoreTxt := text.New(pixel.V(-(win.Bounds().W()/2)+120, (win.Bounds().H()/2)-50), atlas)
	livesTxt := text.New(pixel.V(0.0, (win.Bounds().H()/2)-50), atlas)

	var spawnFreq float64
	var waveDuration float64
	var waveFreq float64
	var waveStart time.Time
	var waveEnd time.Time
	var fireRate float64
	var fireMode string
	var entities []entityData
	var bullets []bullet
	var bulletsFired int
	var spawns int
	var spawnCount int
	var lives int
	var lifeReward int
	var bombReward int
	var bombs int
	var scoreMultiplier int
	var multiplierReward int
	var scoreSinceBorn int
	player := NewEntity(0.0, 0.0, 50, 280)
	score := 0

	currJoystick := pixelgl.Joystick1
	for i := pixelgl.Joystick1; i <= pixelgl.JoystickLast; i++ {
		if win.JoystickPresent(i) {
			currJoystick = i
			fmt.Printf("Joystick Connected: %d", i)
			break
		}
	}

	gameState := "starting"

	// precache player draw
	player.draw(playerDraw)
	for !win.Closed() {
		// update
		dt := time.Since(last).Seconds()
		last = time.Now()

		// lerp the camera position towards the player
		camPos = pixel.Lerp(
			camPos,
			player.rect.Center(),
			1-math.Pow(1.0/128, dt),
		)
		cam := pixel.IM.Moved(camPos.Scaled(-1))
		canvas.SetMatrix(cam)

		if gameState == "paused" {
			if win.Pressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
				gameState = "playing"
			}
		}

		if gameState == "starting" {
			lives = 3
			bombs = 3
			spawnFreq = 1.5
			waveDuration = 5
			waveFreq = 30
			waveStart = time.Time{}
			waveEnd = time.Now()
			fireRate = 0.25
			fireMode = "normal"
			spawns = 0
			entities = make([]entityData, 100)
			bullets = make([]bullet, 100)
			bulletsFired = 0
			player = NewEntity(0.0, 0.0, 50, 280)
			gameState = "playing"
			score = 0
			scoreSinceBorn = 0
			scoreMultiplier = 1
			multiplierReward = 500
			spawnCount = 1
			lifeReward = 100000
			bombReward = 100000
		}

		if gameState == "playing" {
			if !player.alive {
				entities = make([]entityData, 100)
				bullets = make([]bullet, 100)
				bulletsFired = 0
				waveStart = time.Time{}
				player = NewEntity(0.0, 0.0, 50, 280)
				scoreMultiplier = 1
				scoreSinceBorn = 0
				multiplierReward = 500
			}

			if win.Pressed(pixelgl.KeyP) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonStart) {
				gameState = "paused"
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
				orientationDt := (pixel.Lerp(
					player.orientation,
					direction,
					1-math.Pow(1.0/512, dt),
				))
				player.orientation = orientationDt
				player.rect = player.rect.Moved(direction.Scaled(player.speed * dt))
			}

			aim := thumbstickVector(win, currJoystick, pixelgl.AxisRightX, pixelgl.AxisRightY)
			if last.Sub(lastBullet).Seconds() > fireRate {
				if win.Pressed(pixelgl.KeySpace) {
					scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) * (canvas.Bounds().W() / win.Bounds().W())
					scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) * (canvas.Bounds().H() / win.Bounds().H())
					mp := pixel.V(scaledX, scaledY).Add(camPos)

					fmt.Printf(
						"[Player] X: %f, Y: %f	[Mouse] X: %f, Y: %f\n",
						player.rect.Center().X,
						player.rect.Center().Y,
						scaledX,
						scaledY,
					)

					aim = player.rect.Center().To(mp)
				}

				if aim.Len() > 0 {
					// fmt.Printf("Bullet spawned %s", time.Now().String())
					if fireMode == "conic" {
						rad := math.Atan2(aim.Unit().Y, aim.Unit().X)
						ang1 := rad + (10 * (2 * math.Pi) / 360)
						ang2 := rad - (10 * (2 * math.Pi) / 360)
						ang1Vec := pixel.V(math.Cos(ang1), math.Sin(ang1))
						ang2Vec := pixel.V(math.Cos(ang2), math.Sin(ang2))

						var bulletAngle pixel.Vec
						switch i := bulletsFired % 4; i {
						case 0:
							bulletAngle = aim.Unit()
						case 1:
							bulletAngle = ang1Vec
						case 2:
							bulletAngle = aim.Unit()
						case 3:
							bulletAngle = ang2Vec
						}
						b := NewBullet(
							player.rect.Center().X,
							player.rect.Center().Y,
							1200,
							bulletAngle,
						)
						bullets = append(bullets, *b)
					} else {
						b1Pos := pixel.V(5.0, 8.0).Rotated(aim.Angle()).Add(player.rect.Center())
						b2Pos := pixel.V(5.0, -8.0).Rotated(aim.Angle()).Add(player.rect.Center())
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
						bullets = append(bullets, *b1, *b2)
					}
					lastBullet = time.Now()
					bulletsFired += 1
					shot := shotBuffer.Streamer(0, shotBuffer.Len())
					volume := &effects.Volume{
						Streamer: shot,
						Base:     10,
						Volume:   -0.7,
						Silent:   false,
					}
					speaker.Play(volume)
				} else {
					bulletsFired = 0
				}
			}

			// move enemies
			for i, e := range entities {
				dir := e.rect.Center().To(player.rect.Center())
				scaled := dir.Unit().Scaled(e.speed * dt)
				e.rect = e.rect.Moved(scaled)
				entities[i] = e
			}

			for i, b := range bullets {
				b.data.rect = b.data.rect.Moved(b.velocity.Scaled(dt))
				bullets[i] = b
			}

			// check for collisions
			player.enforceWorldBoundary()

			for id, a := range entities {
				for id2, b := range entities {
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
						entities[id] = a
					}
				}
			}

			for bID, b := range bullets {
				if b.data.rect.W() > 0 && b.data.alive {
					for eID, e := range entities {
						if e.rect.W() > 0 {
							if b.data.rect.Intersects(e.rect) {
								b.data.alive = false
								e.alive = false
								score += 50 * scoreMultiplier
								scoreSinceBorn += 50 * scoreMultiplier
								bullets[bID] = b
								entities[eID] = e
								break
							}
							if !pixel.R(-worldWidth/2, -worldHeight/2, worldWidth/2, worldHeight/2).Contains(b.data.rect.Center()) {
								b.data.alive = false
								bullets[bID] = b
							}
						}
					}
				}
			}

			for _, e := range entities {
				if e.alive && e.rect.W() > 0 {
					if e.rect.Intersects(player.rect) {
						lives -= 1
						if lives == 0 {
							gameState = "game_over"

							gameOverTxt.Clear()
							lines := []string{
								"Game Over.",
								"Score: " + fmt.Sprintf("%d", score),
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
			if bombs > 0 && bombPressed && last.Sub(lastBomb).Seconds() > 3.0 {
				sound := bombBuffer.Streamer(0, bombBuffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   0.7,
					Silent:   false,
				}
				speaker.Play(volume)
				lastBomb = time.Now()

				bombs -= 1
				for eID, e := range entities {
					e.alive = false
					entities[eID] = e
				}
			}

			// kill entities
			newEntities := make([]entityData, 100)
			for _, e := range entities {
				if e.alive {
					newEntities = append(newEntities, e)
				}
			}
			entities = newEntities

			// kill bullets
			newBullets := make([]bullet, 100)
			for _, b := range bullets {
				if b.data.alive || b.data.born.After(time.Now().Add(time.Duration(-10)*time.Second)) {
					newBullets = append(newBullets, b)
				}
			}
			bullets = newBullets

			// spawn entities
			if last.Sub(lastSpawn).Seconds() > spawnFreq {
				// spawn
				for i := 0; i < spawnCount; i++ {
					pos := pixel.V(
						float64(rand.Intn(worldWidth)-worldWidth/2),
						float64(rand.Intn(worldHeight)-worldHeight/2),
					)
					for pos.Sub(player.rect.Center()).Len() < 400 {
						pos = pixel.V(
							float64(rand.Intn(worldWidth)-worldWidth/2),
							float64(rand.Intn(worldHeight)-worldHeight/2),
						)
					}

					enemy := NewEntity(
						pos.X,
						pos.Y,
						50,
						120,
					)
					entities = append(entities, *enemy)
					spawns += 1
				}

				spawnSound := spawnBuffer.Streamer(0, spawnBuffer.Len())
				speaker.Play(spawnSound)
				lastSpawn = time.Now()
				if spawns%20 == 0 {
					spawnCount += 1
				}

				if spawns%10 == 0 && spawnFreq > 0.5 {
					spawnFreq -= 0.1
				}
			}

			if (waveStart == time.Time{}) && last.Sub(waveEnd).Seconds() >= waveFreq { // or the
				// Start the next wave
				fmt.Printf("[WaveStart] %s\n", time.Now().String())
				waveStart = time.Now()
			}

			if (waveStart != time.Time{}) && last.Sub(waveStart).Seconds() >= waveDuration { // If a wave has ended
				// End the wave
				fmt.Printf("[WaveEnd] %s\n", time.Now().String())
				waveEnd = time.Now()
				waveStart = time.Time{}
			}

			if last.Sub(waveStart).Seconds() < waveDuration {
				// Continue wave
				// TODO make these data driven
				// waves would have spawn points, and spawn counts, and probably durations and stuff
				// hardcoded for now :D

				if last.Sub(lastWaveSpawn).Seconds() > 0.2 {

					// 4 spawn points
					points := [4]pixel.Vec{
						pixel.V(-(worldWidth/2)+50, -(worldHeight/2)+50),
						pixel.V(-(worldWidth/2)+50, (worldHeight/2)-50),
						pixel.V((worldWidth/2)-50, -(worldHeight/2)+50),
						pixel.V((worldWidth/2)-50, (worldHeight/2)-50),
					}

					for _, p := range points {
						enemy := NewEntity(
							p.X,
							p.Y,
							50,
							130,
						)
						entities = append(entities, *enemy)
						spawns += 1
					}
					spawnSound := spawnBuffer.Streamer(0, spawnBuffer.Len())
					speaker.Play(spawnSound)
					lastWaveSpawn = time.Now()
				}
			}

			// adjust game rules

			if score >= lifeReward {
				lifeReward += 100000
				lives += 1
				sound := lifeBuffer.Streamer(0, lifeBuffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   -0.9,
					Silent:   false,
				}
				speaker.Play(volume)
			}

			if score >= bombReward {
				bombReward += 100000
				bombs += 1
			}

			if scoreSinceBorn >= multiplierReward && scoreMultiplier < 10 {
				// sound := multiplierBuffer.Streamer(0, multiplierBuffer.Len())
				// volume := &effects.Volume{
				// 	Streamer: sound,
				// 	Base:     10,
				// 	Volume:   -0.9,
				// 	Silent:   false,
				// }
				// speaker.Play(volume)
				scoreMultiplier += 1
				multiplierReward *= 2
			}

			if score >= 10000 && fireRate > 0.1 {
				fireRate = 0.1
			}

			if score >= 20000 && fireMode != "conic" {
				fireMode = "conic"
			}

		} else {
			if win.Pressed(pixelgl.KeyEnter) || win.JoystickPressed(currJoystick, pixelgl.ButtonA) {
				gameState = "starting"
			}
		}

		// draw
		imd.Clear()

		if gameState == "playing" {
			// draw player
			imd.Color = colornames.White
			d := imdraw.New(nil)
			d.SetMatrix(pixel.IM.Rotated(pixel.ZV, player.orientation.Angle()-math.Pi/2).Moved(player.rect.Center()))
			playerDraw.Draw(d)
			d.Draw(imd)

			// imd.Push(player.rect.Min, player.rect.Max)
			// imd.Rectangle(2)

			// draw enemies
			imd.Color = colornames.Lightskyblue
			for _, e := range entities {
				if e.alive {
					imd.Push(e.rect.Min, e.rect.Max)
					imd.Rectangle(2)
				}
			}

			bulletDraw.Color = colornames.Lightgoldenrodyellow
			for _, b := range bullets {
				if b.data.alive {
					bulletDraw.Clear()
					bulletDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, b.data.orientation.Angle()-math.Pi/2).Moved(b.data.rect.Center()))
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

		if gameState == "playing" {
			scoreTxt.Clear()
			txt := "Score: %d\n"
			scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(scoreTxt, txt, score)
			txt = "X%d"
			scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(scoreTxt, txt, scoreMultiplier)
			scoreTxt.Draw(
				win,
				pixel.IM.Scaled(scoreTxt.Orig, 2),
			)

			livesTxt.Clear()
			txt = "Lives: %d\n"
			livesTxt.Dot.X -= (livesTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(livesTxt, txt, lives)
			txt = "Bombs: %d"
			livesTxt.Dot.X -= (livesTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(livesTxt, txt, bombs)
			livesTxt.Draw(
				win,
				pixel.IM.Scaled(livesTxt.Orig, 2),
			)
		} else if gameState == "paused" {
			pausedTxt.Draw(
				win,
				pixel.IM.Scaled(
					pausedTxt.Orig,
					5,
				),
			)
		} else if gameState == "game_over" {
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
