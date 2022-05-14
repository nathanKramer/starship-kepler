package main

import (
	"flag"
	_ "image/png"
	"log"
	"math/rand"
	"os"
	"runtime/pprof"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"github.com/nathanKramer/starship-kepler/starshipkepler"
)

// Config
type configuration struct {
	screenWidth  float64
	screenHeight float64
	fullscreen   bool
}

// Pull this stuff out into a config file.
var config = configuration{
	1920.0,
	1080.0,
	true,
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

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	draw := starshipkepler.NewDrawContext(cfg)
	game := starshipkepler.NewGame()
	game.PlayGameMusic()
	uiContext := starshipkepler.NewUi(win)

	for !win.Closed() {
		// TODO: Put this scaled mouse position code in either ui or game.
		// Can probably ditch the depenency on the canvas?
		scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) *
			(draw.PrimaryCanvas.Bounds().W() / win.Bounds().W())
		scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) *
			(draw.PrimaryCanvas.Bounds().H() / win.Bounds().H())
		uiContext.MousePos = pixel.V(scaledX, scaledY).Add(game.CamPos)

		starshipkepler.UpdateGame(win, game, uiContext)
		if win.Bounds().W() > 0 && win.Bounds().W() != draw.PrimaryCanvas.Bounds().W() {
			// Resolution changed, tell the draw context
			draw.SetBounds(win.Bounds())
		}

		starshipkepler.DrawGame(win, game, draw)
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
