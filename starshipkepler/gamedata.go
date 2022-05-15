package starshipkepler

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/pixel"
)

const maxParticles = 5000

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
	fireRate    int64 // milliseconds
	conicAngle  float64
	randomCone  float64
	bulletCount int
	velocity    float64
	reflective  int
	duration    float64

	bulletWidth  float64
	bulletLength float64
}

type gamedata struct {
	camPos pixel.Vec

	mode            string
	lives           int
	bombs           int
	scoreMultiplier int

	entities     []entityData
	bullets      []bullet
	particles    []particle
	newEntities  []entityData
	newParticles []particle
	newBullets   []bullet

	spawns         int
	spawnCount     int
	pendingSpawns  int
	scoreSinceBorn int
	killsSinceBorn int
	player         entityData
	weapon         weapondata
	waves          []wavedata

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

	console bool
}

type debugInfo struct {
	p1   pixel.Vec
	p2   pixel.Vec
	text string
}

type game struct {
	state string
	data  gamedata
	menu  menu
	grid  grid

	CamPos pixel.Vec

	// Frame state
	lastFrame          time.Time
	lastMemCheck       time.Time
	lastMenuChoiceTime time.Time
	totalTime          float64
	debugInfos         []debugInfo
	globalTimeScale    float64

	music bool
}

// Menus
var implementedMenuItems = []string{
	"Development",
	"Quick Play: Evolved",
	"Quick Play: Pacifism",
	"Options",
	"Quit",
	"Resume",
	"Main Menu",
	"Music On",
	"Music Off",
	"Fullscreen (1080p)",
	"Windowed (1024x768)",
	"Back",
}

func NewMainMenu() menu {
	return menu{
		selection: 0,
		options: []string{
			"Development",
			"Story Mode",
			"Quick Play: Evolved",
			"Quick Play: Pacifism",
			"Leaderboard",
			"Achievements",
			"Options",
			"Quit",
		},
	}
}

func NewPauseMenu() menu {
	return menu{
		selection: 0,
		options: []string{
			"Resume",
			"Main Menu",
		},
	}
}

