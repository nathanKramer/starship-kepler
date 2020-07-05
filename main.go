package main

import (
	"fmt"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/nathanKramer/pixel-test/sliceextra"

	"github.com/faiface/beep"
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

const maxParticles = 1600
const worldWidth = 1440.0
const worldHeight = 1080.0
const gameTitle = "Starship Kepler"

var titleFont *text.Atlas
var basicFont *text.Atlas

// Particles

type debugInfo struct {
	p1   pixel.Vec
	p2   pixel.Vec
	text string
}

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
	target                 pixel.Vec // if moving to an arbitrary point, use this
	relativeTarget         pixel.Vec // if moving/aiming at a relative point, use this
	orientation            pixel.Vec // current rotation
	origin                 pixel.Vec
	velocity               pixel.Vec
	pullVec                pixel.Vec // for drawing graphics
	speed                  float64
	acceleration           float64
	friction               float64
	radius                 float64
	spawnTime              float64
	spawning               bool
	born                   time.Time
	death                  time.Time
	expiry                 time.Time
	alive                  bool
	entityType             string
	text                   *text.Text
	color                  color.Color
	hp                     int
	movementColliderRadius float64

	// sounds
	spawnSound *beep.Buffer
	volume     float64

	// enemy data
	bounty       int
	bountyText   string
	killedPlayer bool

	// pink plebs
	virtualOrigin pixel.Vec

	// sneks
	tail          []entityData
	lastTailSpawn time.Time
	bornPos       pixel.Vec
	cone          float64

	// blackholes
	active                bool
	particleEmissionAngle float64

	// debugging
	selected bool
}

func (e *entityData) SpawnSound() beep.Streamer {
	return &effects.Volume{
		Streamer: e.spawnSound.Streamer(0, e.spawnSound.Len()),
		Base:     10,
		Volume:   e.volume,
		Silent:   false,
	}
}

