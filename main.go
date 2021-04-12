package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/nathanKramer/starship-kepler/sliceextra"

	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel/text"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// Config
type configuration struct {
	screenWidth  float64
	screenHeight float64
	fullscreen   bool
}

// Pull this stuff out into a config file.
var config = configuration{
	1024.0,
	768.0,
	false,
}

const maxParticles = 5000
const worldWidth = 1700.0
const worldHeight = 1080.0
const gameTitle = "Starship Kepler"

const uiClickWait = 0.125
const uiJoyThreshold = 0.7

const uiActionShoot = pixelgl.MouseButton1
const uiActionAct = pixelgl.MouseButton2
const uiActionActSelf = pixelgl.MouseButton3
const uiActionSwitchMode = pixelgl.KeyLeftControl
const uiActionBoost = pixelgl.KeyLeftShift
const uiActionBomb = pixelgl.KeySpace
const uiActionStop = pixelgl.KeyLeftAlt

const particlesOn = true

var elementWaterColor = color.RGBA{0x48, 0x64, 0xed, 0xff}
var elementLifeColor = colornames.Green
var elementSpiritColor = colornames.Snow
var elementWindColor = colornames.Orange
var elementLightningColor = color.RGBA{0xc0, 0x30, 0xc0, 0xff}
var elementChaosColor = colornames.Crimson
var elementEarthColor = colornames.Burlywood
var elementFireColor = colornames.Orangered

var elements = map[string]color.RGBA{
	"water":     elementWaterColor,
	"chaos":     elementChaosColor,
	"spirit":    elementSpiritColor,
	"fire":      elementFireColor,
	"lightning": elementLightningColor,
	"wind":      elementWindColor,
	"life":      elementLifeColor,
}

var titleFont *text.Atlas
var basicFont *text.Atlas

// Particles

type debugInfo struct {
	p1   pixel.Vec
	p2   pixel.Vec
	text string
}

// STATE

func uiUp(win *pixelgl.Window, gamePadDir pixel.Vec) bool {
	return win.JustPressed(pixelgl.KeyUp) || gamePadDir.Y > uiJoyThreshold
}

func uiDown(win *pixelgl.Window, gamePadDir pixel.Vec) bool {
	return (win.JustPressed(pixelgl.KeyDown) || gamePadDir.Y < -uiJoyThreshold)
}

func uiChangeSelection(win *pixelgl.Window, gamePadDir pixel.Vec, last time.Time, lastUiAction time.Time) int {
	uiChange := 0

	if last.Sub(lastUiAction).Seconds() > uiClickWait {
		if uiUp(win, gamePadDir) {
			uiChange = -1
		} else if uiDown(win, gamePadDir) {
			uiChange = 1
		}
	}

	return uiChange
}

func uiConfirm(win *pixelgl.Window, currJoystick pixelgl.Joystick) bool {
	return win.JustPressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA)
}

func uiCancel(win *pixelgl.Window, currJoystick pixelgl.Joystick) bool {
	return win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonB)
}

func uiPause(win *pixelgl.Window, currJoystick pixelgl.Joystick) bool {
	return win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonStart)
}

func uiThumbstickVector(win *pixelgl.Window, joystick pixelgl.Joystick, axisX pixelgl.GamepadAxis, axisY pixelgl.GamepadAxis) pixel.Vec {
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
	size := bullet.radius * 2
	d.Color = pixel.ToRGBA(color.RGBA{255, 192, 128, 255})

	d.Push(
		pixel.V(1.0, size),
		pixel.V(1.0, -size),
		pixel.V(-1.0, size),
		pixel.V(-1.0, -size),
	)
	d.Rectangle(3)

	for i, el := range bullet.elements {
		d.Color = elements[el]
		d.Push(
			pixel.V(1.0, (float64(i)*size)+float64(i+1)*size),
			pixel.V(1.0, float64(i)*size),
			pixel.V(-1.0, (float64(i)*size)+float64(i+1)*size),
			pixel.V(-1.0, float64(i)*size),
		)
		d.Rectangle(8)
	}
}

func drawShip(d *imdraw.IMDraw) {
	// weight := 3.0
	// outline := 8.0
	// p := pixel.ZV.Add(pixel.V(0.0, -15.0))
	// pInner := p.Add(pixel.V(0, outline))
	// l1 := p.Add(pixel.V(-10.0, -5.0))
	// l1Inner := l1.Add(pixel.V(0, outline))
	// r1 := p.Add(pixel.V(10.0, -5.0))
	// r1Inner := r1.Add(pixel.V(0.0, outline))
	// d.Push(p, l1)
	// d.Line(weight)
	// d.Push(pInner, l1Inner)
	// d.Line(weight)
	// d.Push(p, r1)
	// d.Line(weight)
	// d.Push(pInner, r1Inner)
	// d.Line(weight)

	// l2 := l1.Add(pixel.V(-15, 20))
	// l2Inner := l2.Add(pixel.V(outline, 0.0))
	// r2 := r1.Add(pixel.V(15, 20))
	// r2Inner := r2.Add(pixel.V(-outline, 0.0))
	// d.Push(l1, l2)
	// d.Line(weight)
	// d.Push(l1Inner, l2Inner)
	// d.Line(weight)
	// d.Push(r1, r2)
	// d.Line(weight)
	// d.Push(r1Inner, r2Inner)
	// d.Line(weight)

	// l3 := l2.Add(pixel.V(15, 25))
	// r3 := r2.Add(pixel.V(-15, 25))
	// d.Push(l2, l3)
	// d.Line(weight)
	// d.Push(r2, r3)
	// d.Line(weight)
	// d.Push(l2Inner, l3)
	// d.Line(weight)
	// d.Push(r2Inner, r3)
	// d.Line(weight)
}

