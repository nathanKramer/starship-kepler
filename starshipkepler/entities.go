package starshipkepler

import (
	"fmt"
	"image/color"
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
)

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

	elements []string

	// sounds
	spawnSound *beep.Buffer
	volume     float64

	// enemy data
	bounty       int
	bountyText   string
	killedPlayer bool

	// pink plebs, and other entities with virtual positions
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

// Not 100% sure if these "inline append" functions are needed anymore
// (They were a blind attempt at solving a performance issue that turned out to be completely unrelated)

func InlineAppendParticles(
	particles []particle,
	particlesToAdd ...particle) []particle {

	// config var
	if !particlesOn {
		return particles
	}
	for _, newParticle := range particlesToAdd {
		if newParticle == (particle{}) {
			continue
		}
		particleID := 0
		if len(particles) < cap(particles) {
			particles = append(particles, newParticle)
		} else {
			for particleID < len(particles) {
				existing := particles[particleID]
				if existing == (particle{}) {
					particles[particleID] = newParticle
					break
				}
				particleID++
			}
		}
	}

	return particles
}

func InlineAppendBullets(
	bullets []bullet,
	bulletsToAdd ...bullet) []bullet {
	for _, newBullet := range bulletsToAdd {
		if newBullet.velocity == (pixel.Vec{}) {
			continue
		}

		bulletID := 0
		if len(bullets) < cap(bullets) {
			bullets = append(bullets, newBullet)
		} else {
			for bulletID < len(bullets) {
				existing := bullets[bulletID]
				if existing.velocity == (pixel.Vec{}) {
					bullets[bulletID] = newBullet
					break
				}
				bulletID++
			}
		}
	}

	return bullets
}

func InlineAppendEntities(
	entities []entityData,
	entitiesToAdd ...entityData) []entityData {
	for _, newEntity := range entitiesToAdd {
		if newEntity.entityType == "" {
			continue
		}

		entId := 0

		if len(entities) < cap(entities) {
			entities = append(entities, newEntity)
		} else {
			for entId < len(entities) {
				existing := entities[entId]
				if existing.entityType == "" || (!existing.alive && !existing.spawning) {
					entities[entId] = newEntity
					break
				}
				entId++
			}
		}
	}

	return entities
}

func (e *entityData) SpawnSound() beep.Streamer {
	return &effects.Volume{
		Streamer: e.spawnSound.Streamer(0, e.spawnSound.Len()),
		Base:     10,
		Volume:   e.volume,
		Silent:   false,
	}
}

func PlaySpawnSounds(spawns []entityData) {
	playback := map[string]bool{}
	for _, e := range spawns {
		if playback[e.entityType] || e.spawnSound == nil {
			continue
		}
		playback[e.entityType] = true
		speaker.Play(e.SpawnSound())
	}
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

func (e *entityData) MovementCollisionCircle() pixel.Circle {
	return pixel.C(e.origin, e.movementColliderRadius)
}

func (e *entityData) Circle() pixel.Circle {
	return pixel.C(e.origin, e.radius)
}

func (e *entityData) IntersectWithPlayer(
	inflictor entityData,
	eID int,
	game *game,
	player *entityData,
	currTime time.Time) {
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
				ent.DealDamage(&ent, entID, 4, currTime, game, player)
				game.data.entities[entID] = ent
			}
			game.data.entities[entID] = ent
		}
		e.alive = false
	} else if e.entityType == "essence" {
		e.alive = false
		e.death = currTime
		player.QueueElement(e.elements[0])
	} else {
		warded := false
		for _, el := range e.elements {
			for _, playerEl := range player.elements {
				if el == playerEl {
					warded = true
					PlaySound("ward/die")
					e.DealDamage(player, eID, 1, currTime, game, player)
					break
				}
			}
		}

		// player died
		if !warded && !g_debug {
			e.killedPlayer = true
			player.alive = false
			player.death = currTime
			game.data.lastWave = time.Now().Add((time.Duration(-game.data.waveFreq) + 2) * time.Second)
			game.data.spawning = false
			PlaySound("player/die")

			for i := 0; i < 1200; i++ {
				speed := 24.0 * (1.0 - 1/((rand.Float64()*32.0)+1))
				p := NewParticle(
					player.origin.X,
					player.origin.Y,
					pixel.ToRGBA(colornames.Lightyellow),
					100,
					pixel.V(
						1.5,
						1.5,
					),
					0.0,
					randomVector(speed),
					2.5,
					"player",
				)
				game.data.newParticles = InlineAppendParticles(game.data.newParticles, p)
			}

			e.alive = false
			for entID, ent := range game.data.entities {
				ent.alive = false
				game.data.entities[entID] = ent
			}

			if game.data.mode == "menu" {
				game.data.player = *NewPlayer(0.0, 0.0)
			} else {
				game.data.lives--
				if game.data.lives == 0 {
					GameOver(game)
				}
			}
		}

	}
	game.data.entities[eID] = *e
}

