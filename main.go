package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"golang.org/x/image/colornames"
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

var g_debug = false

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

type debugInfo struct {
	p1   pixel.Vec
	p2   pixel.Vec
	text string
}

// STATE

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

	draw := NewDrawContext(cfg)
	game := NewGame()
	uiContext := NewUi(win)

	lastMemCheck := time.Now()

	// Todo - move this type of stuff somewhere else
	playMenuMusic()
	game.menu = NewMainMenu()
	game.data = *NewMenuGame()

	PrintMemUsage()

	for !win.Closed() {
		if game.lastFrame.Sub(lastMemCheck).Seconds() > 5.0 {
			PrintMemUsage()
			fmt.Printf("Entities\tlen: %d\tcap: %d\n", len(game.data.entities), cap(game.data.entities))
			fmt.Printf("New Entities\tlen: %d\tcap: %d\n\n", len(game.data.newEntities), cap(game.data.newEntities))

			fmt.Printf("Bullets\tlen: %d\tcap: %d\n", len(game.data.bullets), cap(game.data.bullets))
			fmt.Printf("New Bullets\tlen: %d\tcap: %d\n\n", len(game.data.newBullets), cap(game.data.newBullets))

			fmt.Printf("Particles\tlen: %d\tcap: %d\n", len(game.data.particles), cap(game.data.particles))
			fmt.Printf("New Particles\tlen: %d\tcap: %d\n\n", len(game.data.newParticles), cap(game.data.newParticles))

			lastMemCheck = game.lastFrame

			// runtime.GC()
		}

		// TODO: Put this scaled mouse position code in either ui or game.
		// Can probably ditch the depenency on the canvas?
		scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) *
			(draw.primaryCanvas.Bounds().W() / win.Bounds().W())
		scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) *
			(draw.primaryCanvas.Bounds().H() / win.Bounds().H())
		uiContext.mousePos = pixel.V(scaledX, scaledY).Add(game.camPos)

		updateGame(win, game, uiContext)
		drawGame(win, game, draw)

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