func (e *entityData) IntersectWithPlayer(eID int, game *game, player *entityData, currTime time.Time, entsToAdd []entityData, gameOverTxt *text.Text) {
	if e.entityType == "gate" {
		game.grid.ApplyExplosiveForce(100, Vector3{e.origin.X, e.origin.Y, 0.0}, 100)

		deathSound := blackholeDieBuffer.Streamer(0, blackholeDieBuffer.Len())
		volume := &effects.Volume{
			Streamer: deathSound,
			Base:     10,
			Volume:   -0.25,
			Silent:   false,
		}
		speaker.Play(volume)
		// damage surrounding entities and push them back
		for entID, ent := range game.data.entities {
			if eID == entID || !ent.alive || ent.spawning || ent.entityType == "gate" {
				continue
			}
			dirV := e.origin.Sub(ent.origin)
			dist := dirV.Len()
			if dist < 224 {
				ent.DealDamage(entID, 4, currTime, game, entsToAdd, player)
				game.data.entities[entID] = ent
			}
			game.data.entities[entID] = ent
		}
		e.alive = false
	} else {
		e.killedPlayer = true
		player.alive = false
		player.death = currTime

		for i := 0; i < 1200; i++ {
			speed := 24.0 * (1.0 - 1/((rand.Float64()*32.0)+1))
			p := NewParticle(player.origin.X, player.origin.Y, pixel.ToRGBA(colornames.Lightyellow), 100, pixel.Vec{1.5, 1.5}, 0.0, randomVector(speed), 2.5, "player")
			game.data.particles = append(game.data.particles, p)
		}
		if game.data.mode == "menu_game" {
			game.data.player = *NewPlayer(0.0, 0.0)
			for entID, ent := range game.data.entities {
				if !ent.alive {
					continue
				}
				ent.alive = false
				game.data.entities[entID] = ent
			}
		} else {
			game.data.lives--
			if game.data.lives == 0 {
				game.state = "game_over"

				for entID, ent := range game.data.entities {
					if !ent.alive {
						continue
					}
					ent.alive = false
					game.data.entities[entID] = ent
				}
				game.data.spawning = false

				gameOverTxt.Clear()
				lines := []string{
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
	game.data.entities[eID] = *e
}

func (e *entityData) DealDamage(eID int, amount int, currTime time.Time, game *game, entsToAdd []entityData, player *entityData) []entityData {
	e.hp -= amount
	if e.hp <= 0 {
		e.alive = false
		e.death = currTime
		e.expiry = currTime.Add(time.Millisecond * 300)
		reward := e.bounty * game.data.scoreMultiplier
		e.bountyText = fmt.Sprintf("%d", reward)
		game.data.entities[eID] = *e
		// Draw particles
		hue1 := rand.Float64() * 6.0
		hue2 := math.Mod(hue1+(rand.Float64()*1.5), 6.0)
		for i := 0; i < 120; i++ {
			speed := 24 * (1.0 - (1.0 / ((rand.Float64() * 10.0) + 1.0)))
			t := rand.Float64()
			diff := math.Abs(hue1 - hue2)
			hue := hue1 + (diff * t)

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
					(rand.Float64()*128)-64,
					rand.Float64()*128,
				).Add(e.origin)
				for pos.Sub(player.origin).Len() < 64 { // The player should be able to safely kill at pointblank
					pos = pixel.V(
						(rand.Float64()*128)-64,
						rand.Float64()*128,
					).Add(e.origin)
				}

				pleb := *NewPinkPleb(pos.X, pos.Y)
				entsToAdd = append(entsToAdd, pleb)
			}
			// spawnSound := entsToAdd[0].spawnSound.Streamer(0, entsToAdd[0].spawnSound.Len())
			// speaker.Play(spawnSound)
		} else if e.entityType == "blackhole" {
			game.grid.ApplyExplosiveForce(200, Vector3{e.origin.X, e.origin.Y, 0.0}, 200)
			deathSound := blackholeDieBuffer.Streamer(0, blackholeDieBuffer.Len())
			volume := &effects.Volume{
				Streamer: deathSound,
				Base:     10,
				Volume:   -0.25,
				Silent:   false,
			}
			speaker.Play(volume)
			// damage surrounding entities and push them back
			for entID, ent := range game.data.entities {
				if eID == entID || !ent.alive || ent.spawning {
					continue
				}
				dirV := e.origin.Sub(ent.origin)
				dist := dirV.Len()
				if dist < 150 {
					ent.DealDamage(entID, 4, currTime, game, entsToAdd, player)
					game.data.entities[entID] = ent
				}
				if dist < 350 {
					ent.velocity = ent.velocity.Add(dirV)
				}
				game.data.entities[entID] = ent
			}
		}

		game.data.score += reward
		game.data.scoreSinceBorn += reward
		game.data.killsSinceBorn++
	} else {
		// still alive
		// if black hole, emit sound particles to indicate damage
		if e.entityType == "blackhole" {
			game.grid.ApplyExplosiveForce(e.radius*8, Vector3{e.origin.X, e.origin.Y, 0.0}, e.radius*4)
			e.active = true

			hue1 := rand.Float64() * 6.0
			hue2 := math.Mod(hue1+(rand.Float64()*1.5), 6.0)
			for i := 0; i < 64; i++ {
				speed := 32 * (1.0 - (1.0 / ((rand.Float64() * 10.0) + 1.0)))
				t := rand.Float64()
				diff := math.Abs(hue1 - hue2)
				hue := hue1 + (diff * t)

				p := NewParticle(
					e.origin.X,
					e.origin.Y,
					HSVToColor(hue, 0.5, 1.0),
					64,
					pixel.V(1.0, 1.0),
					0.0,
					randomVector(speed),
					3.0,
					"enemy",
				)

				game.data.particles = append(game.data.particles, p)
			}
		}
	}

	return entsToAdd
}

func (e *entityData) Propel(dir pixel.Vec, dt float64) {
	e.velocity = e.velocity.Add(dir.Scaled(e.acceleration * (dt * 1000) * e.friction))
	if e.velocity.Len() > e.speed {
		e.velocity = e.velocity.Unit().Scaled(e.speed)
	}
}

func (e *entityData) Back(margin float64) pixel.Vec {
	return e.origin.Add(e.orientation.Scaled((-1 * e.radius) - margin))
}

func (e *entityData) Update(dt float64, totalT float64, currTime time.Time) {
	e.velocity = e.velocity.Scaled(e.friction)
	if e.velocity.Len() < 0.2 {
		e.velocity = pixel.ZV
	}

	secondsSinceBirth := currTime.Sub(e.born).Seconds()
	if e.entityType == "pinkpleb" {
		e.virtualOrigin = e.virtualOrigin.Add(e.velocity.Scaled(dt))
		currentT := math.Mod(secondsSinceBirth, 2.0)
		e.origin = e.virtualOrigin.Add(pixel.V(48.0, 0.0).Rotated(currentT * math.Pi))
	} else {
		e.origin = e.origin.Add(e.velocity.Scaled(dt))
	}

	if e.entityType == "snek" {
		e.orientation = e.velocity.Unit()
		nextTailTarget := e.Back(e.radius)
		for tID, snekT := range e.tail {
			if snekT.entityType != "snektail" {
				continue
			}
			snekT.target = nextTailTarget
			snekT.orientation = nextTailTarget.Sub(snekT.Back(snekT.radius)).Unit()
			snekT.origin = nextTailTarget
			e.tail[tID] = snekT
			nextTailTarget = snekT.Back(snekT.radius)
		}
		if len(e.tail) < 20 {
			tailPieceT := nextTailTarget
			if len(e.tail) > 0 {
				tailPieceT = e.tail[len(e.tail)-1].Back(e.radius)
			}
			tailPiece := NewEntity(e.bornPos.X, e.bornPos.Y, math.Max(((e.radius*2)-4.0)-(float64(len(e.tail)+1)), 4.0), e.speed, "snektail")
			tailPiece.target = tailPieceT
			e.tail = append(e.tail, *tailPiece)
			e.lastTailSpawn = currTime
		}
		// for tID, snekT := range e.tail {
		// 	snekT.origin = snekT.origin.Add(snekT.velocity.Scaled(dt))
		// 	e.tail[tID] = snekT
		// }
	}

	if e.entityType == "blackhole" {
		e.radius = 20 + (20 * (float64(e.hp) / 10.0))
	}
	e.enforceWorldBoundary(true)
}

func (e *entityData) DrawDebug(entityID string, imd *imdraw.IMDraw, canvas *pixelgl.Canvas) {
	e.text.Clear()
	e.text.Orig = e.origin
	e.text.Dot = e.origin

	text := fmt.Sprintf("id: %s\npos: [%f, %f]\ntype: %s\n", entityID, e.origin.X, e.origin.Y, e.entityType)

	if e.entityType == "snek" {
		text += fmt.Sprintf("bornPos: [%f, %f]\ntailLength: %d\n", e.bornPos.X, e.bornPos.Y, len(e.tail))
	} else if e.entityType == "blackhole" {
		text += fmt.Sprintf("hp: %d", e.hp)
	}
	size := 1.0

	e.text.Color = colornames.Grey
	if e.selected {
		size = 2.0
		e.text.Color = colornames.White

		text += fmt.Sprintf(`
velocity: [%f,%f]
speed: %f
friction: %f
acceleration: %f
		`, e.velocity.X,
			e.velocity.Y,
			e.velocity.Len(),
			e.friction,
			e.acceleration)
	}

	fmt.Fprintf(e.text, "%s", text)

	e.text.Draw(
		canvas,
		pixel.IM.Scaled(e.text.Orig, size).Moved(pixel.V(50, -50)),
	)

	imd.Color = colornames.Green
	if e.entityType == "gate" {
		imd.Push(e.origin.Add(e.orientation.Scaled(e.radius)), e.origin.Add(e.orientation.Scaled(-e.radius)))
		imd.Line(4.0)

		imd.Color = colornames.Yellow
		imd.Push(e.origin)
		imd.Circle(200.0, 2)
	} else {
		imd.Push(e.origin)
		imd.Circle(e.radius, 2)
	}

	imd.Color = colornames.Blue
	imd.Push(e.origin, e.origin.Add(e.velocity.Scaled(0.5)))
	imd.Line(3)

	imd.Color = colornames.Yellow
	imd.Push(e.origin, e.origin.Add(e.orientation.Scaled(50)))
	imd.Line(3)

	if e.entityType == "pinkpleb" {
		imd.Color = colornames.Orange
		imd.Push(e.origin, e.virtualOrigin)
		imd.Line(3)
	}

	if e.target != (pixel.Vec{}) {
		imd.Color = colornames.Burlywood
		imd.Push(e.origin, e.target)
		imd.Line(2)
	} else {
		// imd.Color = colornames.Red
		// imd.Push(e.origin.Add(pixel.V(20, 20)), e.origin.Add(pixel.V(-20, -20)))
		// imd.Line(2)
		// imd.Push(e.origin.Add(pixel.V(-20, 20)), e.origin.Add(pixel.V(20, -20)))
		// imd.Line(2)
	}

	// imd.Color = colornames.Lawngreen
	// imd.Push(e.Back().Add(pixel.V(10, 10)), e.Back().Add(pixel.V(-10, -10)))
	// imd.Line(2)
	// imd.Push(e.Back().Add(pixel.V(-10, 10)), e.Back().Add(pixel.V(10, -10)))
	// imd.Line(2)

	if e.relativeTarget != (pixel.Vec{}) {
		imd.Color = colornames.Orange
		imd.Push(e.origin, e.origin.Add(e.target.Scaled(100)))
		imd.Line(2)
	}

	// if e.entityType == "snek" {
	// 	for snekID, snekT := range e.tail {
	// 		if snekT.entityType == "snektail" {
	// 			snekT.DrawDebug(fmt.Sprintf("tail-%d", snekID), imd, canvas)
	// 		}
	// 	}
	// }
}

type bullet struct {
	data     entityData
	velocity pixel.Vec
}

func NewEntity(x float64, y float64, size float64, speed float64, entityType string) *entityData {
	p := new(entityData)
	p.target = pixel.Vec{}
	p.orientation = pixel.V(0.0, 1.0)
	p.origin = pixel.V(x, y)
	p.radius = size / 2.0
	p.movementColliderRadius = p.radius
	p.speed = speed
	p.acceleration = 7.0
	p.friction = 0.875
	p.spawnTime = 0.5
	p.spawning = true
	p.hp = 1
	p.alive = true
	p.entityType = entityType
	p.volume = 0.0
	p.born = time.Now()
	p.bornPos = p.origin
	p.text = text.New(pixel.V(0, 0), basicFont)
	return p
}

func NewPlayer(x float64, y float64) *entityData {
	return NewEntity(0.0, 0.0, 44, 575, "player")
}

func NewFollower(x float64, y float64) *entityData {
	e := NewEntity(x, y, 44.0, 280, "follower")
	e.spawnSound = spawnBuffer
	e.color = colornames.Deepskyblue
	e.volume = -0.3
	e.bounty = 50
	e.movementColliderRadius = 24.0
	return e
}

func NewWanderer(x float64, y float64) *entityData {
	w := NewEntity(x, y, 44.0, 200, "wanderer")
	w.spawnSound = spawnBuffer4
	w.color = colornames.Mediumpurple
	w.volume = -0.4
	w.acceleration = 1
	w.bounty = 25
	return w
}

func NewDodger(x float64, y float64) *entityData {
	w := NewEntity(x, y, 44.0, 380, "dodger")
	w.spawnSound = spawnBuffer2
	w.color = colornames.Limegreen
	w.acceleration = 2.0
	w.friction = 0.95
	w.bounty = 100
	return w
}

func NewPinkSquare(x float64, y float64) *entityData {
	w := NewEntity(x, y, 44.0, 460, "pink")
	w.spawnSound = spawnBuffer5
	w.acceleration = 1.0
	w.color = colornames.Hotpink
	w.friction = 0.98
	w.bounty = 100
	return w
}

func NewPinkPleb(x float64, y float64) *entityData {
	w := NewEntity(x, y, 26.0, 256, "pinkpleb")
	// w.spawnSound = spawnBuffer4
	w.virtualOrigin = pixel.V(x, y)
	w.origin = w.virtualOrigin.Add(pixel.V(48.0, 0.0))
	w.color = colornames.Hotpink
	w.spawnTime = 0.0
	w.spawning = false
	w.bounty = 75
	return w
}

func NewSnek(x float64, y float64) *entityData {
	s := NewEntity(x, y, 30.0, 280, "snek")
	s.spawnTime = 0.0
	s.spawning = false
	s.color = colornames.Deepskyblue
	s.spawnSound = snakeSpawnBuffer
	s.volume = -0.8
	s.cone = 30 + (rand.Float64() * 90.0)
	s.lastTailSpawn = time.Now()
	s.bounty = 150
	return s
}

func NewAngryBubble(x float64, y float64) *entityData {
	w := NewEntity(x, y, 35.0, 200, "bubble")
	// w.spawnSound = spawnBuffer4
	w.spawnTime = 0.0
	w.spawning = false
	w.acceleration = 0.9
	w.speed = 600
	w.friction = 0.99
	w.bounty = 100
	return w
}

func NewBlackHole(x float64, y float64) *entityData {
	b := NewEntity(x, y, 40.0, 0.0, "blackhole")
	b.bounty = 150
	// b.color = colornames.Red
	b.spawnSound = spawnBuffer3
	b.volume = -0.6
	b.hp = 10
	b.active = false // dormant until activation (by taking damage)
	return b
}

func NewReplicator(x float64, y float64) *entityData {
	b := NewEntity(x, y, 20.0, 160.0, "replicator")
	b.bounty = 50
	return b
}

func NewGate(x float64, y float64) *entityData {
	b := NewEntity(x, y, 212.0, 40.0, "gate")
	b.orientation = randomVector(1.0)
	b.bounty = 50
	return b
}

func NewBullet(x float64, y float64, speed float64, target pixel.Vec) *bullet {
	b := new(bullet)
	b.data = *NewEntity(x, y, 8, speed, "bullet")
	b.data.target = target
	b.data.orientation = target
	b.velocity = target.Scaled(speed)
	return b
}

// STATE

type menu struct {
	selection int
	options   []string
}

type wavedata struct {
	waveDuration float64
	waveStart    time.Time
	waveEnd      time.Time
	entityType   string
	spawnFreq    float64
	lastSpawn    time.Time
}

type weapondata struct {
	fireRate float64
	fireMode string
}

type gamedata struct {
	mode            string
	lives           int
	bombs           int
	scoreMultiplier int
	entities        []entityData
	bullets         []bullet
	particles       []particle
	spawns          int
	spawnCount      int
	scoreSinceBorn  int
	killsSinceBorn  int
	player          entityData
	weapon          weapondata
	waves           []wavedata

	score             int
	kills             int
	lifeReward        int
	bombReward        int
	multiplierReward  int
	weaponUpgradeFreq float64
	waveFreq          float64
	landingPartyFreq  float64
	ambientSpawnFreq  float64
	notoriety         float64
	timescale         float64
	spawning          bool

	gameStart         time.Time
	lastSpawn         time.Time
	lastBullet        time.Time
	lastBomb          time.Time
	lastWave          time.Time
	lastWeaponUpgrade time.Time
}

type game struct {
	state string
	data  gamedata
	menu  menu
	grid  grid
}

// Menus
var implementedMenuItems = []string{"Quick Play: Evolved", "Quick Play: Pacifism", "Quit"}

func NewMainMenu() menu {
	return menu{
		selection: 1,
		options:   []string{"Story Mode", "Quick Play: Evolved", "Quick Play: Pacifism", "Leaderboard", "Achievements", "Options", "Quit"},
	}
}

func NewPauseMenu() menu {
	return menu{
		selection: 0,
		options:   []string{"Resume", "Main Menu"},
	}
}

func NewWaveData(entityType string, freq float64, duration float64) *wavedata {
	waveData := new(wavedata)
	waveData.waveDuration = duration
	waveData.waveStart = time.Now()
	waveData.spawnFreq = freq
	waveData.entityType = entityType
	waveData.waveEnd = time.Now().Add(time.Second * time.Duration(duration))

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
	weaponData.fireRate = 0.15
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
	gameData.mode = "none"
	gameData.lives = 500
	gameData.bombs = 3
	gameData.scoreMultiplier = 1
	gameData.entities = make([]entityData, 0, 100)
	gameData.bullets = make([]bullet, 0, 100)
	gameData.particles = make([]particle, 0, maxParticles)
	gameData.player = *NewPlayer(0.0, 0.0)
	gameData.spawns = 0
	gameData.spawnCount = 1
	gameData.scoreSinceBorn = 0
	gameData.killsSinceBorn = 0

	gameData.score = 0
	gameData.kills = 0
	gameData.notoriety = 0.0 // brings new enemy types into ambient spawning gradually
	gameData.spawning = true
	gameData.lastSpawn = time.Now()
	gameData.lastBullet = time.Now()
	gameData.lastBomb = time.Now()
	gameData.lastWeaponUpgrade = time.Time{}
	gameData.lastWave = time.Time{}
	gameData.gameStart = time.Now()
	gameData.timescale = 1.0

	return gameData
}

func NewMenuGame() *gamedata {
	data := NewGameData()
	data.mode = "menu_game"
	data.timescale = 0.4
	data.weapon = *NewBurstWeapon()
	data.multiplierReward = 25 // kills
	data.lifeReward = 75000
	data.bombReward = 100000
	data.waveFreq = 30 // waves have a duration so can influence the pace of the game
	data.weaponUpgradeFreq = 30
	data.landingPartyFreq = 10 // more strategic one-off spawn systems
	data.ambientSpawnFreq = 3  // ambient spawning can be toggled off temporarily, but is otherwise always going on
	return data
}

func NewStoryGame() *gamedata {
	data := NewGameData()
	data.mode = "story"
	data.weapon = *NewWeaponData()
	data.multiplierReward = 25 // kills
	data.lifeReward = 75000
	data.bombReward = 100000
	data.waveFreq = 30 // waves have a duration so can influence the pace of the game
	data.weaponUpgradeFreq = 30
	data.landingPartyFreq = 10 // more strategic one-off spawn systems
	data.ambientSpawnFreq = 3  // ambient spawning can be toggled off temporarily, but is otherwise always going on
	return data
}

func NewEvolvedGame() *gamedata {
	data := NewGameData()
	data.mode = "evolved"
	data.weapon = *NewWeaponData()
	data.lives = 3
	data.multiplierReward = 25 // kills
	data.lifeReward = 75000
	data.bombReward = 100000
	data.waveFreq = 30 // waves have a duration so can influence the pace of the game
	data.weaponUpgradeFreq = 30
	data.landingPartyFreq = 10 // more strategic one-off spawn systems
	data.ambientSpawnFreq = 3  // ambient spawning can be toggled off temporarily, but is otherwise always going on
	return data
}

func NewPacifismGame() *gamedata {
	data := NewGameData()
	data.mode = "pacifism"
	data.lives = 1
	data.spawnCount = 3
	data.ambientSpawnFreq = 2 // ambient spawning can be toggled off temporarily, but is otherwise always going on
	return data
}

func NewGame() *game {
	game := new(game)
	game.state = "main_menu"
	game.data = *NewGameData()
	game.menu = NewMainMenu()

	maxGridPoints := 2048.0
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
	data.player = *NewPlayer(0.0, 0.0)
	data.scoreMultiplier = 1
	data.scoreSinceBorn = 0
	data.killsSinceBorn = 0
}

func (data *gamedata) AmbientSpawnFreq() float64 {
	return data.ambientSpawnFreq * data.timescale
}

func (game *game) evolvedGameModeUpdate(debug bool, last time.Time, totalTime float64, player *entityData) {
	// ambient spawns
	if !debug && last.Sub(game.data.lastSpawn).Seconds() > game.data.AmbientSpawnFreq() && game.data.spawning {
		// spawn
		spawns := make([]entityData, 0, game.data.spawnCount)
		for i := 0; i < game.data.spawnCount; i++ {
			pos := pixel.V(
				float64(rand.Intn(worldWidth)-worldWidth/2),
				float64(rand.Intn(worldHeight)-worldHeight/2),
			)
			// to regulate distance from player
			for pos.Sub(player.origin).Len() < 350 {
				pos = pixel.V(
					float64(rand.Intn(worldWidth)-worldWidth/2),
					float64(rand.Intn(worldHeight)-worldHeight/2),
				)
			}

			var enemy entityData
			notoriety := math.Min(0.31, game.data.notoriety)
			r := rand.Float64() * (0.2 + notoriety)
			if r <= 0.1 {
				enemy = *NewWanderer(pos.X, pos.Y)
			} else if r <= 0.4 {
				enemy = *NewFollower(pos.X, pos.Y)
			} else if r <= 0.43 {
				enemy = *NewPinkSquare(pos.X, pos.Y)
			} else if r <= 0.49 {
				enemy = *NewDodger(pos.X, pos.Y)
			} else if r <= 0.5 {
				enemy = *NewBlackHole(pos.X, pos.Y)
			} else {
				enemy = *NewSnek(pos.X, pos.Y)
			}

			spawns = append(spawns, enemy)
		}

		playback := map[string]bool{}
		for _, e := range spawns {
			if playback[e.entityType] || e.spawnSound == nil {
				continue
			}
			playback[e.entityType] = true
			speaker.Play(e.SpawnSound())
		}
		game.data.entities = append(game.data.entities, spawns...)
		game.data.spawns += len(spawns)
		game.data.lastSpawn = time.Now()

		if game.data.spawns%3 == 0 && game.data.spawnCount < 4 {
			game.data.spawnCount++
		}

		if game.data.kills%10 == 0 {
			game.data.notoriety += 0.1
		}

		if game.data.spawns%20 == 0 {
			if game.data.ambientSpawnFreq > 1 {
				game.data.ambientSpawnFreq -= 0.5
			}
		}

		if game.data.spawns%25 == 0 {
			if game.data.waveFreq > 5 {
				game.data.waveFreq -= 5
			}
		}
		if game.data.spawns%3 == 0 && game.data.spawnCount < 4 {
			game.data.spawnCount++
		}

		if game.data.kills%10 == 0 {
			game.data.notoriety += 0.1
		}

		if game.data.spawns%20 == 0 {
			if game.data.ambientSpawnFreq > 1 {
				game.data.ambientSpawnFreq -= 0.5
			}
		}

		if game.data.spawns%25 == 0 {
			if game.data.waveFreq > 5 {
				game.data.waveFreq -= 5
			}
		}
	}

	// wave management
	firstWave := game.data.lastWave == (time.Time{}) && totalTime >= game.data.waveFreq
	subsequentWave := (game.data.lastWave != (time.Time{}) && last.Sub(game.data.lastWave).Seconds() >= game.data.waveFreq)
	if !debug && (firstWave || subsequentWave) {
		corners := [4]pixel.Vec{
			pixel.V(-(worldWidth/2)+80, -(worldHeight/2)+80),
			pixel.V(-(worldWidth/2)+80, (worldHeight/2)-80),
			pixel.V((worldWidth/2)-80, -(worldHeight/2)+80),
			pixel.V((worldWidth/2)-80, (worldHeight/2)-80),
		}
		// one-off landing party
		fmt.Printf("[LandingPartySpawn] %s\n", time.Now().String())
		r := rand.Float64() * (0.4 + math.Min(game.data.notoriety, 0.6))
		if r <= 0.1 {
			count := 2 + rand.Intn(4)
			for i := 0; i < count; i++ {
				p := corners[i%4]
				enemy := NewDodger(
					p.X,
					p.Y,
				)
				game.data.entities = append(game.data.entities, *enemy)
				game.data.spawns++
			}

			spawnSound := spawnBuffer.Streamer(0, spawnBuffer.Len())
			speaker.Play(spawnSound)
		} else if r <= 0.2 {
			count := 2 + rand.Intn(4)
			for i := 0; i < count; i++ {
				p := corners[i%4]
				enemy := NewPinkSquare(
					p.X,
					p.Y,
				)
				game.data.entities = append(game.data.entities, *enemy)
				game.data.spawns++
			}
		} else if r <= 0.25 {
			count := 8 + rand.Intn(4)
			for i := 0; i < count; i++ {
				p := corners[i%4]
				enemy := NewSnek(
					p.X,
					p.Y,
				)
				game.data.entities = append(game.data.entities, *enemy)
				game.data.spawns++
			}
		} else if r <= 0.5 {
			r := rand.Intn(4)
			var t string
			var freq float64
			var duration float64
			switch r {
			case 0:
				t = "follower"
				freq = 0.5
				duration = 10.0
			case 1:
				t = "dodger"
				freq = 0.25
				duration = 3.0
			case 2:
				t = "pink"
				freq = 0.2
				duration = 1.0
			case 3:
				t = "replicator"
				freq = 0.2
				duration = 5.0
			}

			game.data.waves = append(game.data.waves, *NewWaveData(t, freq, duration))
			game.data.lastWave = last
		} else if r <= 0.55 {
			total := 8.0
			step := 360.0 / total
			for i := 0.0; i < total; i++ {
				spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(450.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				game.data.entities = append(
					game.data.entities,
					*NewFollower(spawnPos.X, spawnPos.Y),
					*NewFollower(spawnPos2.X, spawnPos2.Y),
				)
				game.data.spawns += 2
			}
		} else if r <= 0.575 {
			total := 8.0
			step := 360.0 / total
			for i := 0.0; i < total; i++ {
				spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(450.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				game.data.entities = append(
					game.data.entities,
					*NewDodger(spawnPos.X, spawnPos.Y),
					*NewDodger(spawnPos2.X, spawnPos2.Y),
				)
				game.data.spawns += 2
			}
		} else if r <= 0.6 {
			total := 10.0
			step := 360.0 / total
			for i := 0.0; i < total; i++ {
				spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(550.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				game.data.entities = append(
					game.data.entities,
					*NewSnek(spawnPos.X, spawnPos.Y),
					*NewSnek(spawnPos2.X, spawnPos2.Y),
				)
				game.data.spawns += 2
			}
		} else if r <= 0.67 {
			total := 4.0
			step := 360.0 / total
			for i := 0.0; i < total; i++ {
				spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(450.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				game.data.entities = append(
					game.data.entities,
					*NewPinkSquare(spawnPos.X, spawnPos.Y),
					*NewPinkSquare(spawnPos2.X, spawnPos2.Y),
				)
				game.data.spawns += 2
			}
		} else if r <= 0.75 {
			for _, corner := range corners {
				blackhole := NewBlackHole(
					corner.X,
					corner.Y,
				)
				snek := NewSnek(
					corner.X,
					corner.Y,
				)
				pink := NewPinkSquare(
					corner.X,
					corner.Y,
				)
				dodger := NewDodger(
					corner.X,
					corner.Y,
				)
				game.data.entities = append(
					game.data.entities, *blackhole, *snek, *pink, *dodger,
				)
				game.data.spawns += 4
			}
		} else if r <= 0.85 {
			total := 4.0
			step := 360.0 / total
			for i := 0.0; i < total; i++ {
				spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				game.data.entities = append(
					game.data.entities,
					*NewBlackHole(spawnPos.X, spawnPos.Y),
				)
				game.data.spawns += 2
			}
		} else {
			total := 14.0
			step := 360.0 / total
			for i := 0.0; i < total; i++ {
				spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(650.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
				game.data.entities = append(
					game.data.entities,
					*NewFollower(spawnPos.X, spawnPos.Y),
					*NewFollower(spawnPos2.X, spawnPos2.Y),
				)
				game.data.spawns += 2
			}
		}
		game.data.lastWave = last
	}

	for waveID, wave := range game.data.waves {
		if (wave.waveStart != time.Time{}) {
			if last.Sub(wave.waveStart).Seconds() >= wave.waveDuration { // If a wave has ended
				// End the wave
				fmt.Printf("[WaveEnd] %s\n", time.Now().String())
				wave.waveEnd = time.Now()
				wave.waveStart = time.Time{}
				game.data.waves[waveID] = wave
			} else if last.Sub(wave.waveStart).Seconds() < wave.waveDuration {
				// Continue wave
				// TODO make these data driven
				// waves would have spawn points, and spawn counts, and probably durations and stuff
				// hardcoded for now :D

				if last.Sub(wave.lastSpawn).Seconds() > wave.spawnFreq {
					// 4 spawn points
					points := [4]pixel.Vec{
						pixel.V(-(worldWidth/2)+64, -(worldHeight/2)+64),
						pixel.V(-(worldWidth/2)+64, (worldHeight/2)-64),
						pixel.V((worldWidth/2)-64, -(worldHeight/2)+64),
						pixel.V((worldWidth/2)-64, (worldHeight/2)-64),
					}

					for _, p := range points {
						var enemy *entityData
						if wave.entityType == "follower" { // dictionary lookup?
							enemy = NewFollower(
								p.X,
								p.Y,
							)
						} else if wave.entityType == "dodger" {
							enemy = NewDodger(
								p.X,
								p.Y,
							)
						} else if wave.entityType == "pink" {
							enemy = NewPinkSquare(
								p.X,
								p.Y,
							)
						} else if wave.entityType == "replicator" {
							enemy = NewReplicator(
								p.X,
								p.Y,
							)
						}
						game.data.entities = append(game.data.entities, *enemy)
						game.data.spawns++
					}
					spawnSound := spawnBuffer.Streamer(0, spawnBuffer.Len())
					speaker.Play(spawnSound)
					wave.lastSpawn = time.Now()
					game.data.waves[waveID] = wave
				}
			}
		}
	}

	// adjust game rules

	if game.data.score >= game.data.lifeReward {
		game.data.lifeReward += game.data.lifeReward
		game.data.lives++
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
		game.data.bombReward += game.data.bombReward
		game.data.bombs++
	}

	if game.data.killsSinceBorn >= game.data.multiplierReward && game.data.scoreMultiplier < 10 {
		game.data.scoreMultiplier++
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

}

func (game *game) pacifismGameModeUpdate(debug bool, last time.Time, totalTime float64, player *entityData) {
	// ambient spawns
	if !debug && last.Sub(game.data.lastSpawn).Seconds() > game.data.AmbientSpawnFreq() && game.data.spawning {
		// spawn
		spawns := make([]entityData, 0, game.data.spawnCount)
		corners := [4]pixel.Vec{
			pixel.V(-(worldWidth/2)+160, -(worldHeight/2)+160),
			pixel.V(-(worldWidth/2)+160, (worldHeight/2)-160),
			pixel.V((worldWidth/2)-160, -(worldHeight/2)+160),
			pixel.V((worldWidth/2)-160, (worldHeight/2)-160),
		}

		pos := corners[rand.Intn(4)]
		// to regulate distance from player
		for pos.Sub(player.origin).Len() < 350 {
			pos = corners[rand.Intn(4)]
		}
		for i := 0; i < game.data.spawnCount; i++ {
			sPos := pos.Add(pixel.V((rand.Float64()*200.0)-100.0, (rand.Float64()*200.0)-100.0))
			enemy := *NewFollower(sPos.X, sPos.Y)
			spawns = append(spawns, enemy)
		}

		gateCount := game.data.spawnCount / 8
		if gateCount < 1 {
			gateCount = 1
		}
		for i := 0; i < gateCount; i++ {
			pos = pixel.V(
				float64(rand.Intn(worldWidth)-worldWidth/2),
				float64(rand.Intn(worldHeight)-worldHeight/2),
			)
			// to regulate distance from player
			for pos.Sub(player.origin).Len() < 350 {
				pos = pixel.V(
					float64(rand.Intn(worldWidth)-worldWidth/2),
					float64(rand.Intn(worldHeight)-worldHeight/2),
				)
			}
			spawns = append(spawns, *NewGate(pos.X, pos.Y))
		}

		playback := map[string]bool{}
		for _, e := range spawns {
			if playback[e.entityType] || e.spawnSound == nil {
				continue
			}
			playback[e.entityType] = true
			speaker.Play(e.SpawnSound())
		}
		game.data.entities = append(game.data.entities, spawns...)
		game.data.spawns += len(spawns)
		game.data.lastSpawn = time.Now()

		if game.data.spawns%10 == 0 && game.data.spawnCount < 40 {
			game.data.spawnCount++
		}

		if game.data.spawns%20 == 0 {
			if game.data.ambientSpawnFreq > 1 {
				game.data.ambientSpawnFreq -= 0.25
			}
		}
	}
}

func (game *game) respawnPlayer() {
	game.data.respawnPlayer()
	game.data.multiplierReward = 25 // kills
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

func (e *entityData) MovementCollisionCircle() pixel.Circle {
	return pixel.C(e.origin, e.movementColliderRadius)
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

func enforceWorldBoundary(v *pixel.Vec, margin float64) { // this code seems dumb, TODO: find some api call that does it
	minX := -(worldWidth / 2.0) + margin
	minY := -(worldHeight / 2.0) + margin
	maxX := (worldWidth / 2.) - margin
	maxY := (worldHeight / 2.0) - margin
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

func (p *entityData) enforceWorldBoundary(bounce bool) { // this code seems dumb, TODO: find some api call that does it
	minX := -(worldWidth / 2.0) + p.radius
	minY := -(worldHeight / 2.0) + p.radius
	maxX := (worldWidth / 2.) - p.radius
	maxY := (worldHeight / 2.0) - p.radius
	if p.origin.X < minX {
		p.origin.X = minX
		if bounce {
			p.velocity.X *= -1.0
		}
	} else if p.origin.X > maxX {
		p.origin.X = maxX
		if bounce {
			p.velocity.X *= -1.0
		}
	}
	if p.origin.Y < minY {
		p.origin.Y = minY
		if bounce {
			p.velocity.Y *= -1.0
		}
	} else if p.origin.Y > maxY {
		p.origin.Y = maxY
		if bounce {
			p.velocity.Y *= -1.0
		}
	}
}

func loadFileToString(filename string) (string, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(b), nil
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

	imd := imdraw.New(nil)
	playerDraw := imdraw.New(nil)
	bulletDraw := imdraw.New(nil)
	particleDraw := imdraw.New(nil)
	tmpTarget := imdraw.New(nil)
	camPos := pixel.ZV

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

	lastMenuChoice := time.Now()

	// precache player draw
	drawShip(playerDraw)
	// playIntroMusic()
	debugInfos := []debugInfo{}
	totalTime := 0.0

	playMenuMusic()
	game.menu = NewMainMenu()
	game.data = *NewMenuGame()

	for !win.Closed() {
		if game.state == "quitting" {
			win.SetClosed(true)
		}

		// update
		dt := math.Min(time.Since(last).Seconds(), 0.1) * game.data.timescale
		totalTime += dt
		last = time.Now()

		if debug {
			debugInfos = []debugInfo{}
		}

		player := &game.data.player

		// lerp the camera position towards the player
		camPos = pixel.Lerp(
			camPos,
			player.origin.Scaled(0.75),
			1-math.Pow(1.0/128, dt),
		)
		cam := pixel.IM.Moved(camPos.Scaled(-1))
		canvas.SetMatrix(cam)

		if game.state == "main_menu" {
			if win.JustPressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
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
				if game.menu.options[game.menu.selection] == "Quit" {
					game.state = "quitting"
				}
			}

			gamePadDir := pixel.Vec{}
			if win.JoystickPresent(currJoystick) {
				moveVec := thumbstickVector(
					win,
					currJoystick,
					pixelgl.AxisLeftX,
					pixelgl.AxisLeftY,
				)
				gamePadDir = moveVec
			}

			if last.Sub(lastMenuChoice).Seconds() > 0.125 && (win.JustPressed(pixelgl.KeyUp) || gamePadDir.Y > 0.7) {
				game.menu.selection = (game.menu.selection - 1) % len(game.menu.options)
				if game.menu.selection < 0 {
					// would have thought modulo would handle negatives. /shrug
					game.menu.selection += len(game.menu.options)
				}
				for !sliceextra.Contains(implementedMenuItems, game.menu.options[game.menu.selection]) {
					game.menu.selection = (game.menu.selection - 1) % len(game.menu.options)
					if game.menu.selection < 0 {
						// would have thought modulo would handle negatives. /shrug
						game.menu.selection += len(game.menu.options)
					}
				}
				lastMenuChoice = time.Now()
			} else if last.Sub(lastMenuChoice).Seconds() > 0.125 && (win.JustPressed(pixelgl.KeyDown) || gamePadDir.Y < -0.7) {
				game.menu.selection = (game.menu.selection + 1) % len(game.menu.options)
				for !sliceextra.Contains(implementedMenuItems, game.menu.options[game.menu.selection]) {
					game.menu.selection = (game.menu.selection + 1) % len(game.menu.options)
				}
				lastMenuChoice = time.Now()
			}
		}

		if game.state == "start_screen" {
			if win.JustPressed(pixelgl.KeyEnter) || win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
				game.state = "main_menu"
				playMenuMusic()
				game.menu = NewMainMenu()
				game.data = *NewMenuGame()
			}
		}

		if game.state == "paused" {
			if win.Pressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
				game.state = "playing"
			}
			if win.JustPressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA) {
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

			if win.JustPressed(pixelgl.KeyUp) {
				game.menu.selection = (game.menu.selection - 1) % len(game.menu.options)
				if game.menu.selection < 0 {
					// would have thought modulo would handle negatives. /shrug
					game.menu.selection += len(game.menu.options)
				}
			} else if win.JustPressed(pixelgl.KeyDown) {
				game.menu.selection = (game.menu.selection + 1) % len(game.menu.options)
			}
		}

		if game.state == "starting" {
			if game.data.mode == "evolved" {
				game.data = *NewEvolvedGame()
				playMusic()
			} else {
				game.data = *NewPacifismGame()
				playMenuMusic()
			}

			game.state = "playing"
		}

		playerConfirmed := (win.Pressed(pixelgl.KeyEnter) || win.JoystickPressed(currJoystick, pixelgl.ButtonA))
		playerCancelled := win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonB)
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

		direction := pixel.ZV
		if game.state == "playing" {
			if !player.alive {
				game.respawnPlayer()
				game.grid.ApplyDirectedForce(Vector3{0.0, 0.0, 1400.0}, Vector3{player.origin.X, player.origin.Y, 0.0}, 80)
			}

			if win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonStart) {
				game.state = "paused"
				game.menu = NewPauseMenu()
			}

			// player controls
			if win.JustPressed(pixelgl.KeyGraveAccent) {
				debug = !debug
			}
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

			// spawn entities
			// This is a long procedure to allow spawning enemies for test purposes
			if debug {
				scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) * (canvas.Bounds().W() / win.Bounds().W())
				scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) * (canvas.Bounds().H() / win.Bounds().H())
				mp := pixel.V(scaledX, scaledY).Add(camPos)

				if win.JustPressed(pixelgl.KeyJ) {
					enemy := *NewWanderer(
						mp.X,
						mp.Y,
					)
					game.data.entities = append(game.data.entities, enemy)
				}
				if win.JustPressed(pixelgl.KeyK) {
					enemy := *NewFollower(
						mp.X,
						mp.Y,
					)
					game.data.entities = append(game.data.entities, enemy)
				}
				if win.JustPressed(pixelgl.KeyL) {
					enemy := *NewDodger(
						mp.X,
						mp.Y,
					)
					game.data.entities = append(game.data.entities, enemy)
				}
				if win.JustPressed(pixelgl.KeySemicolon) {
					enemy := *NewPinkSquare(
						mp.X,
						mp.Y,
					)
					game.data.entities = append(game.data.entities, enemy)
				}
				if win.JustPressed(pixelgl.KeyRightBracket) {
					enemy := *NewSnek(
						mp.X,
						mp.Y,
					)
					game.data.entities = append(game.data.entities, enemy)
				}
				if win.JustPressed(pixelgl.KeyApostrophe) {
					enemy := *NewBlackHole(
						mp.X,
						mp.Y,
					)
					game.data.entities = append(game.data.entities, enemy)
				}

				total := 16.0
				step := 360.0 / total
				if win.JustPressed(pixelgl.KeyN) {
					for i := 0.0; i < total; i++ {
						spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
						enemy := *NewWanderer(spawnPos.X, spawnPos.Y)
						game.data.entities = append(game.data.entities, enemy)
					}
				}
				if win.JustPressed(pixelgl.KeyM) {
					for i := 0.0; i < total; i++ {
						spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
						enemy := *NewFollower(spawnPos.X, spawnPos.Y)
						game.data.entities = append(game.data.entities, enemy)
					}
				}
				if win.JustPressed(pixelgl.KeyComma) {
					for i := 0.0; i < total; i++ {
						spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
						enemy := *NewDodger(spawnPos.X, spawnPos.Y)
						game.data.entities = append(game.data.entities, enemy)
					}
				}
				if win.JustPressed(pixelgl.KeyPeriod) {
					for i := 0.0; i < total; i++ {
						spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
						enemy := *NewPinkSquare(spawnPos.X, spawnPos.Y)
						game.data.entities = append(game.data.entities, enemy)
					}
				}
				if win.JustPressed(pixelgl.KeySlash) {
					for i := 0.0; i < total; i++ {
						spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
						enemy := *NewBlackHole(spawnPos.X, spawnPos.Y)
						game.data.entities = append(game.data.entities, enemy)
					}
				}

			}
		}

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

			if direction.Len() > 0.5 {
				orientationDt := (pixel.Lerp(
					player.orientation,
					direction,
					1-math.Pow(1.0/512, dt),
				))
				player.orientation = orientationDt
				player.Propel(direction.Unit(), dt)
				// player.velocity = direction.Unit().Scaled(player.speed)

				// partile stream
				baseVelocity := orientationDt.Unit().Scaled(-1 * player.speed).Scaled(dt)
				perpVel := pixel.V(baseVelocity.Y, -baseVelocity.X).Scaled(0.2 * math.Sin(totalTime*10))
				sideColor := pixel.ToRGBA(color.RGBA{200, 128, 9, 192})
				midColor := pixel.ToRGBA(color.RGBA{255, 187, 30, 192})
				white := pixel.ToRGBA(color.RGBA{255, 224, 192, 192})
				pos := player.origin

				vel1 := baseVelocity.Add(perpVel).Add(randomVector((0.2)))
				vel2 := baseVelocity.Sub(perpVel).Add(randomVector((0.2)))
				game.data.particles = append(
					game.data.particles,
					NewParticle(pos.X, pos.Y, midColor, 48.0, pixel.V(0.5, 1.0), 0.0, baseVelocity, 1.0, "ship"),
					NewParticle(pos.X, pos.Y, sideColor, 32.0, pixel.V(1.0, 1.0), 0.0, vel1.Scaled(1.5), 1.0, "ship"),
					NewParticle(pos.X, pos.Y, sideColor, 32.0, pixel.V(1.0, 1.0), 0.0, vel2.Scaled(1.5), 1.0, "ship"),
					NewParticle(pos.X, pos.Y, white, 24.0, pixel.V(0.5, 1.0), 0.0, vel1, 1.0, "ship"),
					NewParticle(pos.X, pos.Y, white, 24.0, pixel.V(0.5, 1.0), 0.0, vel2, 1.0, "ship"),
				)
			}
			player.Update(dt, totalTime, last)

			aim := thumbstickVector(win, currJoystick, pixelgl.AxisRightX, pixelgl.AxisRightY)
			// if !win.Pressed(pixelgl.KeySpace) && aim.Len() == 0 {

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
					}
				} else if win.Pressed(pixelgl.MouseButton1) || win.Pressed(pixelgl.KeyLeftSuper) {
					scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) * (canvas.Bounds().W() / win.Bounds().W())
					scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) * (canvas.Bounds().H() / win.Bounds().H())
					mp := pixel.V(scaledX, scaledY).Add(camPos)
					aim = player.origin.To(mp)
				}

				if aim.Len() > 0.7 {
					// fmt.Printf("Bullet spawned %s", time.Now().String())
					rad := math.Atan2(aim.Unit().Y, aim.Unit().X)
					if game.data.weapon.fireMode == "conic" {
						ang1 := rad + (8 * math.Pi / 180)
						ang2 := rad - (8 * math.Pi / 180)
						ang1Vec := pixel.V(math.Cos(ang1), math.Sin(ang1))
						ang2Vec := pixel.V(math.Cos(ang2), math.Sin(ang2))

						leftB := NewBullet(
							player.origin.X,
							player.origin.Y,
							1400, ang1Vec,
						)
						m := player.origin.Add(aim.Unit().Scaled(10))
						b := NewBullet(
							m.X,
							m.Y,
							1400,
							aim.Unit(),
						)
						rightB := NewBullet(
							player.origin.X,
							player.origin.Y,
							1400,
							ang2Vec,
						)
						game.data.bullets = append(game.data.bullets, *leftB, *b, *rightB)
					} else if game.data.weapon.fireMode == "burst" {
						bulletCount := 5
						width := 24.0
						for i := 0; i < bulletCount; i++ {
							bPos := pixel.V(
								25.0,
								-(width/2)+(float64(i)*(width/float64(bulletCount))),
							).Rotated(
								rad,
							).Add(player.origin)

							increment := (float64(i) * math.Pi / 180.0) - (2 * math.Pi / 180)
							b := NewBullet(
								bPos.X,
								bPos.Y,
								1100,
								aim.Unit().Rotated(increment),
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

					shot := shotBuffer3.Streamer(0, shotBuffer3.Len())
					volume := &effects.Volume{
						Streamer: shot,
						Base:     10,
						Volume:   -1.3,
						Silent:   false,
					}

					if game.data.weapon.fireMode == "conic" {
						shot = shotBuffer2.Streamer(0, shotBuffer2.Len())
						volume = &effects.Volume{
							Streamer: shot,
							Base:     10,
							Volume:   -0.7,
							Silent:   false,
						}
					} else if game.data.weapon.fireMode == "burst" {
						shot = shotBuffer.Streamer(0, shotBuffer.Len())
						volume = &effects.Volume{
							Streamer: shot,
							Base:     10,
							Volume:   -0.9,
							Silent:   false,
						}
					}
					speaker.Play(volume)
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

							midColor := pixel.ToRGBA(color.RGBA{64, 232, 64, 192})
							pos1 := pixel.V(1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							pos2 := pixel.V(-1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							pos3 := pixel.V(1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							pos4 := pixel.V(-1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
							game.data.particles = append(
								game.data.particles,
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
					continue
				}
				b.data.origin = b.data.origin.Add(b.velocity.Scaled(dt))
				if game.data.weapon.fireMode == "burst" {
					if math.Mod(totalTime, 0.4) < 0.8 {
						game.grid.ApplyExplosiveForce(b.velocity.Scaled(dt).Len()*1.25, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 60.0)
					}
				} else {
					game.grid.ApplyDirectedForce(Vector3{b.velocity.X * dt * 0.05, b.velocity.Y * dt * 0.05, 0.0}, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 40.0)
				}

				game.data.bullets[i] = b
			}

			// Process blackholes
			// I don't really care about efficiency atm
			for eID, e := range game.data.entities {
				e.pullVec = pixel.ZV // reset the pull vec (which is used for drawing effects related to the pull)
				game.data.entities[eID] = e
			}

			entsToAdd := make([]entityData, 0, 100)
			for bID, b := range game.data.entities {
				if !b.alive || b.entityType != "blackhole" || !b.active {
					continue
				}

				// emit particles
				if (uint64(totalTime*1000)/125)%2 == 0 {
					v := 6.0 + (rand.Float64() * 12)
					sprayVelocity := pixel.V(math.Cos(b.particleEmissionAngle), math.Sin(b.particleEmissionAngle)).Unit().Scaled(v * game.data.timescale)
					color := colornames.Lightskyblue
					pos := b.origin
					game.data.particles = append(
						game.data.particles,
						NewParticle(pos.X, pos.Y, pixel.ToRGBA(color), 128.0, pixel.V(1.5, 1.5), 0.0, sprayVelocity, 2.0, "blackhole"),
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
						entsToAdd = append(entsToAdd, pleb)
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

						game.data.particles = append(game.data.particles, p)
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
			game.data.entities = append(game.data.entities, entsToAdd...)

			for _, e := range game.data.entities {
				baseVelocity := e.pullVec.Scaled(0.4)

				if e.pullVec.Len() > 0 && e.color != nil {
					midColor := pixel.ToRGBA(e.color)
					pos1 := pixel.V(1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					pos2 := pixel.V(-1, 1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					pos3 := pixel.V(1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					pos4 := pixel.V(-1, -1).Rotated(e.orientation.Angle()).Scaled(e.radius).Add(e.origin)
					game.data.particles = append(
						game.data.particles,
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

			scaledX := (win.MousePosition().X - (win.Bounds().W() / 2)) * (canvas.Bounds().W() / win.Bounds().W())
			scaledY := (win.MousePosition().Y - (win.Bounds().H() / 2)) * (canvas.Bounds().H() / win.Bounds().H())
			mp := pixel.V(scaledX, scaledY).Add(camPos)
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

			entsToAdd = make([]entityData, 0, 100)
			for bID, b := range game.data.bullets {
				if b.data.alive {
					for eID, e := range game.data.entities {
						if e.alive && !e.spawning && b.data.Circle().Intersect(e.Circle()).Radius > 0 {
							b.data.alive = false
							game.data.bullets[bID] = b

							if e.entityType == "blackhole" {
								hitSound := blackholeHitBuffer.Streamer(0, blackholeHitBuffer.Len())
								volume := &effects.Volume{
									Streamer: hitSound,
									Base:     10,
									Volume:   -1.4,
									Silent:   false,
								}
								speaker.Play(volume)
							}
							entsToAdd = e.DealDamage(eID, 1, last, game, entsToAdd, player)

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
							p := NewParticle(b.data.origin.X, b.data.origin.Y, pixel.ToRGBA(colornames.Lightblue), 32, pixel.Vec{1.0, 1.0}, 0.0, randomVector(5.0), 1.0, "bullet")
							game.data.particles = append(game.data.particles, p)
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
					e.IntersectWithPlayer(eID, game, player, last, entsToAdd, gameOverTxt)
				}
			}
			game.data.entities = append(game.data.entities, entsToAdd...)

			// check for bomb here for now
			bombPressed := win.Pressed(pixelgl.KeyR) || win.Pressed(pixelgl.KeySpace) || win.JoystickAxis(currJoystick, pixelgl.AxisRightTrigger) > 0.1
			if game.data.bombs > 0 && bombPressed && last.Sub(game.data.lastBomb).Seconds() > 3.0 {
				game.grid.ApplyExplosiveForce(256.0, Vector3{player.origin.X, player.origin.Y, 0.0}, 256.0)
				sound := bombBuffer.Streamer(0, bombBuffer.Len())
				volume := &effects.Volume{
					Streamer: sound,
					Base:     10,
					Volume:   0.7,
					Silent:   false,
				}
				speaker.Play(volume)

				for i := 0; i < 2400; i++ {
					speed := 48.0 * (1.0 - 1/((rand.Float64()*32.0)+1))
					p := NewParticle(player.origin.X, player.origin.Y, pixel.ToRGBA(colornames.Lightyellow), 100, pixel.Vec{1.5, 1.5}, 0.0, randomVector(speed), 2.0, "player")
					game.data.particles = append(game.data.particles, p)
				}

				game.data.lastBomb = time.Now()

				game.data.bombs--
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

			if game.data.mode == "evolved" || game.data.mode == "menu_game" {
				game.evolvedGameModeUpdate(debug, last, totalTime, player)
			} else if game.data.mode == "pacifism" {
				game.pacifismGameModeUpdate(debug, last, totalTime, player)
			}
		}

		// draw
		imd.Clear()

		if game.state == "paused" || game.data.mode == "menu_game" || game.state == "game_over" {
			a := (math.Min(totalTime, 4) / 8.0)
			canvas.SetColorMask(pixel.Alpha(a))
			uiCanvas.SetColorMask(pixel.Alpha(math.Min(1.0, a*4)))
		} else {
			canvas.SetColorMask(pixel.Alpha(1.0))
		}

		// Draw the grid effect
		// TODO, extract?
		// Add catmullrom splines?
		if game.data.mode == "evolved" || game.data.mode == "pacifism" || game.data.mode == "menu_game" {
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

			imd.Color = colornames.White
			imd.SetColorMask(pixel.Alpha(1))

			// draw player
			if player.alive {
				d := imdraw.New(nil)
				d.SetMatrix(pixel.IM.Rotated(pixel.ZV, player.orientation.Angle()-math.Pi/2).Moved(player.origin))
				playerDraw.Draw(d)
				d.Draw(imd)
			}

			// lastBomb := game.data.lastBomb.Sub(last).Seconds()
			// if game.data.lastBomb.Sub(last).Seconds() < 1.0 {
			// 	// draw bomb
			// 	imd.Color = colornames.White
			// 	imd.Push(player.origin)
			// 	imd.Circle(lastBomb*2048.0, 64)
			// }

			// imd.Push(player.rect.Min, player.rect.Max)
			// imd.Rectangle(2)

			// draw enemies
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
						imd.Color = pixel.ToRGBA(colornames.Red)
						if e.active {
							heartRate := 0.5 - ((float64(e.hp) / 15.0) * 0.35)
							volatility := 5 * (math.Mod(totalTime, heartRate) / heartRate)
							imd.Color = color.RGBA{255, uint8(24 + (38 * volatility)), uint8(24 + (38 * volatility)), 224}

							size += volatility

							ringWeight := 2.0
							if volatility > 0 {
								ringWeight += volatility
							}

							imd.Push(e.origin)
							imd.Circle(size, ringWeight)
						} else {
							imd.Push(e.origin)
							imd.Circle(size, float64(4))
						}
					} else {
						tmpTarget.Clear()
						tmpTarget.SetMatrix(pixel.IM.Rotated(e.origin, e.orientation.Angle()))
						weight := 2.0
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
							tmpTarget.Color = e.color
							tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y-size), pixel.V(e.origin.X+size, e.origin.Y+size))
							tmpTarget.Rectangle(weight)
							tmpTarget.Push(pixel.V(e.origin.X-size, e.origin.Y), pixel.V(e.origin.X, e.origin.Y+size))
							tmpTarget.Push(pixel.V(e.origin.X+size, e.origin.Y), pixel.V(e.origin.X, e.origin.Y-size))
							tmpTarget.Polygon(weight)
						} else if e.entityType == "snek" {
							weight = 2.0
							tmpTarget.Color = pixel.ToRGBA(color.RGBA{66, 135, 245, 192})
							tmpTarget.Push(e.origin)
							tmpTarget.Circle(e.radius, weight)

							tmpTarget.SetMatrix(pixel.IM)
							tmpTarget.Color = colornames.Orangered
							for _, snekT := range e.tail {
								if snekT.entityType != "snektail" {
									continue
								}
								tmpTarget.Push(snekT.origin)
								tmpTarget.Circle(snekT.radius, weight)
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

			bulletDraw.Color = pixel.ToRGBA(color.RGBA{255, 192, 128, 255})
			for _, b := range game.data.bullets {
				if b.data.alive {
					bulletDraw.Clear()
					bulletDraw.SetMatrix(pixel.IM.Rotated(pixel.ZV, b.data.orientation.Angle()-math.Pi/2).Moved(b.data.origin))
					drawBullet(&b.data, bulletDraw)
					bulletDraw.Draw(imd)
				}
			}
		}

		canvas.Clear(colornames.Black)
		imd.Draw(canvas)

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
					// Draw the bounty
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
		uiCanvas.Clear(colornames.Black)
		if game.state == "playing" {
			scoreTxt.Clear()
			txt := "Score: %d\n"
			scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(scoreTxt, txt, game.data.score)
			txt = "X%d\n"
			scoreTxt.Dot.X -= (scoreTxt.BoundsOf(txt).W() / 2)
			fmt.Fprintf(scoreTxt, txt, game.data.scoreMultiplier)

			if debug {
				txt = "Debugging: On"
				fmt.Fprintln(scoreTxt, txt)

				txt = "Timescale: %.2f\n"
				fmt.Fprintf(scoreTxt, txt, game.data.timescale)

				txt = "Entities: %d\n"
				fmt.Fprintf(scoreTxt, txt, len(game.data.entities))

				txt = "Particles: %d\n"
				fmt.Fprintf(scoreTxt, txt, len(game.data.particles))
			}

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
					imd.Push(centeredTxt.Dot.Add(pixel.V(-8.0, (centeredTxt.LineHeight/2.0)-4.0)))
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
		imd.Draw(uiCanvas)
		uiCanvas.Draw(win, pixel.IM.Moved(uiCanvas.Bounds().Center()))

		win.Update()
	}
}

func main() {
	pixelgl.Run(run)
}