func (b *bullet) DealDamage(
	inflictor *entityData,
	bID int,
	amount int,
	currTime time.Time,
	game *game,
	player *entityData,
) {
	for _, el := range b.data.elements {
		for i, inflictorEl := range inflictor.elements {
			if el == inflictorEl {
				b.data.elements = append(b.data.elements[:i], b.data.elements[i+1:]...)
				game.data.bullets[bID] = *b
				return
			}
		}
	}

	b.data.hp -= amount
	if b.data.hp <= 0 {
		b.data.alive = false
		game.data.bullets[bID] = *b
	}
}

func (e *entityData) DealDamage(
	inflictor *entityData,
	eID int,
	amount int,
	currTime time.Time,
	game *game,
	player *entityData,
) {

	if e.entityType == "essence" {
		return
	}

	for _, el := range e.elements {
		for i, inflictorEl := range inflictor.elements {
			if el == inflictorEl {
				inflictor.elements = append(inflictor.elements[:i], inflictor.elements[i+1:]...)
				break
			}
		}
	}

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

			if len(game.data.newParticles) < maxParticles {
				game.data.newParticles = InlineAppendParticles(game.data.newParticles, p)
			} else {
				game.data.newParticles[len(game.data.newParticles)%maxParticles] = p
			}
		}

		if len(e.elements) > 0 {
			r := rand.Float64()
			if r < 0.1 {
				essence := *NewEssence(
					e.origin.X, e.origin.Y, e.elements[0], e.color, game.lastFrame.Add(time.Duration(5)*time.Second),
				)
				game.data.newEntities = InlineAppendEntities(
					game.data.newEntities, essence,
				)
			}
		}

		// on kill
		PlaySound("entity/die")
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
				game.data.newEntities = InlineAppendEntities(game.data.newEntities, pleb)
			}
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
					ent.DealDamage(&ent, entID, 4, currTime, game, player)
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
		game.data.kills++
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

				game.data.newParticles = InlineAppendParticles(game.data.newParticles, p)
			}
		}
	}
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
		enforceWorldBoundary(&nextTailTarget, e.radius)
		for tID, snekT := range e.tail {
			if snekT.entityType != "snektail" {
				continue
			}
			snekT.target = nextTailTarget
			snekT.orientation = nextTailTarget.Sub(snekT.Back(snekT.radius)).Unit()
			snekT.origin = nextTailTarget
			e.tail[tID] = snekT
			nextTailTarget = snekT.Back(snekT.radius)
			enforceWorldBoundary(&nextTailTarget, e.radius)
		}
		if len(e.tail) < 16 {
			tailPieceT := nextTailTarget
			if len(e.tail) > 0 {
				tailPieceT = e.tail[len(e.tail)-1].Back(e.radius)
			}
			tailPiece := NewEntity(e.bornPos.X, e.bornPos.Y, math.Max(((e.radius*1.6)-4.0)-(float64(len(e.tail)+1)), 4.0), e.speed, "snektail")
			tailPiece.elements = []string{"lightning"}
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
	if !e.alive {
		return
	}
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
	duration float64
	velocity pixel.Vec

	width  float64
	length float64
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
	p := NewEntity(0.0, 0.0, 44, 575, "player")
	// p.wards = make([]ward, 0)
	p.elements = make([]string, 0)
	return p
}

func NewEssence(x float64, y float64, essence string, color color.Color, expiry time.Time) *entityData {
	e := NewEntity(x, y, 44.0, 0, "essence")
	e.expiry = expiry
	e.elements = []string{essence}
	e.color = color
	return e
}

