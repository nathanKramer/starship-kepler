package main

import (
	"fmt"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"
	"time"

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
	born  time.Time
	rect  pixel.Rect
	alive bool
	speed float64
}

type bullet struct {
	data     entityData
	velocity pixel.Vec
}

func NewEntity(x float64, y float64, size float64, speed float64) *entityData {
	p := new(entityData)
	p.rect = pixel.R(x-(size/2), y-(size/2), x+(size/2), y+(size/2))
	p.speed = speed
	p.alive = true
	p.born = time.Now()
	return p
}

func NewBullet(x float64, y float64, speed float64, target pixel.Vec) *bullet {
	b := new(bullet)
	b.data = *NewEntity(x, y, 3, speed)
	b.velocity = target.Scaled(speed)
	return b
}

func thumbstickVector(win *pixelgl.Window, joystick pixelgl.Joystick, axisX pixelgl.GamepadAxis, axisY pixelgl.GamepadAxis) pixel.Vec {
	v := pixel.V(0.0, 0.0)
	if win.JoystickPresent(joystick) {
		x := win.JoystickAxis(pixelgl.Joystick(pixelgl.Joystick1), axisX)
		y := win.JoystickAxis(pixelgl.Joystick(pixelgl.Joystick1), axisY) * -1

		if math.Abs(x) < 0.1 {
			x = 0
		}
		if math.Abs(y) < 0.1 {
			y = 0
		}

		v = pixel.V(x, y)
	}
	return v
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
	monitor := pixelgl.PrimaryMonitor()
	width, height := monitor.Size()
	cfg := pixelgl.WindowConfig{
		Title:     "Euclidean Combat",
		Bounds:    pixel.R(0, 0, width, height),
		Monitor:   monitor,
		Maximized: true,
		VSync:     true,
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
	camPos := pixel.ZV
	last := time.Now()
	lastSpawn := time.Now()
	lastBullet := time.Now()

	rand.Seed(time.Now().Unix())
	atlas := text.NewAtlas(basicfont.Face7x13, text.ASCII)
	gameOverTxt := text.New(pixel.V(0, 0), atlas)
	scoreTxt := text.New(pixel.V(-(win.Bounds().W()/2)+50, (win.Bounds().H()/2)-50), atlas)

	var spawnFreq float64
	var entities []entityData
	var bullets []bullet
	var spawns int
	var spawnCount int
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

		if gameState == "starting" {
			spawnFreq = 1.5
			spawns = 0
			entities = make([]entityData, 100)
			bullets = make([]bullet, 100)
			player = NewEntity(0.0, 0.0, 50, 280)
			gameState = "playing"
			score = 0
			spawnCount = 1
		}

		if gameState == "playing" {
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

			if direction.Len() > 0 {
				player.rect = player.rect.Moved(direction.Unit().Scaled(player.speed * dt))
			}

			aim := thumbstickVector(win, currJoystick, pixelgl.AxisRightX, pixelgl.AxisRightY)
			if last.Sub(lastBullet).Seconds() > 0.15 {
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
					b := NewBullet(
						player.rect.Center().X,
						player.rect.Center().Y,
						1200,
						aim.Unit(),
					)
					lastBullet = b.data.born
					bullets = append(bullets, *b)
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
				if b.data.rect.W() > 0 {
					for eID, e := range entities {
						if e.rect.W() > 0 {
							if b.data.rect.Intersects(e.rect) {
								b.data.alive = false
								e.alive = false
								score += 50
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
					}
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
				if spawns%50 == 0 {
					spawnCount = 10
				}
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
				lastSpawn = time.Now()
				spawnCount = 1

				if spawns%5 == 0 && spawnFreq > 0.2 {
					spawnFreq -= 0.1
				}
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
			imd.Push(player.rect.Min, player.rect.Max)
			imd.Rectangle(2)

			// draw enemies
			imd.Color = colornames.Lightskyblue
			for _, e := range entities {
				if e.alive {
					imd.Push(e.rect.Min, e.rect.Max)
					imd.Rectangle(2)
				}
			}

			imd.Color = colornames.Lightgoldenrodyellow
			for _, b := range bullets {
				if b.data.alive {
					imd.Push(b.data.rect.Center())
					imd.Circle(3, 1)
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
			fmt.Fprintf(scoreTxt, "Score: %d", score)
			scoreTxt.Draw(
				win,
				pixel.IM.Scaled(scoreTxt.Orig, 2),
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