func loadFileToString(filename string) (string, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func loadPicture(path string) (pixel.Picture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return pixel.PictureDataFromImage(img), nil
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MB", bToMb(m.Alloc))
	fmt.Printf("\tSys = %v MB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func init() {
	rand.Seed(time.Now().Unix())
}

// resist the urge to refactor. just write a game, don't worry about clean code.
func run() {
	monitor := pixelgl.PrimaryMonitor()
	width := config.screenWidth
	height := config.screenHeight

	if width > 1920 {
		width = 1920
	}
	if height > 1080 {
		height = 1080
	}
	cfg := pixelgl.WindowConfig{
		Title:  "Starship Kepler",
		Bounds: pixel.R(0, 0, width, height),
		VSync:  true,
	}

	if config.fullscreen {
		cfg.Monitor = monitor
	}

	debug := false
	if debug {
		cfg.Bounds = pixel.R(0, 0, 1024, 768)
		cfg.Maximized = false
		cfg.Monitor = nil
	}

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	wardInnerPic, _ := loadPicture("./images/wards/ward_alpha.png")
	wardOuterPic, _ := loadPicture("./images/wards/ward2_alpha.png")
	wardInner := pixel.NewSprite(wardInnerPic, wardInnerPic.Bounds())
	wardOuter := pixel.NewSprite(wardOuterPic, wardOuterPic.Bounds())

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
	uiCanvas := pixelgl.NewCanvas(pixel.R(-cfg.Bounds.W()/2, -cfg.Bounds.H()/2, cfg.Bounds.W()/2, cfg.Bounds.H()/2))

	bloom1 := pixelgl.NewCanvas(pixel.R(-cfg.Bounds.W()/2, -cfg.Bounds.H()/2, cfg.Bounds.W()/2, cfg.Bounds.H()/2))
	extractBrightness, err := loadFileToString("./shaders/extract_bright_areas.glsl")
	if err != nil {
		panic(err)
	}
	bloom1.SetFragmentShader(extractBrightness)

	bloom2 := pixelgl.NewCanvas(pixel.R(-cfg.Bounds.W()/2, -cfg.Bounds.H()/2, cfg.Bounds.W()/2, cfg.Bounds.H()/2))
	blur, err := loadFileToString("./shaders/blur.glsl")
	if err != nil {
		panic(err)
	}
	bloom2.SetFragmentShader(blur)

	bloom3 := pixelgl.NewCanvas(pixel.R(-cfg.Bounds.W()/2, -cfg.Bounds.H()/2, cfg.Bounds.W()/2, cfg.Bounds.H()/2))
	// blur, err = loadFileToString("./shaders/blur.glsl")
	// if err != nil {
	// 	panic(err)
	// }
	bloom3.SetFragmentShader(blur)

	innerWardBatch := imdraw.New(wardInnerPic)
	outerWardBatch := imdraw.New(wardOuterPic)

	// Game initialization
	last := time.Now()

	// Fonts and text
	ttfData, err := ioutil.ReadFile("./font/gabriel_serif/Gabriel Serif.ttf")
	if err != nil {
		log.Fatal(err)
	}
	var titleFace font.Face = basicfont.Face7x13
	tFont, err := truetype.Parse(ttfData)
	if err != nil {
		log.Fatal(err)
	} else {
		titleFace = truetype.NewFace(tFont, &truetype.Options{
			Size: 24.0,
			DPI:  96,
		})
	}
	// Fonts and text
	ttfData, err = ioutil.ReadFile("./font/comfortaa/Comfortaa-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}
	var normalFace font.Face = basicfont.Face7x13
	nFont, err := truetype.Parse(ttfData)
	if err != nil {
		log.Fatal(err)
	} else {
		normalFace = truetype.NewFace(nFont, &truetype.Options{
			Size: 18.0,
			DPI:  96,
		})
	}

	titleFont = text.NewAtlas(titleFace, text.ASCII)
	basicFont = text.NewAtlas(normalFace, text.ASCII)

	titleTxt := text.New(pixel.V(0, 128), titleFont)
	gameOverTxt := text.New(pixel.V(0, 64), basicFont)
	centeredTxt := text.New(pixel.V(0, 0), basicFont)
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
	camPos := pixel.ZV
	imd := imdraw.New(nil)
	uiDraw := imdraw.New(nil)
	bulletDraw := imdraw.New(nil)
	particleDraw := imdraw.New(nil)
	tmpTarget := imdraw.New(nil)

	lastMenuChoice := time.Now()
	lastMemCheck := time.Now()

	// precache player draw
	// drawShip(playerDraw)
	// playIntroMusic()
	debugInfos := []debugInfo{}
	totalTime := 0.0

	playMenuMusic()
	game.menu = NewMainMenu()
	game.data = *NewMenuGame()

	PrintMemUsage()

	for !win.Closed() {
		if game.state == "quitting" {
			runtime.GC()
			PrintMemUsage()
			win.SetClosed(true)
		}
		if game.state == "reset" {
			game.state = "main_menu"
			game.data = *NewGameData()
			game.menu = NewMainMenu()
		}

		// update
		dt := math.Min(time.Since(last).Seconds(), 0.1) * game.data.timescale
		totalTime += dt
		last = time.Now()

		imd.Reset()
		uiDraw.Reset()
		bulletDraw.Reset()
		particleDraw.Reset()
		tmpTarget.Reset()

		if last.Sub(lastMemCheck).Seconds() > 5.0 {
			PrintMemUsage()
			fmt.Printf("Entities\tlen: %d\tcap: %d\n", len(game.data.entities), cap(game.data.entities))
			fmt.Printf("New Entities\tlen: %d\tcap: %d\n\n", len(game.data.newEntities), cap(game.data.newEntities))

			fmt.Printf("Bullets\tlen: %d\tcap: %d\n", len(game.data.bullets), cap(game.data.bullets))
			fmt.Printf("New Bullets\tlen: %d\tcap: %d\n\n", len(game.data.newBullets), cap(game.data.newBullets))

			fmt.Printf("Particles\tlen: %d\tcap: %d\n", len(game.data.particles), cap(game.data.particles))
			fmt.Printf("New Particles\tlen: %d\tcap: %d\n\n", len(game.data.newParticles), cap(game.data.newParticles))

			lastMemCheck = last

			// runtime.GC()
		}

		if debug {
			debugInfos = []debugInfo{}
		}

		player := &game.data.player

		uiGamePadDir := pixel.Vec{}
		if win.JoystickPresent(currJoystick) {
			moveVec := uiThumbstickVector(
				win,
				currJoystick,
				pixelgl.AxisLeftX,
				pixelgl.AxisLeftY,
			)
			uiGamePadDir = moveVec
		}

		playerConfirmed := uiConfirm(win, currJoystick)
		playerCancelled := uiCancel(win, currJoystick)

		// lerp the camera position towards the player
		camPos = pixel.Lerp(
			camPos,
			player.origin.Scaled(0.75),
			1-math.Pow(1.0/128, dt),
		)

		cam := pixel.IM.Moved(camPos.Scaled(-1))
		canvas.SetMatrix(cam)

		if game.state == "main_menu" || game.state == "paused" {
			if playerConfirmed {
				if game.menu.options[game.menu.selection] == "Story Mode" {
					game.state = "starting"
					game.data = *NewStoryGame()
				}
				if game.menu.options[game.menu.selection] == "Quick Play: Evolved" {
					game.state = "starting"
					game.data = *NewEvolvedGame()
				}
				if game.menu.options[game.menu.selection] == "Quick Play: Pacifism" {
					game.state = "starting"
					game.data = *NewPacifismGame()
				}
				if game.menu.options[game.menu.selection] == "Options" {
					game.menu = NewOptionsMenu()
				}
				if game.menu.options[game.menu.selection] == "Main Menu" {
					game.state = "reset"
				}
				if game.menu.options[game.menu.selection] == "Quit" {
					game.state = "quitting"
				}
				if game.menu.options[game.menu.selection] == "Resume" {
					game.state = "playing"
				}
				if game.menu.options[game.menu.selection] == "Main Menu" {
					game.state = "main_menu"
					playMenuMusic()
					game.menu = NewMainMenu()
					game.data = *NewMenuGame()
				}
			}

			implemented := 0
			for _, option := range game.menu.options {
				if sliceextra.Contains(implementedMenuItems, option) {
					implemented += 1
				}
			}

			menuChange := uiChangeSelection(win, uiGamePadDir, last, lastMenuChoice)
			if menuChange != 0 {
				game.menu.selection = (game.menu.selection + menuChange) % len(game.menu.options)
				for (implemented > 0) && !sliceextra.Contains(implementedMenuItems, game.menu.options[game.menu.selection]) {
					game.menu.selection = (game.menu.selection + menuChange) % len(game.menu.options)
					if game.menu.selection < 0 {
						// would have thought modulo would handle negatives. /shrug
						game.menu.selection += len(game.menu.options)
					}
				}
				lastMenuChoice = time.Now()
			}
		}

		if game.state == "start_screen" {
			if playerConfirmed || playerCancelled {
				game.state = "main_menu"
				playMenuMusic()
				game.menu = NewMainMenu()
				game.data = *NewMenuGame()
			}
		}

		if game.state == "paused" {
			if playerCancelled {
				game.state = "playing"
			}
		}

		if game.state == "starting" {
			if game.data.mode == "evolved" {
				game.data = *NewEvolvedGame()
				playMusic()
			} else if game.data.mode == "pacifism" {
				game.data = *NewPacifismGame()
				playPacifismMusic()
			} else {
				game.data = *NewStoryGame()
				playMenuMusic()
			}

			game.state = "playing"
		}

		if (game.state == "playing" && debug) && playerConfirmed {
			game.state = "starting"
		}
		if game.state == "game_over" {
			if playerConfirmed {
				game.state = "starting"
			} else if playerCancelled {
				game.state = "main_menu"
				playMenuMusic()
				game.menu = NewMainMenu()
				game.data = *NewMenuGame()
			}
		}

		scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) *
			(canvas.Bounds().W() / win.Bounds().W())
		scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) *
			(canvas.Bounds().H() / win.Bounds().H())
		mp := pixel.V(scaledX, scaledY).Add(camPos)

		direction := pixel.ZV
		if game.state == "playing" {
			if !player.alive {
				game.respawnPlayer()
				game.grid.ApplyDirectedForce(Vector3{0.0, 0.0, 1400.0}, Vector3{player.origin.X, player.origin.Y, 0.0}, 80)
			}

			if uiPause(win, currJoystick) {
				game.state = "paused"
				game.menu = NewPauseMenu()
			}

			if win.JustPressed(pixelgl.KeyGraveAccent) {
				game.data.console = !game.data.console
			}

			// player controls
			if game.data.console {
				if win.JustPressed(pixelgl.KeyEnter) {
					game.data.console = false
				}
			} else {
				if win.JustPressed(pixelgl.KeyMinus) {
					game.data.timescale *= 0.5
					if game.data.timescale < 0.1 {
						game.data.timescale = 0.0
					}
				}
				if win.JustPressed(pixelgl.KeyEqual) {
					game.data.timescale *= 2.0
					if game.data.timescale > 4.0 || game.data.timescale == 0.0 {
						game.data.timescale = 1.0
					}
				}
				if win.JustPressed(pixelgl.Key1) {
					game.data.weapon = *NewWeaponData()
				}
				if win.JustPressed(pixelgl.Key2) {
					game.data.weapon = *NewBurstWeapon()
				}
				if win.JustPressed(pixelgl.Key3) {
					game.data.weapon = *NewConicWeapon()
				}

				// player.velocity = pixel.ZV
				// if win.Pressed(uiActionAct) {
				// mouse based target movement
				// 	player.target = mp
				// }

				// if win.Pressed(uiActionSwitchMode) {
				// 	// GW
				// 	player.mode = "GW"
				// 	player.elements = make([]string, 0)
				// } else {
				// 	player.mode = "MWW"
				// }

				if win.Pressed(pixelgl.KeyLeft) || win.Pressed(pixelgl.KeyA) {
					direction = direction.Add(pixel.V(-1, 0))
					player.target = pixel.Vec{}
				}
				if win.Pressed(pixelgl.KeyUp) || win.Pressed(pixelgl.KeyW) {
					direction = direction.Add(pixel.V(0, 1))
					player.target = pixel.Vec{}
				}
				if win.Pressed(pixelgl.KeyRight) || win.Pressed(pixelgl.KeyD) {
					direction = direction.Add(pixel.V(1, 0))
					player.target = pixel.Vec{}
				}
				if win.Pressed(pixelgl.KeyDown) || win.Pressed(pixelgl.KeyS) {
					direction = direction.Add(pixel.V(0, -1))
					player.target = pixel.Vec{}
				}

				if win.JustPressed(pixelgl.KeyQ) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonX) {
					player.QueueElement("water")
				}
				if win.JustPressed(pixelgl.KeyE) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonRightBumper) {
					player.QueueElement("chaos")
				}
				if win.JustPressed(pixelgl.KeyR) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonLeftBumper) {
					player.QueueElement("spirit")
				}
				if win.JustPressed(pixelgl.KeyF) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonB) {
					player.QueueElement("fire")
				}
				if win.JustPressed(pixelgl.KeyZ) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
					player.QueueElement("lightning")
				}
				if win.JustPressed(pixelgl.KeyX) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonY) {
					player.QueueElement("wind")
				}
				if win.JustPressed(pixelgl.KeyC) {
					player.QueueElement("life")
				}

				if win.Pressed(uiActionStop) || player.origin.To(player.target).Len() < 50.0 {
					player.target = pixel.Vec{}
				}

				if (player.target != pixel.Vec{}) {
					direction = direction.Add(player.origin.To(player.target).Unit())
					midColor := HSVToColor(3.0, 0.7, 1.0)
					game.data.newParticles = InlineAppendParticles(
						game.data.newParticles,
						NewParticle(player.target.X, player.target.Y, midColor, 32.0, pixel.V(0.5, 1.0), 0.0, randomVector(1), 1.0, "ship"),
					)
				}

				if win.JoystickPresent(currJoystick) {
					moveVec := uiThumbstickVector(
						win,
						currJoystick,
						pixelgl.AxisLeftX,
						pixelgl.AxisLeftY,
					)
					direction = direction.Add(moveVec)
				}
			}

			// paste debug.go
		}

		// main game update
		if game.state == "playing" || game.state == "game_over" || game.data.mode == "menu_game" {
			if game.data.mode == "menu_game" {
				if player.target.Len() == 0 || player.origin.To(player.target).Len() < 5.0 {
					poi := pixel.V(
						(rand.Float64()*worldWidth)-worldWidth/2.0,
						(rand.Float64()*worldHeight)-worldHeight/2.0,
					)
					player.target = player.origin.Sub(poi).Unit().Scaled(rand.Float64()*400 + 200)
					enforceWorldBoundary(&player.target, player.radius*2)
				}
				player.orientation = player.orientation.Rotated(60 * math.Pi / 180 * dt).Unit()
				direction = player.origin.To(player.target).Unit()
			}

			if direction.Len() > 0.2 {
				orientationDt := (pixel.Lerp(
					player.orientation,
					direction,
					1-math.Pow(1.0/512, dt),
				))
				player.orientation = orientationDt
				player.Propel(direction, dt)
				// player.velocity = direction.Unit().Scaled(player.speed)

				// partile stream
				baseVelocity := orientationDt.Unit().Scaled(-1 * player.speed).Scaled(dt)
				perpVel := pixel.V(baseVelocity.Y, -baseVelocity.X).Scaled(0.2 * math.Sin(totalTime*10))
				hue := math.Mod(((math.Mod(totalTime, 16.0) / 16.0) * 6.0), 6.0)
				hue2 := math.Mod(hue+0.6, 6.0)
				midColor := HSVToColor(hue, 0.7, 1.0)
				sideColor := HSVToColor(hue2, 0.5, 1.0)
				white := HSVToColor(hue2, 0.1, 1.0)
				pos := player.origin.Add(baseVelocity.Unit().Scaled(player.radius * 0.8))

				// boosting := win.Pressed(pixelgl.KeyLeftShift) || win.JoystickAxis(currJoystick, pixelgl.AxisLeftTrigger) > 0.1
				// if boosting {
				// 	SetBoosting(player)
				// } else {
				// 	SetDefaultPlayerSpeed(player)
				// }

				vel1 := baseVelocity.Add(perpVel).Add(randomVector((0.2)))
				vel2 := baseVelocity.Sub(perpVel).Add(randomVector((0.2)))
				game.data.newParticles = InlineAppendParticles(
					game.data.newParticles,
					NewParticle(pos.X, pos.Y, midColor, 32.0, pixel.V(0.5, 1.0), 0.0, baseVelocity, 1.0, "ship"),
					NewParticle(pos.X, pos.Y, sideColor, 24.0, pixel.V(1.0, 1.0), 0.0, vel1.Scaled(1.5), 1.0, "ship"),
					NewParticle(pos.X, pos.Y, sideColor, 24.0, pixel.V(1.0, 1.0), 0.0, vel2.Scaled(1.5), 1.0, "ship"),
					// NewParticle(pos.X, pos.Y, white, 24.0, pixel.V(0.5, 1.0), 0.0, vel1, 1.0, "ship"),
					// NewParticle(pos.X, pos.Y, white, 24.0, pixel.V(0.5, 1.0), 0.0, vel2, 1.0, "ship"),
				)

				if player.speed > 600 {
					game.data.newParticles = InlineAppendParticles(
						game.data.newParticles,
						NewParticle(pos.X, pos.Y, white, 32.0, pixel.V(1.0, 1.0), 0.0, vel1.Add(perpVel), 2.0, "ship"),
						NewParticle(pos.X, pos.Y, white, 32.0, pixel.V(1.0, 1.0), 0.0, vel2.Sub(perpVel), 2.0, "ship"),
					)
				}
			}
			player.Update(dt, totalTime, last)

			aim := player.origin.To(mp)
			gamepadAim := uiThumbstickVector(win, currJoystick, pixelgl.AxisRightX, pixelgl.AxisRightY)

			shooting := false
			if gamepadAim.Len() > 0.3 {
				shooting = true
				aim = gamepadAim
			}

			// if win.JustPressed(uiActionActSelf) || win.JustPressed(pixelgl.KeySpace) {
			// 	player.ReifyWard()
			// }

			timeSinceBullet := last.Sub(game.data.lastBullet).Seconds()
			timeSinceAbleToShoot := timeSinceBullet - (game.data.weapon.fireRate / game.data.timescale)

			if game.data.weapon != (weapondata{}) && timeSinceAbleToShoot >= 0 {
				if game.data.mode == "menu_game" {
					closest := 100000.0
					closestEntity := pixel.Vec{}
					for _, e := range game.data.entities {
						if e.alive && !e.spawning && player.origin.To(e.origin).Len() < closest {
							closestEntity = player.origin.To(e.origin)
							closest = closestEntity.Len()
						}
					}
					if closestEntity != (pixel.Vec{}) && closestEntity.Len() < 400 {
						aim = closestEntity
						shooting = true
					}
				} else if win.Pressed(uiActionShoot) || win.Pressed(pixelgl.KeyLeftSuper) {
					shooting = true
				}

				if shooting {
					// fmt.Printf("Bullet spawned %s", time.Now().String())
					rad := math.Atan2(aim.Unit().Y, aim.Unit().X)

					if game.data.weapon.conicAngle > 0 {
						ang1 := rad + (game.data.weapon.conicAngle * math.Pi / 180)
						ang2 := rad - (game.data.weapon.conicAngle * math.Pi / 180)
						ang1Vec := pixel.V(math.Cos(ang1), math.Sin(ang1))
						ang2Vec := pixel.V(math.Cos(ang2), math.Sin(ang2))

						// I really shouldn't use these weird side effect functions lol
						FireBullet(ang1Vec, game, player.origin, player)
						FireBullet(ang2Vec, game, player.origin, player)
						m := player.origin.Add(aim.Unit().Scaled(10))
						FireBullet(aim, game, m, player)
					} else if game.data.weapon.randomCone > 0 {
						for i := 0; i < game.data.weapon.bulletCount; i++ {
							off := (rand.Float64() * game.data.weapon.randomCone) - game.data.weapon.randomCone/2
							ang := rad + (off * math.Pi / 180)
							d := game.data.weapon.duration + (-(game.data.weapon.duration / 4.0) + (rand.Float64() * (game.data.weapon.duration / 2.0)))
							game.data.newBullets = InlineAppendBullets(
								game.data.newBullets,
								*NewBullet(
									player.origin.X,
									player.origin.Y,
									game.data.weapon.bulletSize,
									game.data.weapon.velocity,
									pixel.V(math.Cos(ang), math.Sin(ang)),
									append([]string{}, player.elements...),
									d,
								),
							)
						}
					} else {
						FireBullet(aim, game, player.origin, player)
					}

					// Reflective bullets procedure.
					// Temporarily disabled.
					// finalBullets := make([]bullet, 0)
					// for i := 0; i < len(game.data.newBullets); i++ {
					// 	b := game.data.newBullets[i]
					// 	finalBullets = append(finalBullets, b)
					// 	if game.data.weapon.reflective > 0 {
					// 		ang := 360 / float64(game.data.weapon.reflective+1)

					// 		for j := 1; j <= game.data.weapon.reflective; j++ {
					// 			reflectiveAngle := b.data.orientation.Angle() + (float64(j) * ang * math.Pi / 180)
					// 			reflectiveAngleVec := pixel.V(math.Cos(reflectiveAngle), math.Sin(reflectiveAngle))
					// 			finalBullets = append(
					// 				finalBullets,
					// 				*NewBullet(
					// 					b.data.origin.X,
					// 					b.data.origin.Y,
					// 					game.data.weapon.bulletSize,
					// 					game.data.weapon.velocity,
					// 					reflectiveAngleVec,
					// 					append([]string{}, player.elements...),
					// 					game.data.weapon.duration,
					// 				),
					// 			)
					// 		}
					// 	}
					// }

					bulletID := 0
					for addedID, newBullet := range game.data.newBullets {
						if newBullet.velocity == (pixel.Vec{}) {
							continue
						}

						if len(game.data.bullets) < cap(game.data.bullets) {
							game.data.bullets = append(game.data.bullets, newBullet)
							game.data.newBullets[addedID] = bullet{}
						} else {
							for bulletID < len(game.data.bullets) {
								existing := game.data.bullets[bulletID]
								if (existing.velocity == pixel.Vec{}) {
									game.data.bullets[bulletID] = newBullet
									game.data.newBullets[addedID] = bullet{}
									break
								}
								bulletID++
							}
						}
					}

					overflow := timeSinceAbleToShoot * 1000
					if overflow > game.data.weapon.fireRate {
						overflow = 0
					}
					game.data.lastBullet = last

					shot := shotBuffer3.Streamer(0, shotBuffer3.Len())
					volume := &effects.Volume{
						Streamer: shot,
						Base:     10,
						Volume:   -1.3,
						Silent:   false,
					}

					if game.data.weapon.bulletCount > 2 && game.data.weapon.conicAngle > 0 {
						shot = shotBuffer4.Streamer(0, shotBuffer4.Len())
						volume = &effects.Volume{
							Streamer: shot,
							Base:     10,
							Volume:   -0.9,
							Silent:   false,
						}
						speaker.Play(volume)
					} else if game.data.weapon.conicAngle > 0 {
						shot = shotBuffer2.Streamer(0, shotBuffer2.Len())
						volume = &effects.Volume{
							Streamer: shot,
							Base:     10,
							Volume:   -0.7,
							Silent:   false,
						}
						speaker.Play(volume)
					} else if game.data.weapon.bulletCount > 3 {
						shot = shotBuffer.Streamer(0, shotBuffer.Len())
						volume = &effects.Volume{
							Streamer: shot,
							Base:     10,
							Volume:   -0.9,
							Silent:   false,
						}
						speaker.Play(volume)
					}
					// speaker.Play(volume)
				}
			}
			player.relativeTarget = aim.Unit()

			// set velocities
			closestEnemyDist := 1000000.0
			for i, e := range game.data.entities {
				if !e.alive {
					continue
				}

				if e.spawning {
					if last.Sub(e.born).Seconds() >= e.spawnTime {
						e.spawning = false
						game.data.entities[i] = e
					}
					continue
				}

				dir := pixel.ZV
				toPlayer := e.origin.To(player.origin)
				if player.alive {
					dir = toPlayer.Unit()
				}
				if (e.entityType == "blackhole" || e.entityType == "bubble") && toPlayer.Len() < closestEnemyDist {
					closestEnemyDist = toPlayer.Len()
				}
				if e.entityType == "wanderer" {
					if e.target.Len() == 0 || e.origin.To(e.target).Len() < 5.0 {
						poi := pixel.V(
							(rand.Float64()*worldWidth)-worldWidth/2.0,
							(rand.Float64()*worldHeight)-worldHeight/2.0,
						)
						e.target = e.origin.Sub(poi).Unit().Scaled(rand.Float64() * 400)
					}
					e.orientation = e.orientation.Rotated(60 * math.Pi / 180 * dt).Unit()
					dir = e.origin.To(e.target).Unit()
				} else if e.entityType == "dodger" {
					// https://gamedev.stackexchange.com/questions/109513/how-to-find-if-an-object-is-facing-another-object-given-position-and-direction-a
					// todo, tidy up and put somewhere
					e.orientation = player.origin
					currentlyDodgingDist := -1.0
					for _, b := range game.data.bullets {
						if (len(b.data.elements) > 0 && b.data.elements[0] == "wind") || (len(b.data.elements) > 1 && b.data.elements[1] == "wind") {
							continue
						}
						if !b.data.alive {
							continue
						}
						entToBullet := e.origin.Sub(b.data.origin)
						if entToBullet.Len() > 200 {
							continue
						}
						entToBullet = entToBullet.Unit()
						facing := entToBullet.Dot(b.data.orientation.Unit())

						isClosest := (currentlyDodgingDist == -1.0 || entToBullet.Len() < currentlyDodgingDist)
						if facing > 0.0 && facing > 0.7 && facing < 0.95 && isClosest { // if it's basically dead on, they'll die.
							currentlyDodgingDist = entToBullet.Len()

							if debug {
								debugInfos = append(debugInfos, debugInfo{p1: e.origin, p2: b.data.origin})
							}

							baseVelocity := entToBullet.Unit().Scaled(-4 * game.data.timescale)

							midColor := pixel.ToRGBA(e.color)
							pos1 := pixel.V(1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							pos2 := pixel.V(-1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							pos3 := pixel.V(1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							pos4 := pixel.V(-1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							game.data.newParticles = InlineAppendParticles(
								game.data.newParticles,
								NewParticle(pos1.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "ship"),
								NewParticle(pos2.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "ship"),
								NewParticle(pos3.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "ship"),
								NewParticle(pos4.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "ship"),
							)

							dir = e.origin.Sub(b.data.origin).Scaled(4)
						}
					}
				} else if e.entityType == "pink" {
					e.orientation = player.origin
				} else if e.entityType == "snek" {
					t := math.Mod(last.Sub(e.born).Seconds(), 2.0)
					deg := (math.Sin(t*math.Pi) * e.cone) * math.Pi / 180.0
					dir = dir.Rotated(deg)
					if t > 1.9 {
						e.cone = 60.0 + (rand.Float64() * 90.0)
					}
				} else if e.entityType == "gate" {
					if e.target.Len() == 0 || e.origin.To(e.target).Len() < 5.0 {
						poi := pixel.V(
							(rand.Float64()*worldWidth)-worldWidth/2.0,
							(rand.Float64()*worldHeight)-worldHeight/2.0,
						)
						e.target = e.origin.Sub(poi).Unit().Scaled(rand.Float64() * 400)
					}
					e.orientation = e.orientation.Rotated(7 * math.Pi / 180 * dt).Unit()
					dir = e.origin.To(e.target).Unit()
				}
				e.Propel(dir, dt)

				game.data.entities[i] = e
			}
			game.data.timescale = math.Max(0.1, math.Min(256, closestEnemyDist)/256.0)

			for i, b := range game.data.bullets {
				if !b.data.alive {
					game.data.bullets[i] = bullet{}
					continue
				}
				b.data.origin = b.data.origin.Add(b.velocity.Scaled(dt))
				if game.data.weapon.randomCone == 0 {
					if game.data.weapon.bulletCount > 2 {
						if math.Mod(totalTime, 0.4) < 0.8 {
							game.grid.ApplyExplosiveForce(b.velocity.Scaled(dt).Len()*1.25, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 60.0)
						}
					} else {
						game.grid.ApplyDirectedForce(Vector3{b.velocity.X * dt * 0.05, b.velocity.Y * dt * 0.05, 0.0}, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 40.0)
					}
				}

				game.data.bullets[i] = b
			}

			// Process blackholes
			// I don't really care about efficiency atm
			for eID, e := range game.data.entities {
				e.pullVec = pixel.ZV // reset the pull vec (which is used for drawing effects related to the pull)
				game.data.entities[eID] = e
			}

			for bID, b := range game.data.entities {
				if !b.alive || b.entityType != "blackhole" || !b.active {
					continue
				}

				// emit particles
				if (uint64(totalTime*1000)/125)%2 == 0 {
					v := 6.0 + (rand.Float64() * 12)
					sprayVelocity := pixel.V(
						math.Cos(b.particleEmissionAngle),
						math.Sin(b.particleEmissionAngle),
					).Unit().Scaled(v * game.data.timescale)

					color := colornames.Lightskyblue
					pos := b.origin
					game.data.newParticles = InlineAppendParticles(
						game.data.newParticles,
						NewParticle(pos.X,
							pos.Y,
							pixel.ToRGBA(color),
							128.0,
							pixel.V(1.5, 1.5),
							0.0,
							sprayVelocity,
							2.0,
							"blackhole",
						),
					)

					b.particleEmissionAngle -= math.Pi / 25.0
				}

				if b.hp > 15 {
					game.grid.ApplyExplosiveForce(b.radius*5, Vector3{b.origin.X, b.origin.Y, 0.0}, b.radius*5)
					// game.grid.ApplyDirectedForce(Vector3{b.origin.X, b.origin.Y, 20.0}, Vector3{b.origin.X, b.origin.Y, 0.0}, 20)
					b.alive = false
					// spawn bubbles
					for i := 0; i < 5; i++ {
						pos := pixel.V(
							rand.Float64()*200,
							rand.Float64()*200,
						).Add(b.origin)
						pleb := *NewAngryBubble(pos.X, pos.Y)
						game.data.newEntities = InlineAppendEntities(game.data.newEntities, pleb)
					}
					game.data.entities[bID] = b

					for i := 0; i < 1024; i++ {
						extra := (1.0 / ((rand.Float64() * 10.0) + 1.0))
						speed := 32 * (1.0 - extra)

						p := NewParticle(
							b.origin.X,
							b.origin.Y,
							pixel.ToRGBA(colornames.Deepskyblue).Add(pixel.Alpha(extra*3)),
							64,
							pixel.V(1.0, 1.0),
							0.0,
							randomVector(speed),
							3.0,
							"enemy",
						)

						game.data.newParticles = InlineAppendParticles(game.data.newParticles, p)
					}

					continue
				}

				maxForce := 2400.0

				game.grid.ApplyImplosiveForce(5+b.radius, Vector3{b.origin.X, b.origin.Y, 0.0}, 50+b.radius)

				dist := player.origin.Sub(b.origin)
				length := dist.Len()
				if length <= 300.0 {
					force := pixel.Lerp(
						dist.Unit().Scaled(maxForce),
						pixel.ZV,
						length/300.0,
					)
					player.velocity = player.velocity.Sub(force.Scaled(dt))

					if debug {
						debugInfos = append(debugInfos, debugInfo{
							p1: player.origin,
							p2: player.origin.Sub(force.Scaled(dt * 10)),
						})
					}
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
						p.velocity = p.velocity.Add(pixel.V(n.Y, -n.X).Scaled(45 / (length + 250.0)))
					}
					game.data.particles[pID] = p
				}

				for bulletID, bul := range game.data.bullets {
					if bul.data.alive {
						dist := bul.data.origin.Sub(b.origin)
						length := dist.Len()
						if length > 300.0 {
							continue
						}
						n := dist.Unit()
						// bul.velocity = bul.velocity.Add(dist.Scaled(0.2))
						push1 := bul.velocity.Add(pixel.V(n.Y, -n.X).Scaled(length * 0.1))
						push2 := bul.velocity.Add(pixel.V(-n.Y, n.X).Scaled(length * 0.1))

						pushed := push1
						if bul.data.origin.Add(push2).Sub(b.origin).Len() > bul.data.origin.Add(push1).Sub(b.origin).Len() {
							pushed = push2
						}
						bul.velocity = pushed // either way rather than 1 way!
						game.data.bullets[bulletID] = bul
					}
				}

				for eID, e := range game.data.entities {
					if e.alive && !e.spawning && eID != bID {
						dist := b.origin.Sub(e.origin)
						length := dist.Len()
						if length > 300.0 {
							continue
						}

						n := dist.Unit()

						force := pixel.Lerp(
							n.Scaled(maxForce*2.4), // maximum force at close distance
							pixel.ZV,               // scale down to zero at maximum distance
							length/300.0,           // at max distance, 1.0, = pixel.ZV
						)
						// force = force.Add(pixel.V(n.Y, -n.X).Scaled(force.Len() * 0.5)) // add a bit of orbital force

						e.velocity = e.velocity.Add(force.Scaled(dt))
						e.pullVec = e.pullVec.Add(force.Scaled(dt))

						if debug {
							debugInfos = append(debugInfos, debugInfo{
								p1: e.origin,
								p2: e.origin.Add(force.Scaled(dt * 10)),
							})
						}

						intersection := pixel.C(b.origin, b.radius+8.0).Intersect(e.Circle())
						if intersection.Radius > 5.0 && e.entityType != "blackhole" {
							b.hp += e.hp // Blackhole grows stronger
							b.bounty += e.bounty
							e.alive = false
						}
						game.data.entities[eID] = e
					}
				}

				game.data.entities[bID] = b
			}

			for _, e := range game.data.entities {
				baseVelocity := e.pullVec.Scaled(0.05)

				if e.pullVec.Len() > 0 && e.color != nil {
					midColor := pixel.ToRGBA(e.color)
					pos1 := pixel.V(1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					pos2 := pixel.V(-1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					pos3 := pixel.V(1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					pos4 := pixel.V(-1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					game.data.newParticles = InlineAppendParticles(
						game.data.newParticles,
						NewParticle(pos1.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "enemy"),
						NewParticle(pos2.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "enemy"),
						NewParticle(pos3.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "enemy"),
						NewParticle(pos4.X, pos1.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity.Scaled(rand.Float64()*2.0), 1.0, "enemy"),
					)
				}
			}

			game.grid.Update()

			// Apply velocities
			// player.origin = player.origin.Add(player.velocity.Scaled(dt))

			for i, e := range game.data.entities {
				if !e.alive && !e.spawning {
					continue
				}
				if debug && win.JustPressed(pixelgl.MouseButton1) {
					e.selected = e.Circle().Intersect(pixel.C(mp, 4)).Radius > 0
					if e.entityType == "snek" {
						for tID, snekT := range e.tail {
							snekT.selected = snekT.Circle().Intersect(pixel.C(mp, 4)).Radius > 0
							e.tail[tID] = snekT
						}
					}
				}

				// if e.entityType == "gate" {
				// 	if math.Mod(totalTime, 0.05) < 0.02 {
				// 		game.grid.ApplyExplosiveForce(5, Vector3{e.origin.X, e.origin.Y, 0.0}, 200)
				// 	}
				// }

				e.Update(dt, totalTime, last)
				game.data.entities[i] = e
			}

			// check for collisions
			if game.data.mode != "story" {
				player.enforceWorldBoundary(false)
			}

			for id, a := range game.data.entities {
				//tmpTarget.Push(e.origin.Add(e.orientation.Scaled(e.radius)), e.origin.Add(e.orientation.Scaled(-e.radius)))
				ignoreList := []string{"pinkpleb", "snek", "gate"}
				if sliceextra.Contains(ignoreList, a.entityType) {
					continue
				}
				for id2, b := range game.data.entities {
					if id == id2 || sliceextra.Contains(ignoreList, a.entityType) || a.entityType != b.entityType {
						continue
					}

					intersection := a.MovementCollisionCircle().Intersect(b.MovementCollisionCircle())
					if intersection.Radius > 0 {
						a.origin = a.origin.Add(
							b.origin.To(a.origin).Unit().Scaled(intersection.Radius * dt),
						)
						game.data.entities[id] = a
					}
				}
			}

			for bID, b := range game.data.bullets {
				if b.data.alive {
					for eID, e := range game.data.entities {
						if e.alive && !e.spawning && b.data.Circle().Intersect(e.Circle()).Radius > 0 {
							b.DealDamage(
								&e,
								bID,
								1,
								last,
								game,
								player,
							)

							if e.entityType == "blackhole" {
								hitSound := blackholeHitBuffer.Streamer(0, blackholeHitBuffer.Len())
								volume := &effects.Volume{
									Streamer: hitSound,
									Base:     10,
									Volume:   -0.9,
									Silent:   false,
								}
								speaker.Play(volume)
							}
							e.DealDamage(
								&b.data,
								eID,
								1,
								last,
								game,
								player,
							)

							game.data.entities[eID] = e
							break
						} else if e.alive && !e.spawning && e.entityType == "snek" {
							for _, t := range e.tail {
								if t.entityType == "snektail" && b.data.Circle().Intersect(t.Circle()).Radius > 0 {
									b.data.alive = false
									game.data.bullets[bID] = b
									break
								}
							}
						}
					}
					if !pixel.R(-worldWidth/2, -worldHeight/2, worldWidth/2, worldHeight/2).Contains(b.data.origin) {

						// explode bullets when they hit the edge
						for i := 0; i < 30; i++ {
							p := NewParticle(
								b.data.origin.X,
								b.data.origin.Y,
								pixel.ToRGBA(colornames.Lightblue),
								32,
								pixel.V(1.0, 1.0),
								0.0,
								randomVector(5.0),
								1.0,
								"bullet",
							)
							game.data.newParticles = InlineAppendParticles(game.data.newParticles, p)
						}

						b.data.alive = false
						game.data.bullets[bID] = b
					}
				}
			}

			for eID, e := range game.data.entities {
				intersectionTest := e.Circle().Intersect(player.Circle()).Radius > 0

				if e.entityType == "gate" {
					l := pixel.L(e.origin.Add(e.orientation.Scaled(e.radius)), e.origin.Add(e.orientation.Scaled(-e.radius)))
					intersectionTest = l.IntersectCircle(player.Circle()).Len() > 0
				}

				if player.alive && e.alive && !e.spawning && intersectionTest {
					e.IntersectWithPlayer(
						e,
						eID,
						game,
						player,
						last,
						gameOverTxt,
					)
				}
			}

			if len(player.elements) > 0 {
				SetDefaultPlayerSpeed(player)
				game.data.weapon = *NewWeaponData()

				// inclusion checks
				for _, el := range player.elements {
					if el == "water" || el == "spirit" {
						game.data.weapon.bulletCount = 1
					} else if el == "fire" {
						game.data.weapon.bulletCount = 4
						game.data.weapon.duration = 0.25
						game.data.weapon.fireRate = 0.05
						game.data.weapon.randomCone = 12
					}
				}

				// cumulative effects
				for i := 0; i < len(player.elements); i++ {
					// todo: give each element a damage type.
					// this would give extra HP to the bullet when it hits shapes of its damage type, allowing them to penetrate more

					el := player.elements[i]
					if el == "wind" {
						game.data.weapon.bulletCount = game.data.weapon.bulletCount + 3
						game.data.weapon.duration = game.data.weapon.duration + 0.2
						// allows bullets to not be dodged by dodgers
						// be more data driven than this
						// SetBoosting(player) -> todo make this a spell

						// game.data.weapon.bulletCount = game.data.weapon.bulletCount + 1
					} else if el == "water" {
						game.data.weapon.conicAngle = game.data.weapon.conicAngle + 3
						game.data.weapon.velocity = game.data.weapon.velocity + 100

						// takes precedence over fire
						game.data.weapon.fireRate = math.Max(0.13, game.data.weapon.fireRate-0.03)
						game.data.weapon.duration = 5.0
						if game.data.weapon.randomCone > 0 {
							game.data.weapon.randomCone = 0
							game.data.weapon.bulletCount = 2
						}
					} else if el == "fire" {
						game.data.weapon.velocity = game.data.weapon.velocity + 100
						game.data.weapon.duration = game.data.weapon.duration + 0.125

						if game.data.weapon.conicAngle == 0 {
							game.data.weapon.randomCone = game.data.weapon.randomCone + 8
							game.data.weapon.bulletCount = game.data.weapon.bulletCount + 3
						}
					} else if el == "spirit" {
						game.data.weapon.velocity = game.data.weapon.velocity + 600
						game.data.weapon.bulletSize = game.data.weapon.bulletSize + 6
						game.data.weapon.conicAngle = game.data.weapon.conicAngle + 1

						// takes precedence over fire
						game.data.weapon.duration = 5.0
						game.data.weapon.fireRate = math.Max(0.144, game.data.weapon.fireRate-0.03)
						if game.data.weapon.randomCone > 0 {
							game.data.weapon.randomCone = 0
							game.data.weapon.bulletCount = 2
						}
					} else if el == "lightning" {
						game.data.weapon.fireRate = game.data.weapon.fireRate - 0.03
						game.data.weapon.duration = game.data.weapon.duration + 0.125
					} else if el == "chaos" {
						game.data.weapon.reflective = game.data.weapon.reflective + 1
						game.data.weapon.conicAngle = game.data.weapon.conicAngle + 2
						game.data.weapon.fireRate = game.data.weapon.fireRate - 0.02
						game.data.weapon.duration = game.data.weapon.duration + 0.125
					}
				}
				if game.data.weapon.randomCone > 0 {
					game.data.weapon.randomCone = game.data.weapon.randomCone + game.data.weapon.conicAngle
					game.data.weapon.conicAngle = 0
				}

				if win.JustPressed(uiActionAct) {
					// possible ways this could work:
					// ward elements could have passive effects, OR not.
					// I'm toying around with passive effects, and activation effects (which destroy the ward, thus removing the passive effects)
					// two kinds of activations: internal (probably defensive) and external (probably offensive)
					//end
					game.data.weapon = *NewWeaponData()
					player.elements = make([]string, 0)
				}
			}

			// check for bomb here for now
			bombPressed := win.Pressed(pixelgl.KeySpace) || win.JoystickAxis(currJoystick, pixelgl.AxisRightTrigger) > 0.1
			// game.data.bombs > 0 &&
			// droppping bomb concept for the moment
			if len(player.elements) > 0 && bombPressed && last.Sub(game.data.lastBomb).Seconds() > 3.0 {
				game.grid.ApplyExplosiveForce(256.0, Vector3{player.origin.X, player.origin.Y, 0.0}, 256.0)
				sound := bombBuffer.Streamer(0, bombBuffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   0.7,
					Silent:   false,
				}
				speaker.Play(volume)

				for i := 0; i < 1000; i++ {
					speed := 48.0 * (1.0 - 1/((rand.Float64()*32.0)+1))
					col := int(rand.Float32() * float32(len(player.elements)))
					p := NewParticle(
						player.origin.X,
						player.origin.Y,
						pixel.ToRGBA(elements[player.elements[col]]),
						100,
						pixel.V(1.5, 1.5),
						0.0,
						randomVector(speed),
						2.0,
						"player",
					)
					game.data.newParticles = InlineAppendParticles(game.data.newParticles, p)
				}

				game.data.lastBomb = time.Now()

				game.data.bombs--
				for eID, e := range game.data.entities {
					e.alive = false
					e.death = last
					e.expiry = last
					game.data.entities[eID] = e
				}

				player.elements = make([]string, 0)
			}

			// Keep buffered particles ticking so they don't stack up too much
			for pID, p := range game.data.newParticles {
				p.percentLife -= 1.0 / p.duration
				game.data.newParticles[pID] = p
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
			killedParticles := 0
			for particleID, existing := range game.data.newParticles {
				if (existing != particle{}) && existing.percentLife <= 0 {
					game.data.newParticles[particleID] = particle{}
				}
			}
			for particleID, existing := range game.data.particles {
				if (existing != particle{}) && existing.percentLife <= 0 {
					game.data.particles[particleID] = particle{}
					killedParticles++
				}
			}
			killedEnt := 0
			for entID, existing := range game.data.entities {
				if (!existing.alive && existing.born != time.Time{}) || (existing.expiry != time.Time{}) && last.After(existing.expiry) {
					game.data.entities[entID] = entityData{}
					killedEnt++
				} // kill entities
			}
			// fmt.Printf("Killed\t(%d entities)\t(%d particles)\n", killedEnt, killedParticles)

			particleID := 0
			for addedID, newParticle := range game.data.newParticles {
				if newParticle != (particle{}) {
					if len(game.data.particles) < cap(game.data.particles) {
						game.data.particles = append(game.data.particles, newParticle)
						game.data.newParticles[addedID] = particle{}
					} else {
						for particleID < len(game.data.particles) {
							existing := game.data.particles[particleID]
							if existing == (particle{}) {
								game.data.particles[particleID] = newParticle
								game.data.newParticles[addedID] = particle{}
								break
							}
							particleID++
						}
					}
				}
			}

			entID := 0
			toSpawn := 0
			spawnedEnt := 0
			for addedID, newEnt := range game.data.newEntities {
				if newEnt.entityType != "" {
					toSpawn++
					if len(game.data.entities) < cap(game.data.entities) {
						game.data.entities = append(game.data.entities, newEnt)
						game.data.newEntities[addedID] = entityData{}
						spawnedEnt++
						entID++
					} else {
						for entID < len(game.data.entities) {
							existing := game.data.entities[entID]
							if existing.origin == (pixel.Vec{}) {
								game.data.entities[entID] = newEnt
								game.data.newEntities[addedID] = entityData{}
								spawnedEnt++
								break
							}
							entID++
						}
					}
				}
			} // bring in new entities
			// fmt.Printf("Killed: %d	To Spawn: %d	Spawned: %d\n", killedEnt, toSpawn, spawnedEnt)

			// kill bullets
			for bID, b := range game.data.bullets {
				if !b.data.alive && time.Now().After(b.data.born.Add(time.Duration(b.duration*1000)*time.Millisecond)) {
					game.data.bullets[bID] = bullet{}
				}
			}

			if game.data.mode == "evolved" || game.data.mode == "menu_game" {
				game.evolvedGameModeUpdate(debug, last, totalTime, player)
			} else if game.data.mode == "pacifism" {
				game.pacifismGameModeUpdate(debug, last, totalTime, player)
			}
		}

		// draw_
		{
			imd.Clear()
			uiDraw.Clear()
			uiDraw.Color = colornames.Black

			if game.state == "paused" || game.data.mode == "menu_game" || game.state == "game_over" {
				a := (math.Min(totalTime, 4) / 8.0)
				canvas.SetColorMask(pixel.Alpha(a))
				uiCanvas.SetColorMask(pixel.Alpha(math.Min(1.0, a*4)))
			} else {
				canvas.SetColorMask(pixel.Alpha(1.0))
			}

			if game.data.console {
				// draw: console
				w := win.Bounds().W()
				h := win.Bounds().H()
				uiDraw.Push(
					pixel.V(-w/2.0, h/2.0),
					pixel.V(-w/2.0, (h/2.0)-32),
					pixel.V(w/2.0, h/2.0),
					pixel.V(w/2.0, (h/2.0)-32),
				)
				uiDraw.Rectangle(0.0)
			}

			// Draw: grid effect
			// TODO, extract?
			// Add catmullrom splines?
			if game.data.mode == "evolved" || game.data.mode == "pacifism" || game.data.mode == "menu_game" {
				width := len(game.grid.points)
				height := len(game.grid.points[0])
				imd.SetColorMask(pixel.Alpha(0.1))
				hue := math.Mod((3.6 + ((math.Mod(totalTime, 300.0) / 300.0) * 6.0)), 6.0)
				imd.Color = HSVToColor(hue, 0.5, 1.0)

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
								enforceWorldBoundary(&p, 0.0)
								enforceWorldBoundary(&left, 0.0)
								thickness := 1.0
								if y%2 == 0 {
									thickness = 4.0
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
								enforceWorldBoundary(&p, 0.0)
								enforceWorldBoundary(&up, 0.0)
								thickness := 1.0
								if x%2 == 0 {
									thickness = 4.0
								}
								imd.Push(up, p)
								imd.Line(thickness)
							}
						}

						if x > 0 && y > 0 {
							upLeft := game.grid.points[x-1][y-1].origin.ToVec2(cfg.Bounds)
							p1, p2 := upLeft.Add(up).Scaled(0.5), left.Add(p).Scaled(0.5)

							if withinWorld(p1) || withinWorld(p2) {
								enforceWorldBoundary(&p1, 0.0)
								enforceWorldBoundary(&p2, 0.0)
								imd.Push(p1, p2)
								imd.Line(1.0)
							}

							p3, p4 := upLeft.Add(left).Scaled(0.5), up.Add(p).Scaled(0.5)

							if withinWorld(p3) || withinWorld(p4) {
								enforceWorldBoundary(&p3, 0.0)
								enforceWorldBoundary(&p4, 0.0)
								imd.Push(p3, p4)
								imd.Line(1.0)
							}
						}
					}
				}
			}

			if game.data.mode == "evolved" || game.data.mode == "pacifism" || game.data.mode == "menu_game" {
				// draw: particles
				imd.SetColorMask(pixel.Alpha(0.4))
				for _, p := range game.data.particles {
					particleDraw.Clear()
					if p != (particle{}) {
						defaultSize := pixel.V(8, 2)
						pModel := defaultSize.ScaledXY(p.scale)
						particleDraw.Color = p.colour
						particleDraw.SetColorMask(pixel.Alpha(p.colour.A))
						particleDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, p.orientation).Moved(p.origin))
						particleDraw.Push(pixel.V(-pModel.X/2, 0.0), pixel.V(pModel.X/2, 0.0))
						particleDraw.Line(pModel.Y)
						particleDraw.Draw(imd)
					}
				}

				imd.Color = colornames.White
				imd.SetColorMask(pixel.Alpha(1))

				// draw: player
				if player.alive {
					d := imdraw.New(nil)
					d.SetMatrix(pixel.IM.Rotated(pixel.ZV, player.orientation.Angle()).Moved(player.origin))
					d.Push(pixel.ZV)

					size := 20.0
					rad := 4.0
					d.Circle(size, rad)
					d.Push(pixel.ZV)
					d.CircleArc(28.0, 0.3, -0.3, 2.0)

					// d.Push(pixel.ZV)
					// d.CircleArc(28.0, 0.2, -0.2, 2.0)
					d.Color = colornames.Lightsteelblue

					if (game.data.weapon != weapondata{}) {
						d.SetMatrix(pixel.IM.Moved(player.origin))
						d.Push(pixel.V(12.0, 0.0).Rotated(player.relativeTarget.Angle()))
						d.Circle(4.0, 2.0)
					}
					// playerDraw.Draw(d)
					d.Draw(imd)

					// draw: elements
					// e := imdraw.New(nil)
					// e.SetMatrix(pixel.IM.Moved(player.origin.Add(pixel.V(-32, -40))))
					// for i := 0; i < len(player.elements); i++ {
					// 	element := player.elements[i]
					// 	e.Color = elements[element]

					// 	e.Push(pixel.V(float64(i)*32, 0))
					// 	e.Circle(12, 4)
					// }
					// e.Draw(imd)
					// c := pixelgl.NewCanvas(pixel.R(-200, -200, 200, 200))
					// wardInner.Draw(c, pixel.IM)
					// wardOuter.Draw(c, pixel.IM)
					// c.DrawColorMask(imd, pixel.IM, elementLifeColor)
				}

				// lastBomb := game.data.lastBomb.Sub(last).Seconds()
				// if game.data.lastBomb.Sub(last).Seconds() < 1.0 {
				// 	// draw: bomb
				// 	imd.Color = colornames.White
				// 	imd.Push(player.origin)
				// 	imd.Circle(lastBomb*2048.0, 64)
				// }

				// imd.Push(player.rect.Min, player.rect.Max)
				// imd.Rectangle(2)

				// draw: enemies
				imd.Color = colornames.Lightskyblue
				for _, e := range game.data.entities {
					if e.alive {
						imd.SetColorMask(pixel.Alpha(1))
						size := e.radius
						if e.spawning {
							imd.SetColorMask(pixel.Alpha(0.7))
							timeSinceBorn := last.Sub(e.born).Seconds()
							spawnIndicatorT := e.spawnTime / 2.0

							size = e.radius * (math.Mod(timeSinceBorn, spawnIndicatorT) / spawnIndicatorT)
							if e.entityType == "blackhole" {
								size = e.radius * ((timeSinceBorn) / e.spawnTime) // grow from small to actual size
							}
						}

						if e.entityType == "wanderer" {
							tmpTarget.Clear()
							tmpTarget.Color = e.color
							baseTransform := pixel.IM.Rotated(pixel.ZV, e.orientation.Angle()).Moved(e.origin)
							tmpTarget.SetMatrix(baseTransform)
							tmpTarget.Push(
								pixel.V(0, size),
								pixel.V(2, 1).Scaled(size/8),
								pixel.V(0, size).Rotated(-120.0*math.Pi/180),
								pixel.V(0, -2.236).Scaled(size/8),
								pixel.V(0, size).Rotated(120.0*math.Pi/180),
								pixel.V(-2, 1).Scaled(size/8),
							)
							tmpTarget.Polygon(3)
							tmpTarget.Push(
								pixel.V(0, size).Rotated(60.0*math.Pi/180),
								pixel.V(2, 1).Scaled(size/8).Rotated(60.0*math.Pi/180),
								pixel.V(0, size).Rotated(-120.0*math.Pi/180).Rotated(60.0*math.Pi/180),
								pixel.V(0, -2.236).Scaled(size/8).Rotated(60.0*math.Pi/180),
								pixel.V(0, size).Rotated(120.0*math.Pi/180).Rotated(60.0*math.Pi/180),
								pixel.V(-2, 1).Scaled(size/8).Rotated(60.0*math.Pi/180),
							)
							tmpTarget.Polygon(3)
							tmpTarget.Draw(imd)
						} else if e.entityType == "blackhole" {
							imd.Color = pixel.ToRGBA(e.color)
							if e.active {
								heartRate := 0.5 - ((float64(e.hp) / 15.0) * 0.35)
								volatility := (math.Mod(totalTime, heartRate) / heartRate)
								size += (5 * volatility)

								ringWeight := 2.0
								if volatility > 0 {
									ringWeight += (3 * volatility)
								}

								hue := (math.Mod(last.Sub(e.born).Seconds(), 6.0))
								baseColor := HSVToColor(hue, 0.5+(volatility/2), 1.0)
								baseColor = baseColor.Add(pixel.Alpha(volatility / 2))

								imd.Color = baseColor
								imd.Push(e.origin)
								imd.Circle(size, ringWeight)

								v2 := math.Mod(volatility+0.5, 1.0)
								hue2 := (math.Mod(last.Sub(e.born).Seconds()+1.0, 6.0))
								baseColor2 := HSVToColor(hue2, 0.5+(v2/2), 1.0)
								baseColor2 = baseColor2.Add(pixel.Alpha(v2 / 2))
								imd.Color = baseColor2
								imd.Push(e.origin)
								imd.Circle(size-ringWeight, ringWeight)
							} else {
								imd.Push(e.origin)
								imd.Circle(size, float64(4))
							}
						} else {
							tmpTarget.Clear()
							tmpTarget.SetMatrix(pixel.IM.Rotated(e.origin, e.orientation.Angle()))
							weight := 3.0
							if e.entityType == "follower" {
								tmpTarget.Color = e.color

								growth := size / 10.0
								timeSinceBorn := last.Sub(e.born).Seconds()

								xRad := (size * 1.2) + (growth * math.Sin(2*math.Pi*(math.Mod(timeSinceBorn, 2.0)/2.0)))
								yRad := (size * 1.2) + (growth * -math.Cos(2*math.Pi*(math.Mod(timeSinceBorn, 2.0)/2.0)))
								tmpTarget.Push(
									pixel.V(e.origin.X-xRad, e.origin.Y),
									pixel.V(e.origin.X, e.origin.Y+yRad),
									pixel.V(e.origin.X+xRad, e.origin.Y),
									pixel.V(e.origin.X, e.origin.Y-yRad),
									pixel.V(e.origin.X-xRad, e.origin.Y),
								)
								tmpTarget.Polygon(weight)
							} else if e.entityType == "pink" {
								weight = 4.0
								tmpTarget.Color = e.color
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
								tmpTarget.Rectangle(weight)
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
								tmpTarget.Line(weight)
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y+size), pixel.V(e.origin.X+size, e.origin.Y-size))
								tmpTarget.Line(weight)
							} else if e.entityType == "pinkpleb" {
								weight = 3.0
								tmpTarget.Color = e.color
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
								tmpTarget.Rectangle(weight)
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
								tmpTarget.Line(weight)
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y+size), pixel.V(e.origin.X+size, e.origin.Y-size))
								tmpTarget.Line(weight)
							} else if e.entityType == "bubble" {
								weight = 2.0
								tmpTarget.Color = pixel.ToRGBA(color.RGBA{66, 135, 245, 192})
								tmpTarget.Push(e.origin)
								tmpTarget.Circle(e.radius, weight)
							} else if e.entityType == "dodger" {
								weight = 3.0
								tmpTarget.SetColorMask(pixel.Alpha(0.8))
								tmpTarget.Color = e.color
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
								tmpTarget.Rectangle(weight)
								tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y), pixel.V(e.origin.X, e.origin.Y+size))
								tmpTarget.Push(pixel.V(e.origin.X+size, e.origin.Y), pixel.V(e.origin.X, e.origin.Y-size))
								tmpTarget.Polygon(weight)
								// tmpTarget.Push(e.origin)
								// tmpTarget.Circle(e.radius, 1.0)
							} else if e.entityType == "snek" {
								weight = 3.0
								// outline := 8.0
								tmpTarget.SetMatrix(pixel.IM.Rotated(pixel.ZV, e.orientation.Angle()-math.Pi/2).Moved(e.origin))
								tmpTarget.Color = e.color
								tmpTarget.Push(pixel.ZV)
								// small := r / 12.0
								// medium := r / 4.0
								// large := r / 2.0

								// tmpTarget.Push(
								// 	pixel.ZV,
								// 	pixel.V(-large, medium),
								// 	pixel.V(-large-medium, large+medium),
								// 	pixel.V(-large-medium, medium),
								// 	pixel.V(-large, -large),
								// 	pixel.V(0.0, -large),
								// 	pixel.V(large, -large),
								// 	pixel.V(large+medium, medium),
								// 	pixel.V(large+medium, large+medium),
								// 	pixel.V(large, medium),
								// )
								tmpTarget.Circle(e.radius, weight)

								tmpTarget.SetMatrix(pixel.IM)
								tmpTarget.Color = colornames.Blueviolet
								for _, snekT := range e.tail {
									if snekT.entityType != "snektail" {
										continue
									}
									tmpTarget.Push(
										snekT.origin,
									)
									tmpTarget.Circle(snekT.radius, 3.0)
								}
							} else if e.entityType == "replicator" {
								tmpTarget.Color = colornames.Orangered
								tmpTarget.Push(e.origin)
								tmpTarget.Circle(e.radius, 4.0)
							} else if e.entityType == "gate" {
								tmpTarget.Color = colornames.Lightyellow
								tmpTarget.Push(e.origin.Add(pixel.V(-e.radius, 0.0)), e.origin.Add(pixel.V(e.radius, 0.0)))
								tmpTarget.Line(4.0)
							}
							tmpTarget.Draw(imd)
						}
					}
				}

				for _, b := range game.data.bullets {
					if b.data.alive {
						bulletDraw.Clear()
						bulletDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, b.data.orientation.Angle()-math.Pi/2).Moved(b.data.origin))
						bulletDraw.SetColorMask(pixel.Alpha(0.9 - (time.Since(b.data.born).Seconds() / b.duration)))
						drawBullet(&b.data, bulletDraw)
						bulletDraw.Draw(imd)
					}
				}
			}

			canvas.Clear(colornames.Black)
			imd.Draw(canvas)

			// draw: wards
			// todo move these batch initializers to run once territory
			innerWardBatch.Clear()
			outerWardBatch.Clear()

			rotInterp := 2 * math.Pi * math.Mod(totalTime, 8.0) / 8
			currentT := math.Sin(rotInterp)
			ang := (currentT * 2 * math.Pi) - math.Pi

			canvas.SetComposeMethod(pixel.ComposePlus)
			if len(player.elements) > 0 {
				innerWardBatch.Clear()
				innerWardBatch.SetMatrix(pixel.IM.Rotated(pixel.ZV, ang).Moved(player.origin))
				el := player.elements[0]
				innerWardBatch.SetColorMask(elements[el])
				wardInner.Draw(innerWardBatch, pixel.IM.Scaled(pixel.ZV, 0.6))
				innerWardBatch.Draw(canvas)
			}

			if len(player.elements) > 1 {
				outerWardBatch.Clear()
				el := player.elements[1]
				outerWardBatch.SetMatrix(pixel.IM.Rotated(pixel.ZV, ang).Moved(player.origin))
				outerWardBatch.SetColorMask(elements[el])
				wardOuter.Draw(outerWardBatch, pixel.IM.Scaled(pixel.ZV, 0.6))
				outerWardBatch.Draw(canvas)
			}

			// if len(player.elements) > 2 {
			// 	innerWardBatch.Clear()
			// 	el := player.elements[2]
			// 	innerWardBatch.SetMatrix(pixel.IM.Rotated(pixel.ZV, ang).Moved(player.origin))
			// 	innerWardBatch.SetColorMask(elements[el])
			// 	wardInner.Draw(innerWardBatch, pixel.IM.Scaled(pixel.ZV, 0.6))
			// 	innerWardBatch.Draw(canvas)
			// }
			canvas.SetComposeMethod(pixel.ComposeOver)

			if game.data.mode != "story" {
				mapRect.Draw(canvas)
			}

			bloom1.Clear(colornames.Black)
			bloom2.Clear(colornames.Black)
			canvas.Draw(bloom1, pixel.IM.Moved(canvas.Bounds().Center()))
			bloom1.Draw(bloom2, pixel.IM.Moved(canvas.Bounds().Center()))
			bloom1.Clear(colornames.Black)
			canvas.Draw(bloom1, pixel.IM.Moved(canvas.Bounds().Center()))
			bloom2.Draw(bloom1, pixel.IM.Moved(canvas.Bounds().Center()))
			bloom1.Draw(bloom3, pixel.IM.Moved(canvas.Bounds().Center()))

			imd.Clear()
			if game.state == "playing" {
				for eID, e := range game.data.entities {
					if (!e.alive && e.death != time.Time{}) {
						// fmt.Print("[DrawBounty]")
						// Draw: bounty
						e.text.Clear()
						e.text.Orig = e.origin
						e.text.Dot = e.origin

						text := fmt.Sprintf("%d", e.bounty*game.data.scoreMultiplier)
						e.text.Dot.X -= (e.text.BoundsOf(text).W() / 2)
						fmt.Fprintf(e.text, "%s", text)
						e.text.Color = colornames.Lightgoldenrodyellow

						growth := (0.5 - (float64(e.expiry.Sub(last).Milliseconds()) / 300.0))
						e.text.Draw(
							canvas,
							pixel.IM.Scaled(e.text.Orig, 1.0-growth),
						)
					}

					if debug {
						e.DrawDebug(fmt.Sprintf("%d", eID), imd, canvas)
					}
				}

				if debug {
					player.DrawDebug("player", imd, canvas)
					for _, debugLog := range debugInfos {
						if debugLog != (debugInfo{}) {
							imd.Color = colornames.Whitesmoke
							imd.Push(debugLog.p1, debugLog.p2)
							imd.Line(2)
						}
					}
					imd.Draw(canvas)
				}
			}

			// stretch the canvas to the window
			win.Clear(colornames.Black)
			win.SetMatrix(pixel.IM.ScaledXY(pixel.ZV,
				pixel.V(
					win.Bounds().W()/bloom2.Bounds().W(),
					win.Bounds().H()/bloom2.Bounds().H(),
				),
			).Moved(win.Bounds().Center()))

			win.SetComposeMethod(pixel.ComposePlus)
			// bloom2.Draw(win, pixel.IM.Moved(bloom2.Bounds().Center()))
			bloom3.Draw(win, pixel.IM.Moved(bloom2.Bounds().Center()))
			canvas.Draw(win, pixel.IM.Moved(canvas.Bounds().Center()))

			imd.Clear()
			imd.Color = colornames.Orange
			imd.SetColorMask(pixel.Alpha(1.0))
			uiCanvas.Clear(colornames.Black)
			if game.state == "playing" {
				scoreTxt.Clear()
				txt := "Score: %d\n"
				scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
				fmt.Fprintf(scoreTxt, txt, game.data.score)
				txt = "X%d\n"
				scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
				fmt.Fprintf(scoreTxt, txt, game.data.scoreMultiplier)

				// if debug {
				txt = "Debugging: On"
				fmt.Fprintln(scoreTxt, txt)

				txt = "Timescale: %.2f\n"
				fmt.Fprintf(scoreTxt, txt, game.data.timescale)

				txt = "Entities: %d\n"
				fmt.Fprintf(scoreTxt, txt, len(game.data.entities))

				txt = "Entities Cap: %d\n"
				fmt.Fprintf(scoreTxt, txt, cap(game.data.entities))

				bufferedSpawns := 0
				for _, ent := range game.data.newEntities {
					if ent.entityType != "" {
						bufferedSpawns++
					}
				}

				// txt = "Buffered Living Entities: %d\n"
				// fmt.Fprintf(scoreTxt, txt, bufferedSpawns)

				// txt = "Buffered Entities Cap: %d\n"
				// fmt.Fprintf(scoreTxt, txt, cap(game.data.newEntities))

				activeParticles := 0
				for _, p := range game.data.particles {
					if p != (particle{}) {
						activeParticles++
					}
				}

				txt = "Particles: %d\n"
				fmt.Fprintf(scoreTxt, txt, activeParticles)
				txt = "Particles Cap: %d\n"
				fmt.Fprintf(scoreTxt, txt, cap(game.data.particles))

				bufferedParticles := 0
				for _, particle := range game.data.newParticles {
					if (particle.origin != pixel.Vec{}) {
						bufferedParticles++
					}
				}

				txt = "Buffered Particles: %d\n"
				fmt.Fprintf(scoreTxt, txt, bufferedParticles)

				txt = "Bullets: %d\n"
				fmt.Fprintf(scoreTxt, txt, len(game.data.bullets))
				txt = "Bullets Cap: %d\n"
				fmt.Fprintf(scoreTxt, txt, cap(game.data.bullets))

				txt = "Kills: %d\n"
				fmt.Fprintf(scoreTxt, txt, game.data.kills)

				txt = "Notoriety: %f\n"
				fmt.Fprintf(scoreTxt, txt, game.data.notoriety)

				txt = "spawnCount: %d\n"
				fmt.Fprintf(scoreTxt, txt, game.data.spawnCount)

				txt = "spawnFreq: %f\n"
				fmt.Fprintf(scoreTxt, txt, game.data.ambientSpawnFreq)

				txt = "waveFreq: %f\n"
				fmt.Fprintf(scoreTxt, txt, game.data.waveFreq)

				txt = "multiplierReward: %d kills required\n"
				fmt.Fprintf(scoreTxt, txt, game.data.multiplierReward-game.data.killsSinceBorn)
				// }

				scoreTxt.Draw(
					win,
					pixel.IM.Scaled(scoreTxt.Orig, 1),
				)

				livesTxt.Clear()
				txt = "Lives: %d\n"
				livesTxt.Dot.X -= (livesTxt.BoundsOf(txt).W() / 2)
				fmt.Fprintf(livesTxt, txt, game.data.lives)
				txt = "Bombs: %d\n"
				livesTxt.Dot.X -= (livesTxt.BoundsOf(txt).W() / 2)
				fmt.Fprintf(livesTxt, txt, game.data.bombs)

				// draw: UI
				uiOrigin := pixel.V(-width/2+128, -height/2+192)

				// WASD
				imd.Color = colornames.Black
				imd.Push(uiOrigin.Add(pixel.V(50, 0)))
				imd.Circle(20.0, 0)

				imd.Push(uiOrigin.Add(pixel.V(10, -50)))
				imd.Circle(20.0, 0)

				imd.Push(uiOrigin.Add(pixel.V(60, -50)))
				imd.Circle(20.0, 0)

				imd.Push(uiOrigin.Add(pixel.V(110, -50)))
				imd.Circle(20.0, 0)

				imd.Color = elementWaterColor
				imd.Push(uiOrigin)
				imd.Circle(20.0, 0)

				imd.Color = elementChaosColor
				imd.Push(uiOrigin.Add(pixel.V(100, 0)))
				imd.Circle(20.0, 0)

				imd.Color = elementSpiritColor
				imd.Push(uiOrigin.Add(pixel.V(150, 0)))
				imd.Circle(20.0, 0)

				imd.Color = elementFireColor
				imd.Push(uiOrigin.Add(pixel.V(160, -50)))
				imd.Circle(20.0, 0)

				imd.Color = elementLightningColor
				imd.Push(uiOrigin.Add(pixel.V(20, -100)))
				imd.Circle(20.0, 0)

				imd.Color = elementWindColor
				imd.Push(uiOrigin.Add(pixel.V(70, -100)))
				imd.Circle(20.0, 0)

				imd.Color = elementLifeColor
				imd.Push(uiOrigin.Add(pixel.V(120, -100)))
				imd.Circle(20.0, 0)
				// imd.Color = elementFireColor
				// imd.Push(uiOrigin.Add(pixel.V(160, -50)))
				// imd.Circle(20.0, 0)

				livesTxt.Draw(
					win,
					pixel.IM.Scaled(livesTxt.Orig, 1),
				)
			} else if game.state == "paused" {
				titleTxt.Clear()
				titleTxt.Orig = pixel.V(0.0, 128.0)
				titleTxt.Dot.X -= titleTxt.BoundsOf(gameTitle).W() / 2
				fmt.Fprintln(titleTxt, gameTitle)
				titleTxt.Draw(
					win,
					pixel.IM.Scaled(
						titleTxt.Orig,
						2,
					),
				)
				imd.Push(
					titleTxt.Orig.Add(pixel.V(-128, -18.0)),
					titleTxt.Orig.Add(pixel.V(128, -18.0)),
				)
				imd.Line(1.0)

				centeredTxt.Orig = pixel.V(-96, 64)
				centeredTxt.Clear()
				for _, item := range game.menu.options {
					if item == game.menu.options[game.menu.selection] {
						imd.Push(centeredTxt.Dot.Add(pixel.V(-12.0, (centeredTxt.LineHeight/2.0)-4)))
						imd.Circle(2.0, 4.0)
					}
					fmt.Fprintln(centeredTxt, item)
				}

				// centeredTxt.Color = color.RGBA64{255, 255, 255, 255}
				centeredTxt.Draw(
					win,
					pixel.IM.Scaled(centeredTxt.Orig, 1),
				)
			} else if game.state == "start_screen" {
				titleTxt.Clear()
				line := gameTitle

				titleTxt.Orig = pixel.Lerp(
					pixel.V(0.0, -400), pixel.V(0.0, 128.0), totalTime/6.0,
				)
				titleTxt.Dot.X -= (titleTxt.BoundsOf(line).W() / 2)
				fmt.Fprintln(titleTxt, line)
				if totalTime > 6.0 {
					game.state = "main_menu"
				}
				titleTxt.Draw(
					win,
					pixel.IM.Scaled(
						titleTxt.Orig,
						5,
					),
				)
			} else if game.state == "main_menu" {
				titleTxt.Clear()
				titleTxt.Orig = pixel.V(0.0, 128.0)
				titleTxt.Dot.X -= titleTxt.BoundsOf(gameTitle).W() / 2
				fmt.Fprintln(titleTxt, gameTitle)
				titleTxt.Draw(
					uiCanvas,
					pixel.IM.Scaled(
						titleTxt.Orig,
						2,
					),
				)
				imd.Push(
					titleTxt.Orig.Add(pixel.V(-128, -18.0)),
					titleTxt.Orig.Add(pixel.V(128, -18.0)),
				)
				imd.Line(1.0)

				centeredTxt.Orig = pixel.V(-112, 64)
				centeredTxt.Clear()
				for _, item := range game.menu.options {
					if sliceextra.Contains(implementedMenuItems, item) {
						centeredTxt.Color = colornames.White
					} else {
						centeredTxt.Color = colornames.Grey
					}
					if item == game.menu.options[game.menu.selection] {
						centeredTxt.Color = colornames.Deepskyblue
						imd.Push(
							centeredTxt.Dot.Add(
								pixel.V(
									-8.0,
									(centeredTxt.LineHeight/2.0)-4.0,
								),
							),
						)
						imd.Circle(2.0, 4.0)
					}
					fmt.Fprintln(centeredTxt, item)
				}

				// centeredTxt.Color = color.RGBA64{255, 255, 255, 255}
				centeredTxt.Draw(
					uiCanvas,
					pixel.IM.Scaled(centeredTxt.Orig, 1),
				)
			} else if game.state == "story_mode" {
				// centeredTxt.Orig = pixel.V(-112, 64)
				// centeredTxt.Clear()
				// for _, page := range chapter1.pages {

				// 		imd.Push(centeredTxt.Dot.Add(pixel.V(-8.0, (centeredTxt.LineHeight/2.0)-4.0)))
				// 		imd.Circle(2.0, 4.0)
				// 	fmt.Fprintln(centeredTxt, item)
				// }

				// // centeredTxt.Color = color.RGBA64{255, 255, 255, 255}
				// centeredTxt.Draw(
				// 	uiCanvas,
				// 	pixel.IM.Scaled(centeredTxt.Orig, 1),
				// )
			} else if game.state == "game_over" {
				titleTxt.Clear()
				titleTxt.Orig = pixel.V(0.0, 128.0)
				titleTxt.Dot.X -= titleTxt.BoundsOf("Game Over").W() / 2
				fmt.Fprintln(titleTxt, "Game Over")
				titleTxt.Draw(
					uiCanvas,
					pixel.IM.Scaled(
						titleTxt.Orig,
						2,
					),
				)
				imd.Push(
					titleTxt.Orig.Add(pixel.V(-128, -18.0)),
					titleTxt.Orig.Add(pixel.V(128, -18.0)),
				)
				imd.Line(1.0)

				gameOverTxt.Draw(
					win,
					pixel.IM.Scaled(
						gameOverTxt.Orig,
						1,
					),
				)
			}

			uiDraw.Draw(uiCanvas)
			imd.Draw(uiCanvas) // refactor away from using this draw target for UI concerns
			uiCanvas.Draw(win, pixel.IM.Moved(uiCanvas.Bounds().Center()))
		}

		win.Update()
	}
}

// To read about how to use these profiles,
// https://blog.golang.org/pprof
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	pixelgl.Run(run)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}
}