func NewFollower(x float64, y float64) *entityData {
	e := NewEntity(x, y, 44.0, 280, "follower")
	e.spawnSound = spawnBuffer
	e.elements = []string{"water"}
	e.color = colornames.Cornflowerblue
	e.volume = -0.6
	e.bounty = 50
	e.movementColliderRadius = 24.0
	return e
}

func NewWanderer(x float64, y float64) *entityData {
	w := NewEntity(x, y, 44.0, 200, "wanderer")
	w.spawnSound = spawnBuffer4
	w.elements = []string{"lightning"}
	w.color = colornames.Mediumpurple
	w.volume = -0.4
	w.acceleration = 1
	w.bounty = 25
	return w
}

func NewDodger(x float64, y float64) *entityData {
	w := NewEntity(x, y, 44.0, 380, "dodger")
	w.elements = []string{"wind"}
	w.spawnSound = spawnBuffer2
	w.color = colornames.Orange
	w.acceleration = 2.0
	w.volume = -0.5
	w.friction = 0.95
	w.bounty = 100
	return w
}

func NewPinkSquare(x float64, y float64) *entityData {
	w := NewEntity(x, y, 44.0, 460, "pink")
	w.elements = []string{"chaos"}
	w.spawnSound = spawnBuffer5
	w.acceleration = 1.0
	w.volume = -0.2
	w.color = colornames.Crimson
	w.friction = 0.98
	w.bounty = 100
	return w
}

func NewPinkPleb(x float64, y float64) *entityData {
	w := NewEntity(x, y, 26.0, 256, "pinkpleb")
	// w.spawnSound = spawnBuffer4
	w.elements = []string{"chaos"}
	w.virtualOrigin = pixel.V(x, y)
	w.origin = w.virtualOrigin.Add(pixel.V(48.0, 0.0))
	w.color = colornames.Crimson
	w.spawnTime = 0.0
	w.spawning = false
	w.bounty = 75
	return w
}

func NewSnek(x float64, y float64) *entityData {
	s := NewEntity(x, y, 30.0, 280, "snek")
	s.elements = []string{"spirit"}
	s.spawnTime = 0.0
	s.spawning = false
	s.color = colornames.Azure
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
	w.elements = []string{"spirit"}
	w.spawnTime = 0.0
	w.spawning = false
	w.color = elementSpiritColor
	w.acceleration = 0.9
	w.speed = 600
	w.friction = 0.99
	w.bounty = 100
	return w
}

func NewBlackHole(x float64, y float64) *entityData {
	b := NewEntity(x, y, 40.0, 0.0, "blackhole")
	b.bounty = 150
	b.elements = []string{"fire"}
	b.color = elementFireColor
	b.spawnSound = spawnBuffer3
	b.volume = -0.6
	b.hp = 10
	b.active = false // dormant until activation (by taking damage)
	return b
}

func NewReplicator(x float64, y float64) *entityData {
	b := NewEntity(x, y, 20.0, 160.0, "replicator")
	b.elements = []string{"fire"}
	b.color = colornames.Orangered
	b.bounty = 50
	return b
}

func NewGate(x float64, y float64) *entityData {
	b := NewEntity(x, y, 212.0, 40.0, "gate")
	b.orientation = randomVector(1.0)
	b.bounty = 50
	return b
}

func NewBullet(x float64, y float64, width float64, length float64, speed float64, target pixel.Vec, elements []string, duration float64) *bullet {
	b := new(bullet)
	b.duration = duration
	b.data = *NewEntity(x, y, width, speed, "bullet")
	b.width = width
	b.length = length
	b.data.target = target
	b.data.elements = elements
	b.data.orientation = target
	b.velocity = target.Scaled(speed)
	return b
}

// Player

func SetDefaultPlayerSpeed(p *entityData) {
	p.speed = 575
	p.acceleration = 7.0
	p.friction = 0.875
}

func SetBoosting(p *entityData) {
	p.speed = 650
	p.acceleration = 2.0
	p.friction = 0.98
}

func (p *entityData) QueueElement(element string) {
	PlaySound("ward/spawn")
	if len(p.elements) < 2 {
		p.elements = append(p.elements, element)
	} else if p.elements[0] != element && p.elements[1] != element {
		p.elements[1] = p.elements[0]
		p.elements[0] = element
	} else {
		for i, el := range p.elements {
			if el != element {
				p.elements[i] = element
				break
			}
		}
	}
}
