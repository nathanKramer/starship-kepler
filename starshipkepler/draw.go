package starshipkepler

import (
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel/text"
	"github.com/golang/freetype/truetype"
	"github.com/nathanKramer/starship-kepler/sliceextra"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

type DrawContext struct {
	// Draw targets
	mapRect      *imdraw.IMDraw
	imd          *imdraw.IMDraw
	uiDraw       *imdraw.IMDraw
	bulletDraw   *imdraw.IMDraw
	particleDraw *imdraw.IMDraw
	tmpTarget    *imdraw.IMDraw

	// Sprites
	wardInner      *pixel.Sprite
	wardOuter      *pixel.Sprite
	innerWardBatch *imdraw.IMDraw
	outerWardBatch *imdraw.IMDraw

	// Canvases
	PrimaryCanvas *pixelgl.Canvas
	uiCanvas      *pixelgl.Canvas

	// Post processing canvases
	bloom1 *pixelgl.Canvas
	bloom2 *pixelgl.Canvas
	bloom3 *pixelgl.Canvas

	// Fonts
	titleFont *text.Atlas

	// Text objects
	titleTxt     *text.Text
	gameOverTxt  *text.Text
	centeredTxt  *text.Text
	highscoreTxt *text.Text
	scoreTxt     *text.Text
	consoleTxt   *text.Text
	livesTxt     *text.Text
}

var basicFont *text.Atlas
var smallFont *text.Atlas

func NewDrawContext(cfg pixelgl.WindowConfig) *DrawContext {
	drawContext := new(DrawContext)

	mapRect := imdraw.New(nil)
	mapRect.Color = color.RGBA{0x64, 0x64, 0xff, 0xbb}
	mapRect.Push(
		pixel.V(-(worldWidth/2), (worldHeight/2)), // Make a home for these globals
		pixel.V((worldWidth/2), (worldHeight/2)),
	)
	mapRect.Push(
		pixel.V(-(worldWidth/2), -(worldHeight/2)),
		pixel.V((worldWidth/2), -(worldHeight/2)),
	)
	mapRect.Rectangle(4)
	drawContext.mapRect = mapRect

	drawContext.imd = imdraw.New(nil)
	drawContext.uiDraw = imdraw.New(nil)
	drawContext.bulletDraw = imdraw.New(nil)
	drawContext.particleDraw = imdraw.New(nil)
	drawContext.tmpTarget = imdraw.New(nil)

	wardInnerPic, _ := loadPicture("./images/wards/ward_alpha.png")
	wardOuterPic, _ := loadPicture("./images/wards/ward2_alpha.png")

	drawContext.wardInner = pixel.NewSprite(wardInnerPic, wardInnerPic.Bounds())
	drawContext.wardOuter = pixel.NewSprite(wardOuterPic, wardOuterPic.Bounds())

	drawContext.innerWardBatch = imdraw.New(wardInnerPic)
	drawContext.outerWardBatch = imdraw.New(wardOuterPic)

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
	var smallFace font.Face = basicfont.Face7x13

	nFont, err := truetype.Parse(ttfData)
	if err != nil {
		log.Fatal(err)
	} else {
		normalFace = truetype.NewFace(nFont, &truetype.Options{
			Size: 18.0,
			DPI:  96,
		})

		smallFace = truetype.NewFace(nFont, &truetype.Options{
			Size: 14.0,
			DPI:  96,
		})
	}

	drawContext.titleFont = text.NewAtlas(titleFace, text.ASCII)

	// Todo, remove entities direct dependency of font so that this isn't global
	basicFont = text.NewAtlas(normalFace, text.ASCII)
	smallFont = text.NewAtlas(smallFace, text.ASCII)

	drawContext.titleTxt = text.New(pixel.V(0, 128), drawContext.titleFont)
	drawContext.gameOverTxt = text.New(pixel.V(0, 64), basicFont)
	drawContext.centeredTxt = text.New(pixel.V(0, 0), basicFont)
	drawContext.centeredTxt.LineHeight = basicFont.LineHeight() * 1.5

	drawContext.SetBounds(cfg.Bounds)
	return drawContext
}

func (d *DrawContext) SetBounds(bounds pixel.Rect) {
	d.PrimaryCanvas = pixelgl.NewCanvas(pixel.R(-bounds.W()/2, -bounds.H()/2, bounds.W()/2, bounds.H()/2))
	d.uiCanvas = pixelgl.NewCanvas(pixel.R(-bounds.W()/2, -bounds.H()/2, bounds.W()/2, bounds.H()/2))

	d.bloom1 = pixelgl.NewCanvas(pixel.R(-bounds.W()/2, -bounds.H()/2, bounds.W()/2, bounds.H()/2))
	extractBrightness, err := loadFileToString("./shaders/extract_bright_areas.glsl")
	if err != nil {
		panic(err)
	}
	d.bloom1.SetFragmentShader(extractBrightness)

	d.bloom2 = pixelgl.NewCanvas(pixel.R(-bounds.W()/2, -bounds.H()/2, bounds.W()/2, bounds.H()/2))
	blur, err := loadFileToString("./shaders/blur.glsl")
	if err != nil {
		panic(err)
	}
	d.bloom2.SetFragmentShader(blur)

	d.bloom3 = pixelgl.NewCanvas(pixel.R(-bounds.W()/2, -bounds.H()/2, bounds.W()/2, bounds.H()/2))
	// blur, err = loadFileToString("./shaders/blur.glsl")
	// if err != nil {
	// 	panic(err)
	// }
	d.bloom3.SetFragmentShader(blur)

	d.scoreTxt = text.New(pixel.V(-(bounds.W()/2)+120, (bounds.H()/2)-50), basicFont)
	d.highscoreTxt = text.New(pixel.V((bounds.W()/2)-120, (bounds.H()/2)-50), basicFont)
	d.livesTxt = text.New(pixel.V(0.0, (bounds.H()/2)-50), basicFont)
	d.consoleTxt = text.New(pixel.V(-(bounds.W()/2)+50, (bounds.H()/2)-170), smallFont)
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

func drawBullet(bullet *bullet, d *imdraw.IMDraw) {
	d.Color = pixel.ToRGBA(color.RGBA{255, 192, 128, 255})

	hw := bullet.width / 2
	hl := bullet.length / 2
	d.Push(
		pixel.V(hw, hl),
		pixel.V(hw, -hl),
		pixel.V(-hw, hl),
		pixel.V(-hw, -hl),
	)
	d.Rectangle(0)

	for i, el := range bullet.data.elements {
		d.Color = elements[el]
		d.Push(
			pixel.V(hw, hl),
			pixel.V(hw, hl/4.0*float64(i)),
			pixel.V(-hw, hl),
			pixel.V(-hw, hl/4.0*float64(i)),
		)
		d.Rectangle(hw)
	}
}

func drawMenu(d *DrawContext, menu *menu) {
	for _, item := range menu.options {
		if sliceextra.Contains(implementedMenuItems, item) {
			d.centeredTxt.Color = colornames.White
		} else {
			d.centeredTxt.Color = colornames.Grey
		}
		if item == menu.options[menu.selection] {
			d.centeredTxt.Color = colornames.Deepskyblue
			d.imd.Push(
				d.centeredTxt.Dot.Add(
					pixel.V(
						-8.0,
						(d.centeredTxt.LineHeight/2.0)-10,
					),
				),
			)
			d.imd.Circle(2.0, 4.0)
		}
		fmt.Fprintln(d.centeredTxt, item)
	}
}

func drawDebug(d *DrawContext, game *game) {
	txt := "Debugging: On"
	fmt.Fprintln(d.consoleTxt, txt)

	txt = "Timescale: %.2f\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.timescale)

	txt = "Entities: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, len(game.data.entities))

	txt = "Entities Cap: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, cap(game.data.entities))

	bufferedSpawns := 0
	for _, ent := range game.data.newEntities {
		if ent.entityType != "" {
			bufferedSpawns++
		}
	}

	// txt = "Buffered Living Entities: %d\n"
	// fmt.Fprintf(d.consoleTxt, txt, bufferedSpawns)

	// txt = "Buffered Entities Cap: %d\n"
	// fmt.Fprintf(d.consoleTxt, txt, cap(game.data.newEntities))

	activeParticles := 0
	for _, p := range game.data.particles {
		if p != (particle{}) {
			activeParticles++
		}
	}

	txt = "Particles: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, activeParticles)
	txt = "Particles Cap: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, cap(game.data.particles))

	bufferedParticles := 0
	for _, particle := range game.data.newParticles {
		if (particle.origin != pixel.Vec{}) {
			bufferedParticles++
		}
	}

	txt = "Buffered Particles: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, bufferedParticles)

	txt = "Bullets: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, len(game.data.bullets))
	txt = "Bullets Cap: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, cap(game.data.bullets))

	txt = "Kills: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.kills)

	txt = "Notoriety: %f\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.notoriety)

	txt = "spawnCount: %d\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.spawnCount)

	txt = "spawnFreq: %f\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.ambientSpawnFreq)

	txt = "waveFreq: %f\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.waveFreq)

	txt = "multiplierReward: %d kills required\n"
	fmt.Fprintf(d.consoleTxt, txt, game.data.multiplierReward-game.data.killsSinceBorn)
	// }

}

func DrawGame(win *pixelgl.Window, game *game, d *DrawContext) {
	d.imd.Reset()
	d.uiDraw.Reset()
	d.bulletDraw.Reset()
	d.particleDraw.Reset()
	d.tmpTarget.Reset()

	cam := pixel.IM.Moved(game.CamPos.Scaled(-1))
	d.PrimaryCanvas.SetMatrix(cam)

	// draw_
	{
		d.imd.Clear()
		d.uiDraw.Clear()
		d.uiDraw.Color = colornames.Black

		if game.state == "paused" || game.data.mode == "menu" || game.state == "game_over" {
			a := (math.Min(game.totalTime, 4) / 8.0)
			d.PrimaryCanvas.SetColorMask(pixel.Alpha(a))
			d.uiCanvas.SetColorMask(pixel.Alpha(math.Min(1.0, a*4)))
		} else {
			d.PrimaryCanvas.SetColorMask(pixel.Alpha(1.0))
		}

		if game.data.console {
			// draw: console
			w := win.Bounds().W()
			h := win.Bounds().H()
			d.uiDraw.Push(
				pixel.V(-w/2.0, h/2.0),
				pixel.V(-w/2.0, (h/2.0)-32),
				pixel.V(w/2.0, h/2.0),
				pixel.V(w/2.0, (h/2.0)-32),
			)
			d.uiDraw.Rectangle(0.0)
		}

		if game.state == "game_over" {
			d.gameOverTxt.Clear()
			lines := []string{
				"Score: " + fmt.Sprintf("%d", game.data.score),
				"Press enter to restart",
			}
			for _, line := range lines {
				d.gameOverTxt.Dot.X -= (d.gameOverTxt.BoundsOf(line).W() / 2)
				fmt.Fprintln(d.gameOverTxt, line)
			}
		}

		if game.data.mode != "story" {
			// Draw: grid effect
			// TODO, extract?
			// Add catmullrom splines?
			width := len(game.grid.points)
			height := len(game.grid.points[0])
			d.imd.SetColorMask(pixel.Alpha(0.1))
			hue := math.Mod((3.6 + ((math.Mod(game.totalTime, 300.0) / 300.0) * 6.0)), 6.0)
			d.imd.Color = HSVToColor(hue, 0.5, 1.0)

			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					left, up := pixel.ZV, pixel.ZV
					// p := game.grid.points[x][y].origin.ToVec2(win.Bounds())
					p := game.grid.points[x][y].origin.ToVec2(win.Bounds()) // TODO: make sure this is equivalent

					// fmt.Printf("Drawing point %f %f\n", p.X, p.Y)
					if x > 0 {
						left = game.grid.points[x-1][y].origin.ToVec2(win.Bounds())
						if withinWorld(p) || withinWorld(left) {
							// It's possible that one but not the other point is brought in from out of the world boundary
							// If being brought in from out of the world, render right on the border
							enforceWorldBoundary(&p, 0.0)
							enforceWorldBoundary(&left, 0.0)
							thickness := 1.0
							if y%2 == 0 {
								thickness = 4.0
							}
							d.imd.Push(left, p)
							d.imd.Line(thickness)
						}
					}
					if y > 0 {
						up = game.grid.points[x][y-1].origin.ToVec2(win.Bounds())
						if withinWorld(p) || withinWorld(up) {
							// It's possible that one but not the other point is brought in from out of the world boundary
							// If being brought in from out of the world, render right on the border
							enforceWorldBoundary(&p, 0.0)
							enforceWorldBoundary(&up, 0.0)
							thickness := 1.0
							if x%2 == 0 {
								thickness = 4.0
							}
							d.imd.Push(up, p)
							d.imd.Line(thickness)
						}
					}

					if x > 0 && y > 0 {
						upLeft := game.grid.points[x-1][y-1].origin.ToVec2(win.Bounds())
						p1, p2 := upLeft.Add(up).Scaled(0.5), left.Add(p).Scaled(0.5)

						if withinWorld(p1) || withinWorld(p2) {
							enforceWorldBoundary(&p1, 0.0)
							enforceWorldBoundary(&p2, 0.0)
							d.imd.Push(p1, p2)
							d.imd.Line(1.0)
						}

						p3, p4 := upLeft.Add(left).Scaled(0.5), up.Add(p).Scaled(0.5)

						if withinWorld(p3) || withinWorld(p4) {
							enforceWorldBoundary(&p3, 0.0)
							enforceWorldBoundary(&p4, 0.0)
							d.imd.Push(p3, p4)
							d.imd.Line(1.0)
						}
					}
				}
			}

			// draw: particles
			d.imd.SetColorMask(pixel.Alpha(0.4))
			for _, p := range game.data.particles {
				d.particleDraw.Clear()
				if p != (particle{}) {
					defaultSize := pixel.V(8, 2)
					pModel := defaultSize.ScaledXY(p.scale)
					d.particleDraw.Color = p.colour
					d.particleDraw.SetColorMask(pixel.Alpha(p.colour.A))
					d.particleDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, p.orientation).Moved(p.origin))
					d.particleDraw.Push(pixel.V(-pModel.X/2, 0.0), pixel.V(pModel.X/2, 0.0))
					d.particleDraw.Line(pModel.Y)
					d.particleDraw.Draw(d.imd)
				}
			}

			d.imd.Color = colornames.White
			d.imd.SetColorMask(pixel.Alpha(1))

			// draw: game.data.player
			if game.data.player.alive {
				tmpD := imdraw.New(nil)
				tmpD.SetMatrix(pixel.IM.Rotated(pixel.ZV, game.data.player.orientation.Angle()).Moved(game.data.player.origin))
				tmpD.Push(pixel.ZV)

				size := 20.0
				rad := 4.0
				tmpD.Circle(size, rad)
				tmpD.Push(pixel.ZV)
				tmpD.CircleArc(28.0, 0.3, -0.3, 2.0)

				// d.Push(pixel.ZV)
				// d.CircleArc(28.0, 0.2, -0.2, 2.0)
				tmpD.Color = colornames.Lightsteelblue

				if (game.data.weapon != weapondata{}) {
					tmpD.SetMatrix(pixel.IM.Moved(game.data.player.origin))
					tmpD.Push(pixel.V(12.0, 0.0).Rotated(game.data.player.relativeTarget.Angle()))
					tmpD.Circle(4.0, 2.0)
				}
				// game.data.playerDraw.Draw(d)
				tmpD.Draw(d.imd)

				// draw: elements
				// e := imdraw.New(nil)
				// e.SetMatrix(pixel.IM.Moved(game.data.player.origin.Add(pixel.V(-32, -40))))
				// for i := 0; i < len(game.data.player.elements); i++ {
				// 	element := game.data.player.elements[i]
				// 	e.Color = elements[element]

				// 	e.Push(pixel.V(float64(i)*32, 0))
				// 	e.Circle(12, 4)
				// }
				// e.Draw(d.imd)
				// c := pixelgl.NewCanvas(pixel.R(-200, -200, 200, 200))
				// wardInner.Draw(c, pixel.IM)
				// wardOuter.Draw(c, pixel.IM)
				// c.DrawColorMask(d.imd, pixel.IM, elementLifeColor)
			}

			// lastBomb := game.data.lastBomb.Sub(game.lastFrame).Seconds()
			// if game.data.lastBomb.Sub(game.lastFrame).Seconds() < 1.0 {
			// 	// draw: bomb
			// 	d.imd.Color = colornames.White
			// 	d.imd.Push(game.data.player.origin)
			// 	d.imd.Circle(lastBomb*2048.0, 64)
			// }

			// imd.Push(game.data.player.rect.Min, game.data.player.rect.Max)
			// imd.Rectangle(2)

			// draw: enemies
			d.imd.Color = colornames.Lightskyblue
			for _, e := range game.data.entities {
				if e.alive {
					d.imd.SetColorMask(pixel.Alpha(1))
					size := e.radius
					if e.spawning {
						d.imd.SetColorMask(pixel.Alpha(0.7))
						timeSinceBorn := game.lastFrame.Sub(e.born).Seconds()
						spawnIndicatorT := e.spawnTime / 2.0

						size = e.radius * (math.Mod(timeSinceBorn, spawnIndicatorT) / spawnIndicatorT)
						if e.entityType == "blackhole" {
							size = e.radius * ((timeSinceBorn) / e.spawnTime) // grow from small to actual size
						}
					}

					if e.entityType == "wanderer" {
						d.tmpTarget.Clear()
						d.tmpTarget.Color = e.color
						baseTransform := pixel.IM.Rotated(pixel.ZV, e.orientation.Angle()).Moved(e.origin)
						d.tmpTarget.SetMatrix(baseTransform)
						d.tmpTarget.Push(
							pixel.V(0, size),
							pixel.V(2, 1).Scaled(size/8),
							pixel.V(0, size).Rotated(-120.0*math.Pi/180),
							pixel.V(0, -2.236).Scaled(size/8),
							pixel.V(0, size).Rotated(120.0*math.Pi/180),
							pixel.V(-2, 1).Scaled(size/8),
						)
						d.tmpTarget.Polygon(3)
						d.tmpTarget.Draw(d.imd)
					} else if e.entityType == "blackhole" {
						d.imd.Color = pixel.ToRGBA(e.color)
						if e.active {
							heartRate := 0.5 - ((float64(e.hp) / 15.0) * 0.35)
							volatility := (math.Mod(game.totalTime, heartRate) / heartRate)
							size += (5 * volatility)

							ringWeight := 2.0
							if volatility > 0 {
								ringWeight += (3 * volatility)
							}

							hue := (math.Mod(game.lastFrame.Sub(e.born).Seconds(), 6.0))
							baseColor := HSVToColor(hue, 0.5+(volatility/2), 1.0)
							baseColor = baseColor.Add(pixel.Alpha(volatility / 2))

							d.imd.Color = baseColor
							d.imd.Push(e.origin)
							d.imd.Circle(size, ringWeight)

							v2 := math.Mod(volatility+0.5, 1.0)
							hue2 := (math.Mod(game.lastFrame.Sub(e.born).Seconds()+1.0, 6.0))
							baseColor2 := HSVToColor(hue2, 0.5+(v2/2), 1.0)
							baseColor2 = baseColor2.Add(pixel.Alpha(v2 / 2))
							d.imd.Color = baseColor2
							d.imd.Push(e.origin)
							d.imd.Circle(size-ringWeight, ringWeight)
						} else {
							d.imd.Push(e.origin)
							d.imd.Circle(size, float64(4))
						}
					} else {
						d.tmpTarget.Clear()
						d.tmpTarget.SetMatrix(pixel.IM.Rotated(e.origin, e.orientation.Angle()))
						weight := 3.0

						// essences
						if e.entityType == "essence" {
							rotInterp := 2 * math.Pi * math.Mod(game.totalTime, 8.0) / 8
							currentT := math.Sin(rotInterp)
							ang := (currentT * 2 * math.Pi) - math.Pi

							d.tmpTarget.Clear()
							d.tmpTarget.Color = e.color
							baseTransform := pixel.IM.Rotated(pixel.ZV, ang).Moved(e.origin)
							size = size / 2
							d.tmpTarget.SetMatrix(baseTransform)
							d.tmpTarget.Push(
								pixel.V(0, size),
								pixel.V(2, 1).Scaled(size/8),
								pixel.V(0, size).Rotated(-120.0*math.Pi/180),
								pixel.V(0, -2.236).Scaled(size/8),
								pixel.V(0, size).Rotated(120.0*math.Pi/180),
								pixel.V(-2, 1).Scaled(size/8),
							)
							d.tmpTarget.Polygon(3)
							d.tmpTarget.Push(
								pixel.V(0, size).Rotated(60.0*math.Pi/180),
								pixel.V(2, 1).Scaled(size/8).Rotated(60.0*math.Pi/180),
								pixel.V(0, size).Rotated(-120.0*math.Pi/180).Rotated(60.0*math.Pi/180),
								pixel.V(0, -2.236).Scaled(size/8).Rotated(60.0*math.Pi/180),
								pixel.V(0, size).Rotated(120.0*math.Pi/180).Rotated(60.0*math.Pi/180),
								pixel.V(-2, 1).Scaled(size/8).Rotated(60.0*math.Pi/180),
							)
							d.tmpTarget.Polygon(2)
							d.tmpTarget.Draw(d.imd)
						} else if e.entityType == "follower" {
							d.tmpTarget.Color = e.color

							growth := size / 10.0
							timeSinceBorn := game.lastFrame.Sub(e.born).Seconds()

							xRad := (size * 1.2) + (growth * math.Sin(2*math.Pi*(math.Mod(timeSinceBorn, 2.0)/2.0)))
							yRad := (size * 1.2) + (growth * -math.Cos(2*math.Pi*(math.Mod(timeSinceBorn, 2.0)/2.0)))
							d.tmpTarget.Push(
								pixel.V(e.origin.X-xRad, e.origin.Y),
								pixel.V(e.origin.X, e.origin.Y+yRad),
								pixel.V(e.origin.X+xRad, e.origin.Y),
								pixel.V(e.origin.X, e.origin.Y-yRad),
								pixel.V(e.origin.X-xRad, e.origin.Y),
							)
							d.tmpTarget.Polygon(weight)
						} else if e.entityType == "pink" {
							weight = 4.0
							d.tmpTarget.Color = e.color
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
							d.tmpTarget.Rectangle(weight)
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
							d.tmpTarget.Line(weight)
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y+size), pixel.V(e.origin.X+size, e.origin.Y-size))
							d.tmpTarget.Line(weight)
						} else if e.entityType == "pinkpleb" {
							weight = 3.0
							d.tmpTarget.Color = e.color
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
							d.tmpTarget.Rectangle(weight)
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
							d.tmpTarget.Line(weight)
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y+size), pixel.V(e.origin.X+size, e.origin.Y-size))
							d.tmpTarget.Line(weight)
						} else if e.entityType == "bubble" {
							weight = 2.0
							d.tmpTarget.Color = pixel.ToRGBA(color.RGBA{66, 135, 245, 192})
							d.tmpTarget.Push(e.origin)
							d.tmpTarget.Circle(e.radius, weight)
						} else if e.entityType == "dodger" {
							weight = 3.0
							d.tmpTarget.SetColorMask(pixel.Alpha(0.8))
							d.tmpTarget.Color = e.color
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
							d.tmpTarget.Rectangle(weight)
							d.tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y), pixel.V(e.origin.X, e.origin.Y+size))
							d.tmpTarget.Push(pixel.V(e.origin.X+size, e.origin.Y), pixel.V(e.origin.X, e.origin.Y-size))
							d.tmpTarget.Polygon(weight)
							// tmpTarget.Push(e.origin)
							// tmpTarget.Circle(e.radius, 1.0)
						} else if e.entityType == "snek" {
							weight = 3.0
							// outline := 8.0
							d.tmpTarget.SetMatrix(pixel.IM.Rotated(pixel.ZV, e.orientation.Angle()-math.Pi/2).Moved(e.origin))
							d.tmpTarget.Color = e.color
							d.tmpTarget.Push(pixel.ZV)
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
							d.tmpTarget.Circle(e.radius, weight)

							d.tmpTarget.SetMatrix(pixel.IM)
							d.tmpTarget.Color = colornames.Blueviolet
							for _, snekT := range e.tail {
								if snekT.entityType != "snektail" {
									continue
								}
								d.tmpTarget.Push(
									snekT.origin,
								)
								d.tmpTarget.Circle(snekT.radius, 3.0)
							}
						} else if e.entityType == "replicator" {
							d.tmpTarget.Color = colornames.Orangered
							d.tmpTarget.Push(e.origin)
							d.tmpTarget.Circle(e.radius, 4.0)
						} else if e.entityType == "gate" {
							d.tmpTarget.Color = colornames.Lightyellow
							d.tmpTarget.Push(e.origin.Add(pixel.V(-e.radius, 0.0)), e.origin.Add(pixel.V(e.radius, 0.0)))
							d.tmpTarget.Line(4.0)
						}
						d.tmpTarget.Draw(d.imd)
					}
				}
			}

			for _, b := range game.data.bullets {
				if b.data.alive {
					d.bulletDraw.Clear()
					d.bulletDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, b.data.orientation.Angle()-math.Pi/2).Moved(b.data.origin))
					d.bulletDraw.SetColorMask(pixel.Alpha(0.9 - (time.Since(b.data.born).Seconds() / b.duration)))
					drawBullet(&b, d.bulletDraw)
					d.bulletDraw.Draw(d.imd)
				}
			}
		}

		d.PrimaryCanvas.Clear(colornames.Black)
		d.imd.Draw(d.PrimaryCanvas)

		// draw: wards
		// todo move these batch initializers to run once territory
		d.innerWardBatch.Clear()
		d.outerWardBatch.Clear()

		if game.data.player.alive {
			rotInterp := 2 * math.Pi * math.Mod(game.totalTime, 8.0) / 8
			currentT := math.Sin(rotInterp)
			ang := (currentT * 2 * math.Pi) - math.Pi

			d.PrimaryCanvas.SetComposeMethod(pixel.ComposePlus)
			if len(game.data.player.elements) > 0 {
				d.innerWardBatch.Clear()
				d.innerWardBatch.SetMatrix(pixel.IM.Rotated(pixel.ZV, ang).Moved(game.data.player.origin))
				el := game.data.player.elements[0]
				d.innerWardBatch.SetColorMask(elements[el])
				d.wardInner.Draw(d.innerWardBatch, pixel.IM.Scaled(pixel.ZV, 0.6))
				d.innerWardBatch.Draw(d.PrimaryCanvas)
			}

			if len(game.data.player.elements) > 1 {
				d.outerWardBatch.Clear()
				el := game.data.player.elements[1]
				d.outerWardBatch.SetMatrix(pixel.IM.Rotated(pixel.ZV, ang).Moved(game.data.player.origin))
				d.outerWardBatch.SetColorMask(elements[el])
				d.wardOuter.Draw(d.outerWardBatch, pixel.IM.Scaled(pixel.ZV, 0.6))
				d.outerWardBatch.Draw(d.PrimaryCanvas)
			}
		}

		// if len(game.data.player.elements) > 2 {
		// 	d.innerWardBatch.Clear()
		// 	el := game.data.player.elements[2]
		// 	d.innerWardBatch.SetMatrix(pixel.IM.Rotated(pixel.ZV, ang).Moved(game.data.player.origin))
		// 	d.innerWardBatch.SetColorMask(elements[el])
		// 	d.wardInner.Draw(innerWardBatch, pixel.IM.Scaled(pixel.ZV, 0.6))
		// 	d.innerWardBatch.Draw(d.PrimaryCanvas)
		// }
		d.PrimaryCanvas.SetComposeMethod(pixel.ComposeOver)

		if game.data.mode != "story" {
			d.mapRect.Draw(d.PrimaryCanvas)
		}

		d.bloom1.Clear(colornames.Black)
		d.bloom2.Clear(colornames.Black)
		d.PrimaryCanvas.Draw(d.bloom1, pixel.IM.Moved(d.PrimaryCanvas.Bounds().Center()))
		d.bloom1.Draw(d.bloom2, pixel.IM.Moved(d.PrimaryCanvas.Bounds().Center()))
		d.bloom1.Clear(colornames.Black)
		d.PrimaryCanvas.Draw(d.bloom1, pixel.IM.Moved(d.PrimaryCanvas.Bounds().Center()))
		d.bloom2.Draw(d.bloom1, pixel.IM.Moved(d.PrimaryCanvas.Bounds().Center()))
		d.bloom1.Draw(d.bloom3, pixel.IM.Moved(d.PrimaryCanvas.Bounds().Center()))

		d.imd.Clear()
		if game.state == "playing" {
			for eID, e := range game.data.entities {
				if (!e.alive && e.death != time.Time{} && e.entityType != "" && e.bounty > 0) {
					// fmt.Print("[DrawBounty]")
					// Draw: bounty
					e.text.Clear()
					e.text.Orig = e.origin
					e.text.Dot = e.origin

					text := fmt.Sprintf("%d", e.bounty*game.data.scoreMultiplier)
					e.text.Dot.X -= (e.text.BoundsOf(text).W() / 2)
					fmt.Fprintf(e.text, "%s", text)
					e.text.Color = colornames.Lightgoldenrodyellow

					growth := (0.5 - (float64(e.expiry.Sub(game.lastFrame).Milliseconds()) / 300.0))
					e.text.Draw(
						d.PrimaryCanvas,
						pixel.IM.Scaled(e.text.Orig, 1.0-growth),
					)
				}

				if g_debug {
					e.DrawDebug(fmt.Sprintf("%d", eID), d.imd, d.PrimaryCanvas)
				}
			}

			if g_debug {
				game.data.player.DrawDebug("player", d.imd, d.PrimaryCanvas)
				for _, debugLog := range game.debugInfos {
					if debugLog != (debugInfo{}) {
						d.imd.Color = colornames.Whitesmoke
						d.imd.Push(debugLog.p1, debugLog.p2)
						d.imd.Line(2)
					}
				}
				d.imd.Draw(d.PrimaryCanvas)
			}
		}

		// stretch the canvas to the window
		win.Clear(colornames.Black)
		win.SetMatrix(pixel.IM.ScaledXY(pixel.ZV,
			pixel.V(
				win.Bounds().W()/d.bloom2.Bounds().W(),
				win.Bounds().H()/d.bloom2.Bounds().H(),
			),
		).Moved(win.Bounds().Center()))

		win.SetComposeMethod(pixel.ComposePlus)
		// bloom2.Draw(win, pixel.IM.Moved(bloom2.Bounds().Center()))
		d.bloom3.Draw(win, pixel.IM.Moved(d.bloom2.Bounds().Center()))
		d.PrimaryCanvas.Draw(win, pixel.IM.Moved(d.PrimaryCanvas.Bounds().Center()))

		d.imd.Clear()
		d.imd.Color = colornames.Orange
		d.imd.SetColorMask(pixel.Alpha(1.0))
		d.uiCanvas.Clear(colornames.Black)
		if game.state == "playing" {
			d.scoreTxt.Clear()
			txt := "Score: %d\n"
			d.scoreTxt.Dot.X -= (d.scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(d.scoreTxt, txt, game.data.score)
			txt = "X%d\n"
			d.scoreTxt.Dot.X -= (d.scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(d.scoreTxt, txt, game.data.scoreMultiplier)

			d.highscoreTxt.Clear()
			highscore := game.localData.Highscore()
			highscoreTxt := fmt.Sprintf("%s: %d", highscore.Name, highscore.Score)
			d.highscoreTxt.Dot.X -= (d.highscoreTxt.BoundsOf(highscoreTxt).W() / 2)
			fmt.Fprintf(d.highscoreTxt, highscoreTxt)

			d.consoleTxt.Clear()
			if g_debug {
				drawDebug(d, game)
				d.consoleTxt.Draw(win, pixel.IM.Scaled(d.consoleTxt.Orig, 1))
			}

			d.scoreTxt.Draw(
				win,
				pixel.IM.Scaled(d.scoreTxt.Orig, 1),
			)

			if highscore.Score > 0 {
				d.highscoreTxt.Draw(
					win,
					pixel.IM.Scaled(d.highscoreTxt.Orig, 1),
				)
			}

			d.livesTxt.Clear()
			txt = "Lives: %d\n"
			d.livesTxt.Dot.X -= (d.livesTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(d.livesTxt, txt, game.data.lives)
			txt = "Bombs: %d\n"
			d.livesTxt.Dot.X -= (d.livesTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(d.livesTxt, txt, game.data.bombs)

			// draw: UI
			// uiOrigin := pixel.V(-win.Bounds().W()/2+128, -win.Bounds().H()/2+192)

			// WASD
			// d.imd.Color = colornames.Black
			// d.imd.Push(uiOrigin.Add(pixel.V(50, 0)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Push(uiOrigin.Add(pixel.V(10, -50)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Push(uiOrigin.Add(pixel.V(60, -50)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Push(uiOrigin.Add(pixel.V(110, -50)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementWaterColor
			// d.imd.Push(uiOrigin)
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementChaosColor
			// d.imd.Push(uiOrigin.Add(pixel.V(100, 0)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementSpiritColor
			// d.imd.Push(uiOrigin.Add(pixel.V(150, 0)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementFireColor
			// d.imd.Push(uiOrigin.Add(pixel.V(160, -50)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementLightningColor
			// d.imd.Push(uiOrigin.Add(pixel.V(20, -100)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementWindColor
			// d.imd.Push(uiOrigin.Add(pixel.V(70, -100)))
			// d.imd.Circle(20.0, 0)

			// d.imd.Color = elementLifeColor
			// d.imd.Push(uiOrigin.Add(pixel.V(120, -100)))
			// d.imd.Circle(20.0, 0)
			// imd.Color = elementFireColor
			// imd.Push(uiOrigin.Add(pixel.V(160, -50)))
			// imd.Circle(20.0, 0)

			d.livesTxt.Draw(
				win,
				pixel.IM.Scaled(d.livesTxt.Orig, 1),
			)
		} else if game.state == "paused" {
			d.titleTxt.Clear()
			d.titleTxt.Orig = pixel.V(0.0, 128.0)
			d.titleTxt.Dot.X -= d.titleTxt.BoundsOf(gameTitle).W() / 2
			fmt.Fprintln(d.titleTxt, gameTitle)
			d.titleTxt.Draw(
				win,
				pixel.IM.Scaled(
					d.titleTxt.Orig,
					2,
				),
			)
			d.imd.Push(
				d.titleTxt.Orig.Add(pixel.V(-128, -18.0)),
				d.titleTxt.Orig.Add(pixel.V(128, -18.0)),
			)
			d.imd.Line(1.0)

			d.centeredTxt.Orig = pixel.V(-96, 64)
			d.centeredTxt.Clear()
			drawMenu(d, &game.menu)

			// d.centeredTxt.Color = color.RGBA64{255, 255, 255, 255}
			d.centeredTxt.Draw(
				win,
				pixel.IM.Scaled(d.centeredTxt.Orig, 1),
			)
		} else if game.state == "start_screen" {
			d.titleTxt.Clear()
			line := gameTitle

			d.titleTxt.Orig = pixel.Lerp(
				pixel.V(0.0, -400), pixel.V(0.0, 128.0), game.totalTime/6.0,
			)
			d.titleTxt.Dot.X -= (d.titleTxt.BoundsOf(line).W() / 2)
			fmt.Fprintln(d.titleTxt, line)
			if game.totalTime > 6.0 {
				game.state = "main_menu"
			}
			d.titleTxt.Draw(
				win,
				pixel.IM.Scaled(
					d.titleTxt.Orig,
					5,
				),
			)
		} else if game.state == "main_menu" {
			d.titleTxt.Clear()
			d.titleTxt.Orig = pixel.V(0.0, 128.0)
			d.titleTxt.Dot.X -= d.titleTxt.BoundsOf(gameTitle).W() / 2
			fmt.Fprintln(d.titleTxt, gameTitle)
			d.titleTxt.Draw(
				d.uiCanvas,
				pixel.IM.Scaled(
					d.titleTxt.Orig,
					2,
				),
			)
			d.imd.Push(
				d.titleTxt.Orig.Add(pixel.V(-128, -18.0)),
				d.titleTxt.Orig.Add(pixel.V(128, -18.0)),
			)
			d.imd.Line(1.0)

			d.centeredTxt.Orig = pixel.V(-112, 64)
			d.centeredTxt.Clear()
			drawMenu(d, &game.menu)

			// d.centeredTxt.Color = color.RGBA64{255, 255, 255, 255}
			d.centeredTxt.Draw(
				d.uiCanvas,
				pixel.IM.Scaled(d.centeredTxt.Orig, 1),
			)
		} else if game.state == "story_mode" {
			// d.centeredTxt.Orig = pixel.V(-112, 64)
			// d.centeredTxt.Clear()
			// for _, page := range chapter1.pages {

			// 		d.imd.Push(d.centeredTxt.Dot.Add(pixel.V(-8.0, (d.centeredTxt.LineHeight/2.0)-4.0)))
			// 		d.imd.Circle(2.0, 4.0)
			// 	fmt.Fprintln(centeredTxt, item)
			// }

			// // d.centeredTxt.Color = color.RGBA64{255, 255, 255, 255}
			// d.centeredTxt.Draw(
			// 	d.uiCanvas,
			// 	pixel.IM.Scaled(d.centeredTxt.Orig, 1),
			// )
		} else if game.state == "game_over" {
			d.titleTxt.Clear()
			d.titleTxt.Orig = pixel.V(0.0, 128.0)
			d.titleTxt.Dot.X -= d.titleTxt.BoundsOf("Game Over").W() / 2
			fmt.Fprintln(d.titleTxt, "Game Over")
			d.titleTxt.Draw(
				d.uiCanvas,
				pixel.IM.Scaled(
					d.titleTxt.Orig,
					2,
				),
			)
			d.imd.Push(
				d.titleTxt.Orig.Add(pixel.V(-128, -18.0)),
				d.titleTxt.Orig.Add(pixel.V(128, -18.0)),
			)
			d.imd.Line(1.0)

			d.gameOverTxt.Draw(
				win,
				pixel.IM.Scaled(
					d.gameOverTxt.Orig,
					1,
				),
			)
		}

		d.uiDraw.Draw(d.uiCanvas)
		d.imd.Draw(d.uiCanvas) // refactor away from using this draw target for UI concerns
		d.uiCanvas.Draw(win, pixel.IM.Moved(d.uiCanvas.Bounds().Center()))
	}
}