func NewOptionsMenu() menu {
	return menu{
		selection: 0,
		options: []string{
			"Fullscreen (1080p)",
			"Windowed (1024x768)",
			"Music Off",
			"Music On",
			"Back",
		},
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
	weaponData.velocity = 1100
	weaponData.fireRate = 150
	weaponData.bulletCount = 2
	weaponData.bulletWidth = 8
	weaponData.bulletLength = 20
	weaponData.conicAngle = 0
	weaponData.randomCone = 0
	weaponData.duration = 5.0

	return weaponData
}

func NewBurstWeapon() *weapondata {
	weaponData := new(weapondata)
	weaponData.velocity = 1100
	weaponData.fireRate = 150
	weaponData.bulletCount = 5
	weaponData.bulletWidth = 8
	weaponData.bulletLength = 20
	weaponData.conicAngle = 0
	weaponData.randomCone = 0

	return weaponData
}

func NewConicWeapon() *weapondata {
	weaponData := new(weapondata)
	weaponData.velocity = 1400
	weaponData.fireRate = 100
	weaponData.bulletWidth = 8
	weaponData.bulletLength = 20
	weaponData.conicAngle = 8
	weaponData.randomCone = 0

	return weaponData
}

func NewGameData() *gamedata {
	gameData := new(gamedata)

	gameData.mode = "none"
	gameData.lives = 500
	gameData.bombs = 3
	gameData.scoreMultiplier = 1

	gameData.entities = make([]entityData, 0, 200)
	gameData.bullets = make([]bullet, 0, 500)
	gameData.particles = make([]particle, 0, maxParticles)
	gameData.newEntities = make([]entityData, 0, 200)
	gameData.newBullets = make([]bullet, 0, 500)
	gameData.newParticles = make([]particle, 0, 1000)

	gameData.player = *NewPlayer(0.0, 0.0)
	gameData.spawns = 0
	gameData.spawnCount = 1
	gameData.pendingSpawns = 0
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

	gameData.console = false

	return gameData
}

func NewMenuGame() *gamedata {
	data := NewGameData()
	data.mode = "menu"
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
	data.waveFreq = 5 // waves have a duration so can influence the pace of the game
	data.weaponUpgradeFreq = 30
	data.landingPartyFreq = 10 // more strategic one-off spawn systems
	data.ambientSpawnFreq = 3  // ambient spawning can be toggled off temporarily, but is otherwise always going on
	return data
}

func NewDevelopmentGame() *gamedata {
	data := NewGameData()
	data.mode = "development"
	data.weapon = *NewWeaponData()
	data.lives = 100
	data.multiplierReward = 25 // kills
	data.lifeReward = 75000
	data.bombReward = 100000
	data.waveFreq = 5 // waves have a duration so can influence the pace of the game
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
	game.data = *NewMenuGame()
	game.menu = NewMainMenu()
	game.CamPos = pixel.ZV
	game.lastFrame = time.Now()
	game.lastMenuChoiceTime = time.Now()
	game.lastMemCheck = time.Now()

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

	game.totalTime = 0.0
	game.debugInfos = []debugInfo{}
	game.globalTimeScale = 1.0

	// TODO: Put user options in a sensible place
	game.music = true

	return game
}

func FireBullet(aim pixel.Vec, game *game, origin pixel.Vec, player *entityData) {
	width := float64(game.data.weapon.bulletCount) * 5.0
	rad := math.Atan2(aim.Unit().Y, aim.Unit().X)
	for i := 0; i < game.data.weapon.bulletCount; i++ {
		bPos := pixel.V(
			25.0,
			-(width/2)+(float64(i)*(width/float64(game.data.weapon.bulletCount))),
		).Rotated(
			rad,
		).Add(origin)

		increment := (float64(i) * math.Pi / 180.0) - (2 * math.Pi / 180)
		b := NewBullet(
			bPos.X,
			bPos.Y,
			game.data.weapon.bulletWidth,
			game.data.weapon.bulletLength,
			game.data.weapon.velocity,
			aim.Unit().Rotated(increment),
			append([]string{}, player.elements...),
			game.data.weapon.duration,
		)
		game.data.newBullets = InlineAppendBullets(game.data.newBullets, *b)
	}
}

func (data *gamedata) respawnPlayer() {
	for entID, _ := range data.newEntities {
		data.newEntities[entID] = entityData{}
	}
	for entID, _ := range data.entities {
		data.entities[entID] = entityData{}
	}
	for bullID, _ := range data.bullets {
		data.bullets[bullID] = bullet{}
	}
	data.player = *NewPlayer(0.0, 0.0)
	data.weapon = *NewWeaponData()
	data.scoreMultiplier = 1
	data.scoreSinceBorn = 0
	data.killsSinceBorn = 0
}

func (data *gamedata) AmbientSpawnFreq() float64 {
	return data.ambientSpawnFreq * data.timescale
}

func (data *gamedata) WaveFreq() float64 {
	return data.waveFreq * data.timescale
}

func (game *game) PlayGameMusic() {
	if game.music {
		PlaySong(game.data.mode)
	}
}

func (game *game) developmentGameModeUpdate(debug bool, last time.Time, totalTime float64, player *entityData) {

}

func (game *game) evolvedGameModeUpdate(debug bool, last time.Time, totalTime float64, player *entityData) {
	updateMusic(game.data.mode)
	// ambient spawns
	// This spawns between 1 and 4 enemies every AmbientSpawnFreq seconds
	if last.Sub(game.data.lastSpawn).Seconds() > game.data.AmbientSpawnFreq() && game.data.spawning {
		// spawn
		spawns := make([]entityData, 0, game.data.spawnCount)
		for i := 0; i < game.data.spawnCount; i++ {
			pos := pixel.V(
				float64(rand.Intn(worldWidth)-worldWidth/2),
				float64(rand.Intn(worldHeight)-worldHeight/2),
			)
			// to regulate distance from player
			for pos.Sub(player.origin).Len() < 450 {
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

		PlaySpawnSounds(spawns)
		game.data.newEntities = InlineAppendEntities(game.data.newEntities, spawns...)
		game.data.spawns += len(spawns)
		game.data.lastSpawn = time.Now()

		game.data.spawnCount = 1
		n := int(math.Min(float64(game.data.spawns/50), 4))
		if n > game.data.spawnCount {
			game.data.spawnCount = n
		}
	}

	game.data.notoriety = float64(game.data.kills) / 100.0

	livingEntities := 0
	for _, e := range game.data.entities {
		if e.alive && e.entityType != "essence" {
			livingEntities++
		}
	}

	// wave management
	waveDead := livingEntities == 0 && (game.data.pendingSpawns == 0 && !game.data.spawning)
	firstWave := game.data.lastWave == (time.Time{}) && totalTime >= 2
	subsequentWave := (game.data.lastWave != (time.Time{}) &&
		(last.Sub(game.data.lastWave).Seconds() >= game.data.WaveFreq()) || waveDead)

	// spawnCount + wave N
	spawns := make([]entityData, 0, 200)

	// waves happen every waveFreq seconds
	if firstWave || subsequentWave {
		// New wave, so re-assess wave frequency etc in case it was set by a custom wave
		game.data.ambientSpawnFreq = math.Max(
			1.0,
			3-((float64(game.data.spawns)/30.0)*0.5),
		)
		game.data.waveFreq = math.Max(
			5.0, 20.0-(3*game.data.notoriety),
		)

		game.data.spawning = false
		corners := [4]pixel.Vec{
			pixel.V(-(worldWidth/2)+80, -(worldHeight/2)+80),
			pixel.V(-(worldWidth/2)+80, (worldHeight/2)-80),
			pixel.V((worldWidth/2)-80, -(worldHeight/2)+80),
			pixel.V((worldWidth/2)-80, (worldHeight/2)-80),
		}

		if (rand.Float64() * (0.1 + math.Min(game.data.notoriety, 0.8))) > 0.5 {
			game.data.spawning = true
		}

		// one-off landing party
		r := rand.Float64() * (0.1 + math.Min(game.data.notoriety, 0.8))
		fmt.Printf("[LandingPartySpawn] %f %s\n", r, time.Now().String())
		// landing party spawn
		{
			if r <= 0.1 {
				// if we roll 0.1, just do a wave of ambient spawning
				game.data.waveFreq = 5
				game.data.spawning = true
			} else if r <= 0.25 {
				count := 2 + rand.Intn(4)
				for i := 0; i < count; i++ {
					p := corners[i%4]
					enemy := NewFollower(
						p.X,
						p.Y,
					)
					spawns = append(spawns, *enemy)
				}
			} else if r <= 0.3 {
				count := 2 + rand.Intn(4)
				for i := 0; i < count; i++ {
					p := corners[i%4]
					enemy := NewPinkSquare(
						p.X,
						p.Y,
					)
					spawns = append(spawns, *enemy)
				}
			} else if r <= 0.35 {
				count := 2 + rand.Intn(4)
				for i := 0; i < count; i++ {
					p := corners[i%4]
					enemy := NewDodger(
						p.X,
						p.Y,
					)
					spawns = append(spawns, *enemy)
				}
			} else if r <= 0.45 {
				count := 8 + rand.Intn(4)
				for i := 0; i < count; i++ {
					p := corners[i%4]
					enemy := NewSnek(
						p.X,
						p.Y,
					)
					spawns = append(spawns, *enemy)
				}
			} else if r <= 0.55 {
				r := rand.Float64() * (0.1 + math.Min(game.data.notoriety, 0.8))
				var t string
				var freq float64
				var duration float64
				if r <= 0.15 {
					t = "wanderer"
					freq = 0.3
					duration = 3.0
				} else if r <= 0.3 {
					t = "follower"
					freq = 0.5
					duration = 10.0
				} else if r <= 0.5 {
					t = "dodger"
					freq = 0.25
					duration = 3.0
				} else if r <= 0.6 {
					t = "pink"
					freq = 0.2
					duration = 1.0
				} else if r <= 0.6 {
					t = "follower"
					freq = 0.25
					duration = 5.0
				} else {
					t = "replicator"
					freq = 0.2
					duration = 5.0
				}

				fmt.Printf("[Wave]: Creating a wave %s, %f, %f\n", t, freq, duration)

				game.data.waves = append(game.data.waves, *NewWaveData(t, freq, duration))
				game.data.lastWave = last
			} else if r <= 0.55 {
				total := 8.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // circle of followers
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(450.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawns = append(
						spawns,
						*NewFollower(spawnPos.X, spawnPos.Y),
						*NewFollower(spawnPos2.X, spawnPos2.Y),
					)
				}
			} else if r <= 0.575 {
				total := 8.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // circle of dodgers
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(450.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawns = append(
						spawns,
						*NewDodger(spawnPos.X, spawnPos.Y),
						*NewDodger(spawnPos2.X, spawnPos2.Y),
					)
				}
			} else if r <= 0.6 {
				total := 5.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // circle of replicators
					for j := 0; j < 16; j++ {
						spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0).Add(randomVector(16.0)).Add(player.origin)
						spawns = append(
							spawns,
							*NewReplicator(spawnPos.X, spawnPos.Y),
						)
					}
				}
			} else if r <= 0.64 {
				total := 10.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // circle of snakes
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(550.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawns = append(
						spawns,
						*NewSnek(spawnPos.X, spawnPos.Y),
						*NewSnek(spawnPos2.X, spawnPos2.Y),
					)
				}
			} else if r <= 0.67 {
				total := 4.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // circle of pink squares
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(450.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawns = append(
						spawns,
						*NewPinkSquare(spawnPos.X, spawnPos.Y),
						*NewPinkSquare(spawnPos2.X, spawnPos2.Y),
					)
				}
			} else if r <= 0.75 {
				game.data.spawning = false       // corner spawns will feel like a bit of a breather
				for _, corner := range corners { // corner spawns
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
					spawns = append(
						spawns, *blackhole, *snek, *pink, *dodger,
					)
				}
			} else if r <= 0.85 {
				total := 4.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // circle of blackholes
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawns = append(
						spawns,
						*NewBlackHole(spawnPos.X, spawnPos.Y),
					)
				}
			} else {
				total := 14.0
				step := 360.0 / total
				for i := 0.0; i < total; i++ { // big circle of followers
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(500.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawnPos2 := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(650.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					spawns = append(
						spawns,
						*NewFollower(spawnPos.X, spawnPos.Y),
						*NewFollower(spawnPos2.X, spawnPos2.Y),
					)
				}
			}
		}

		game.data.lastWave = last
	}

	for waveID, wave := range game.data.waves {
		if (wave.waveStart == time.Time{}) {
			// fmt.Printf("[WaveSkip] %s, %f, %f", wave.entityType, wave.waveDuration, last.Sub(wave.waveStart).Seconds())
			continue
		}
		if last.Sub(wave.waveStart).Seconds() >= wave.waveDuration { // If a wave has ended
			// End the wave
			fmt.Printf("[WaveEnd] %s\n", time.Now().String())
			wave.waveEnd = time.Now()
			wave.waveStart = time.Time{}
			game.data.waves[waveID] = wave
			continue
		}
		if last.Sub(wave.waveStart).Seconds() < wave.waveDuration {
			// Continue wave
			// TODO make these data driven
			// waves would have spawn points, and spawn counts, and probably durations and stuff
			// hardcoded for now :D
			// fmt.Printf("[WaveTick] %s %s\n", wave.entityType, time.Now().String())

			if last.Sub(wave.lastSpawn).Seconds() > wave.spawnFreq {
				// 4 spawn points
				points := [4]pixel.Vec{
					pixel.V(-(worldWidth/2)+32, -(worldHeight/2)+32),
					pixel.V(-(worldWidth/2)+32, (worldHeight/2)-32),
					pixel.V((worldWidth/2)-32, -(worldHeight/2)+32),
					pixel.V((worldWidth/2)-32, (worldHeight/2)-32),
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
					} else if wave.entityType == "wanderer" {
						enemy = NewWanderer(
							p.X,
							p.Y,
						)
					} else {
						panic(fmt.Errorf("Unhandled entity type: %s", wave.entityType))
					}
					spawns = append(spawns, *enemy)
				}

				wave.lastSpawn = time.Now()
				game.data.waves[waveID] = wave
			}
		}
	}

	// spawn entitites
	PlaySpawnSounds(spawns)
	game.data.spawns += len(spawns)
	game.data.newEntities = InlineAppendEntities(game.data.newEntities, spawns...)

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

	// weapon upgrading doesn't seem relevant anymore
	// timeToUpgrade := game.data.score >= 10000 && game.data.lastWeaponUpgrade == time.Time{}
	// if timeToUpgrade || (game.data.lastWeaponUpgrade != time.Time{} && last.Sub(game.data.lastWeaponUpgrade).Seconds() >= game.data.weaponUpgradeFreq) {
	// 	fmt.Printf("[UpgradingWeapon]\n")
	// 	game.data.lastWeaponUpgrade = time.Now()
	// 	switch rand.Intn(2) {
	// 	case 0:
	// 		game.data.weapon = *NewBurstWeapon()
	// 	case 1:
	// 		game.data.weapon = *NewConicWeapon()
	// 	}
	// }

}

func (game *game) pacifismGameModeUpdate(debug bool, last time.Time, totalTime float64, player *entityData) {
	// ambient spawns
	if last.Sub(game.data.lastSpawn).Seconds() > game.data.AmbientSpawnFreq() && game.data.spawning {
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

		PlaySpawnSounds(spawns)
		game.data.newEntities = InlineAppendEntities(game.data.newEntities, spawns...)
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
