package starshipkepler

import (
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"github.com/nathanKramer/starship-kepler/sliceextra"
	"golang.org/x/image/colornames"
)

func (game *game) timescale() float64 {
	return game.globalTimeScale * game.data.timescale
}

func GameOver(game *game) {
	PlaySound("game/over")
	game.state = "game_over"
	game.data.spawning = false
	game.localData.NewScore(ScoreEntry{
		Score: game.data.score,
		Name:  "Nathan",
		Time:  time.Now(),
	})

	game.localData.WriteToFile()
}

func UpdateGame(win *pixelgl.Window, game *game, ui *uiContext) {

	if game.lastFrame.Sub(game.lastMemCheck).Seconds() > 5.0 {
		// PrintMemUsage()
		// fmt.Printf("Entities\tlen: %d\tcap: %d\n", len(game.data.entities), cap(game.data.entities))
		// fmt.Printf("New Entities\tlen: %d\tcap: %d\n\n", len(game.data.newEntities), cap(game.data.newEntities))

		// fmt.Printf("Bullets\tlen: %d\tcap: %d\n", len(game.data.bullets), cap(game.data.bullets))
		// fmt.Printf("New Bullets\tlen: %d\tcap: %d\n\n", len(game.data.newBullets), cap(game.data.newBullets))

		// fmt.Printf("Particles\tlen: %d\tcap: %d\n", len(game.data.particles), cap(game.data.particles))
		// fmt.Printf("New Particles\tlen: %d\tcap: %d\n\n", len(game.data.newParticles), cap(game.data.newParticles))

		game.lastMemCheck = game.lastFrame

		// runtime.GC()
	}

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
	dt := math.Min(time.Since(game.lastFrame).Seconds(), 0.1) * game.timescale()
	game.totalTime += dt
	game.lastFrame = time.Now()

	if g_debug {
		game.debugInfos = []debugInfo{}
	}

	player := &game.data.player

	uiGamePadDir := pixel.Vec{}
	if win.JoystickPresent(ui.currJoystick) {
		moveVec := uiThumbstickVector(
			win,
			ui.currJoystick,
			pixelgl.AxisLeftX,
			pixelgl.AxisLeftY,
		)
		uiGamePadDir = moveVec
	}

	playerConfirmed := uiConfirm(win, ui.currJoystick)
	playerCancelled := uiCancel(win, ui.currJoystick)

	// lerp the camera position towards the player
	game.CamPos = pixel.Lerp(
		game.CamPos,
		player.origin.Scaled(0.75),
		1-math.Pow(1.0/128, dt),
	)

	if game.state == "main_menu" || game.state == "paused" {
		if playerConfirmed {
			PlaySound("menu/confirm")
			switch game.menu.options[game.menu.selection] {
			case "Development":
				game.state = "starting"
				game.data = *NewDevelopmentGame()

			case "Story Mode":
				game.state = "starting"
				game.data = *NewStoryGame()

			case "Quick Play: Evolved":
				game.state = "starting"
				game.data = *NewEvolvedGame()

			case "Quick Play: Pacifism (pre-alpha)":
				game.state = "starting"
				game.data = *NewPacifismGame()

			case "Options":
				game.menu = NewOptionsMenu()

			case "Fullscreen (1080p)":
				win.SetMonitor(pixelgl.PrimaryMonitor())

			case "Windowed (1024x768)":
				win.SetMonitor(nil)
				win.SetBounds(pixel.R(0, 0, 1024, 768))

			case "Quit":
				game.state = "quitting"

			case "Resume":
				game.state = "playing"

			case "Back":
				game.menu = NewMainMenu()

			case "Main Menu":
				game.state = "main_menu"
				game.menu = NewMainMenu()
				game.data = *NewMenuGame()
				game.PlayGameMusic()
				PlaySound("menu/confirm")

			case "Music On":
				game.music = true
				game.PlayGameMusic()
				PlaySound("menu/confirm")
			case "Music Off":
				game.music = false
				speaker.Clear()
				PlaySound("menu/confirm")
			}
		}

		implemented := 0
		for _, option := range game.menu.options {
			if sliceextra.Contains(implementedMenuItems, option) {
				implemented += 1
			}
		}

		menuChange := uiChangeSelection(win, uiGamePadDir, game.lastFrame, game.lastMenuChoiceTime)
		if menuChange != 0 {
			PlaySound("menu/step")
			game.menu.selection = (game.menu.selection + menuChange) % len(game.menu.options)
			if game.menu.selection < 0 {
				// would have thought modulo would handle negatives. /shrug
				game.menu.selection += len(game.menu.options)
			}
			for (implemented > 0) && !sliceextra.Contains(implementedMenuItems, game.menu.options[game.menu.selection]) {
				game.menu.selection = (game.menu.selection + menuChange) % len(game.menu.options)
				if game.menu.selection < 0 {
					// would have thought modulo would handle negatives. /shrug
					game.menu.selection += len(game.menu.options)
				}
			}
			game.lastMenuChoiceTime = time.Now()
		}
	}

	if game.state == "start_screen" {
		if playerConfirmed || playerCancelled {
			game.state = "main_menu"
			game.menu = NewMainMenu()
			game.data = *NewMenuGame()
			game.PlayGameMusic()
		}
	}

	if game.state == "starting" {
		switch game.data.mode {
		case "development":
			game.data = *NewDevelopmentGame()
		case "evolved":
			game.data = *NewEvolvedGame()
		case "pacifism":
			game.data = *NewPacifismGame()
		default:
			game.data = *NewStoryGame()
		}
		game.PlayGameMusic()
		PlaySound("menu/confirm")

		game.state = "playing"
	}

	if (game.state == "playing" && g_debug) && playerConfirmed {
		game.state = "starting"
	}
	if game.state == "game_over" {
		if playerConfirmed {
			game.state = "starting"
		} else if playerCancelled {
			game.state = "main_menu"
			game.menu = NewMainMenu()
			game.data = *NewMenuGame()
			game.PlayGameMusic()
		}
	}

	direction := pixel.ZV

	if game.state == "paused" {
		if playerCancelled {
			game.state = "playing"
		}
	} else if game.state == "playing" {
		if !player.alive {
			if game.lastFrame.Sub(player.death).Seconds() > 1.0 {
				game.respawnPlayer()
				game.grid.ApplyDirectedForce(Vector3{0.0, 0.0, 1400.0}, Vector3{player.origin.X, player.origin.Y, 0.0}, 80)
			}
		}

		if uiPause(win, ui.currJoystick) {
			game.state = "paused"
			game.menu = NewPauseMenu()
		}

		if win.JustPressed(pixelgl.KeyGraveAccent) {
			g_debug = !g_debug
			game.data.console = !game.data.console
		}

		// player controls
		if player.alive {
			if win.JustPressed(pixelgl.KeyMinus) {
				game.globalTimeScale *= 0.5
				if game.globalTimeScale < 0.1 {
					game.globalTimeScale = 0.0
				}
			}
			if win.JustPressed(pixelgl.KeyEqual) {
				game.globalTimeScale *= 2.0
				if game.globalTimeScale > 4.0 || game.globalTimeScale == 0.0 {
					game.globalTimeScale = 1.0
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

			if win.JoystickPresent(ui.currJoystick) {
				moveVec := uiThumbstickVector(
					win,
					ui.currJoystick,
					pixelgl.AxisLeftX,
					pixelgl.AxisLeftY,
				)
				direction = direction.Add(moveVec)
			}
		}

		// paste debug.go
		// spawn entities
		// This is a long procedure to allow spawning enemies for test purposes
		if g_debug {
			expiry := game.lastFrame.Add(time.Duration(10) * time.Second)
			if win.JustPressed(pixelgl.KeyQ) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"water",
					elementWaterColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}

			if win.JustPressed(pixelgl.KeyE) || win.JoystickJustPressed(ui.currJoystick, pixelgl.ButtonRightBumper) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"chaos",
					elementChaosColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}
			if win.JustPressed(pixelgl.KeyR) || win.JoystickJustPressed(ui.currJoystick, pixelgl.ButtonLeftBumper) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"spirit",
					elementSpiritColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}
			if win.JustPressed(pixelgl.KeyF) || win.JoystickJustPressed(ui.currJoystick, pixelgl.ButtonB) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"fire",
					elementFireColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}
			if win.JustPressed(pixelgl.KeyZ) || win.JoystickJustPressed(ui.currJoystick, pixelgl.ButtonA) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"lightning",
					elementLightningColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}
			if win.JustPressed(pixelgl.KeyX) || win.JoystickJustPressed(ui.currJoystick, pixelgl.ButtonY) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"wind",
					elementWindColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}
			if win.JustPressed(pixelgl.KeyC) {
				essence := *NewEssence(
					ui.MousePos.X,
					ui.MousePos.Y,
					"life",
					elementLifeColor,
					expiry,
				)
				game.data.newEntities = append(game.data.newEntities, essence)
			}

			if win.JustPressed(pixelgl.KeyG) {
				enemy := *NewReplicator(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeyH) {
				enemy := *NewSnek(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeyJ) {
				enemy := *NewWanderer(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeyK) {
				enemy := *NewFollower(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeyL) {
				enemy := *NewDodger(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeySemicolon) {
				enemy := *NewPinkSquare(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeyRightBracket) {
				enemy := *NewSnek(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}
			if win.JustPressed(pixelgl.KeyApostrophe) {
				enemy := *NewBlackHole(
					ui.MousePos.X,
					ui.MousePos.Y,
				)
				game.data.newEntities = append(game.data.newEntities, enemy)
			}

			total := 16.0
			step := 360.0 / total
			if win.JustPressed(pixelgl.KeyN) {
				for i := 0.0; i < total; i++ {
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					enemy := *NewWanderer(spawnPos.X, spawnPos.Y)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
			}
			if win.JustPressed(pixelgl.KeyM) {
				for i := 0.0; i < total; i++ {
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					enemy := *NewFollower(spawnPos.X, spawnPos.Y)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
			}
			if win.JustPressed(pixelgl.KeyComma) {
				for i := 0.0; i < total; i++ {
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					enemy := *NewDodger(spawnPos.X, spawnPos.Y)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
			}
			if win.JustPressed(pixelgl.KeyPeriod) {
				for i := 0.0; i < total; i++ {
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					enemy := *NewPinkSquare(spawnPos.X, spawnPos.Y)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
			}
			if win.JustPressed(pixelgl.KeySlash) {
				for i := 0.0; i < total; i++ {
					spawnPos := pixel.V(1.0, 0.0).Rotated(i * step * math.Pi / 180.0).Unit().Scaled(400.0 + (rand.Float64()*64 - 32.0)).Add(player.origin)
					enemy := *NewBlackHole(spawnPos.X, spawnPos.Y)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
			}

		}

	}

	// main game update
	if game.state == "playing" || game.state == "game_over" || game.data.mode == "menu" {
		if game.data.mode == "menu" {
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
			perpVel := pixel.V(baseVelocity.Y, -baseVelocity.X).Scaled(0.2 * math.Sin(game.totalTime*10))
			hue := math.Mod(((math.Mod(game.totalTime, 16.0) / 16.0) * 6.0), 6.0)
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

		player.Update(dt, game.totalTime, game.lastFrame)

		aim := player.origin.To(ui.MousePos)
		gamepadAim := uiThumbstickVector(win, ui.currJoystick, pixelgl.AxisRightX, pixelgl.AxisRightY)

		shooting := false
		if gamepadAim.Len() > 0.3 {
			shooting = true
			aim = gamepadAim
		}

		// if win.JustPressed(uiActionActSelf) || win.JustPressed(pixelgl.KeySpace) {
		// 	player.ReifyWard()
		// }

		timeSinceBullet := game.lastFrame.Sub(game.data.lastBullet).Milliseconds()
		timeSinceAbleToShoot := timeSinceBullet - int64(float64(game.data.weapon.fireRate)/game.timescale())

		if game.data.weapon != (weapondata{}) && timeSinceAbleToShoot >= 0 && game.data.player.alive {
			if game.data.mode == "menu" {
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
								game.data.weapon.bulletWidth,
								game.data.weapon.bulletLength,
								game.data.weapon.velocity,
								pixel.V(math.Cos(ang), math.Sin(ang)),
								append([]string{}, player.elements...),
								d,
								game.data.weapon.hp,
							),
						)
					}
				} else {
					FireBullet(aim, game, player.origin, player)
				}

				// Reflective bullets procedure.
				// Temporarily disabled.
				additionalBullets := make([]bullet, 0)
				if game.data.weapon.reflective > 0 {
					for i := 0; i < len(game.data.newBullets); i++ {
						b := game.data.newBullets[i]

						if !b.data.alive {
							continue
						}

						ang := 360 / float64(game.data.weapon.reflective+1)

						for j := 1; j <= game.data.weapon.reflective; j++ {
							reflectiveAngle := b.data.orientation.Angle() + (float64(j) * ang * math.Pi / 180)
							reflectiveAngleVec := pixel.V(math.Cos(reflectiveAngle), math.Sin(reflectiveAngle))
							firingPos := player.origin.Add(reflectiveAngleVec.Scaled(25.0))
							additionalBullets = append(
								additionalBullets,
								*NewBullet(
									firingPos.X,
									firingPos.Y,
									game.data.weapon.bulletWidth,
									game.data.weapon.bulletLength,
									game.data.weapon.velocity,
									reflectiveAngleVec,
									append([]string{}, player.elements...),
									game.data.weapon.duration,
									game.data.weapon.hp,
								),
							)
						}
					}
				}
				for _, b := range additionalBullets {
					game.data.newBullets = InlineAppendBullets(
						game.data.newBullets,
						b,
					)
				}

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

				game.data.lastBullet = game.lastFrame

				shotSound := "sound/shoot3.mp3"
				if game.data.weapon.bulletCount > 2 && game.data.weapon.conicAngle > 0 {
					shotSound = "sound/shoot-mixed.mp3"
				} else if game.data.weapon.conicAngle > 0 {
					shotSound = "sound/shoot2.mp3"
				} else if game.data.weapon.bulletCount > 3 {
					shotSound = "sound/shoot.mp3"
				}

				PlaySound(shotSound)
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
				if game.lastFrame.Sub(e.born).Seconds() >= e.spawnTime {
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

						if g_debug {
							game.debugInfos = append(game.debugInfos, debugInfo{p1: e.origin, p2: b.data.origin})
						}

						baseVelocity := entToBullet.Unit().Scaled(-4 * game.timescale())

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
				t := math.Mod(game.lastFrame.Sub(e.born).Seconds(), 2.0)
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

		// TODO: Need to make the bullet grid force effect better with lots of bullets in one place
		for i, b := range game.data.bullets {
			if !b.data.alive {
				game.data.bullets[i] = bullet{}
				continue
			}
			b.data.origin = b.data.origin.Add(b.velocity.Scaled(dt))
			if game.data.weapon.randomCone == 0 {
				// if game.data.weapon.bulletCount > 2 {
				if math.Mod(game.totalTime, 0.4) < 0.8 {
					game.grid.ApplyExplosiveForce(b.velocity.Scaled(dt).Len()*0.8, Vector3{b.data.origin.X, b.data.origin.Y, 0.0}, 60.0)
				}
				// } else {
				// 	game.grid.ApplyDirectedForce(
				// 		Vector3{b.velocity.Scaled(dt).X * 0.02, b.velocity.Scaled(dt).Y * 0.02, 0.0},
				// 		Vector3{b.data.origin.X, b.data.origin.Y, 0.0},
				// 		40.0,
				// 	)
				// }
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
			if (uint64(game.totalTime*1000)/125)%2 == 0 {
				v := 6.0 + (rand.Float64() * 12)
				sprayVelocity := pixel.V(
					math.Cos(b.particleEmissionAngle),
					math.Sin(b.particleEmissionAngle),
				).Unit().Scaled(v * game.timescale())

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

				if g_debug {
					game.debugInfos = append(game.debugInfos, debugInfo{
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

					if g_debug {
						game.debugInfos = append(game.debugInfos, debugInfo{
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

		for _, e := range game.data.entities {
			if e.entityType != "essence" || !e.alive {
				continue
			}

			game.grid.ApplyImplosiveForce(3.0, Vector3{e.origin.X, e.origin.Y, 0.0}, 50+e.radius)
		}

		game.grid.Update()

		// Apply velocities
		// player.origin = player.origin.Add(player.velocity.Scaled(dt))

		for i, e := range game.data.entities {
			if !e.alive && !e.spawning {
				continue
			}
			if g_debug && win.JustPressed(pixelgl.MouseButton1) {
				e.selected = e.Circle().Intersect(pixel.C(ui.MousePos, 4)).Radius > 0
				if e.entityType == "snek" {
					for tID, snekT := range e.tail {
						snekT.selected = snekT.Circle().Intersect(pixel.C(ui.MousePos, 4)).Radius > 0
						e.tail[tID] = snekT
					}
				}
			}

			// if e.entityType == "gate" {
			// 	if math.Mod(totalTime, 0.05) < 0.02 {
			// 		game.grid.ApplyExplosiveForce(5, Vector3{e.origin.X, e.origin.Y, 0.0}, 200)
			// 	}
			// }

			e.Update(dt, game.totalTime, game.lastFrame)
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
			if b.data.alive && (b.data.expiry == time.Time{} || b.data.expiry.After(game.lastFrame)) {
				for eID, e := range game.data.entities {
					if e.alive && !e.spawning && b.data.Circle().Intersect(e.Circle()).Radius > 0 && e.entityType != "essence" {
						bulletHp := b.data.hp
						b.DealDamage(
							&e,
							bID,
							e.hp,
							game.lastFrame,
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
							bulletHp,
							game.lastFrame,
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
					game.lastFrame,
				)
			}
		}

		if len(player.elements) > 0 {
			SetDefaultPlayerSpeed(player)
			game.data.weapon = *NewWeaponData()

			// inclusion checks
			elCounts := map[string]int{}
			for _, el := range player.elements {
				count, ok := elCounts[el]
				if ok {
					elCounts[el] = count + 1
				} else {
					elCounts[el] = 1
				}

				if el == "water" || el == "spirit" {
					game.data.weapon.bulletCount = 1
				} else if el == "fire" {
					game.data.weapon.bulletCount = 4
					game.data.weapon.duration = 0.25
					game.data.weapon.fireRate = 50
					game.data.weapon.randomCone = 12
				}
			}
			if elCounts["chaos"] == 2 {
				game.data.weapon.bulletCount = 1
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
					game.data.weapon.fireRate = int64(math.Max(float64(130), float64(game.data.weapon.fireRate-30)))
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
					game.data.weapon.velocity = game.data.weapon.velocity + 400
					game.data.weapon.bulletLength = game.data.weapon.bulletLength + 4
					game.data.weapon.conicAngle = game.data.weapon.conicAngle + 1
					game.data.weapon.hp = game.data.weapon.hp * 2 // make the bullets suuuuuuper tanky

					// takes precedence over fire
					game.data.weapon.duration = 5.0
					game.data.weapon.fireRate = int64(math.Max(float64(120), float64(game.data.weapon.fireRate-30)))
					if game.data.weapon.randomCone > 0 {
						game.data.weapon.randomCone = 0
						game.data.weapon.bulletCount = 2
					}
				} else if el == "lightning" {
					game.data.weapon.bulletWidth = game.data.weapon.bulletWidth / 1.4
					game.data.weapon.bulletLength = game.data.weapon.bulletLength * 1.4
					game.data.weapon.velocity = game.data.weapon.velocity + 150
					game.data.weapon.fireRate = game.data.weapon.fireRate - 30
					game.data.weapon.duration = game.data.weapon.duration + 0.125
				} else if el == "chaos" {
					game.data.weapon.reflective = game.data.weapon.reflective + 1
					game.data.weapon.conicAngle = game.data.weapon.conicAngle + 2
					game.data.weapon.fireRate = game.data.weapon.fireRate - 20
					game.data.weapon.duration = game.data.weapon.duration + 0.125
				}
			}
			if game.data.weapon.randomCone > 0 {
				game.data.weapon.randomCone = game.data.weapon.randomCone + game.data.weapon.conicAngle
				game.data.weapon.conicAngle = 0
			}

			// if win.JustPressed(uiActionAct) {
			// 	// possible ways this could work:
			// 	// ward elements could have passive effects, OR not.
			// 	// I'm toying around with passive effects, and activation effects (which destroy the ward, thus removing the passive effects)
			// 	// two kinds of activations: internal (probably defensive) and external (probably offensive)
			// 	//end
			// 	game.data.weapon = *NewWeaponData()
			// 	player.elements = make([]string, 2)
			// }
		}

		// check for bomb here for now
		bombPressed := win.Pressed(pixelgl.KeySpace) || win.JoystickAxis(ui.currJoystick, pixelgl.AxisRightTrigger) > 0.1
		// game.data.bombs > 0 &&
		// droppping bomb concept for the moment
		if len(player.elements) > 0 && bombPressed && game.lastFrame.Sub(game.data.lastBomb).Seconds() > 3.0 {
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
				e.death = game.lastFrame
				e.expiry = game.lastFrame
				game.data.entities[eID] = e
			}

			player.elements = make([]string, 0)
		}

		// Keep buffered particles ticking so they don't stack up too much
		for pID, p := range game.data.newParticles {
			p.percentLife -= 1.0 / p.duration * game.timescale()
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

				p.percentLife -= 1.0 / p.duration * game.timescale()

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
			died := (!existing.alive && existing.born != time.Time{})
			expired := (existing.expiry != time.Time{}) && game.lastFrame.After(existing.expiry)
			diedWithNoExpiry := died && existing.expiry == (time.Time{})
			if diedWithNoExpiry || expired {
				game.data.entities[entID] = entityData{}
				killedEnt++
			} // kill entities
		}
		// if killedEnt > 0 {
		// 	fmt.Printf("Killed\t(%d entities)\n", killedEnt)
		// }

		particleID := 0
		for addedID, newParticle := range game.data.newParticles {
			if newParticle == (particle{}) {
				continue
			}

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

		entID := 0
		toSpawn := 0
		spawnedEnt := 0
		for addedID, newEnt := range game.data.newEntities {
			if newEnt.entityType == "" {
				continue
			}

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
		} // bring in new entities
		// fmt.Printf("Killed: %d	To Spawn: %d	Spawned: %d\n", killedEnt, toSpawn, spawnedEnt)

		// kill bullets
		for bID, b := range game.data.bullets {
			if !b.data.alive && time.Now().After(b.data.born.Add(time.Duration(b.duration*1000)*time.Millisecond)) {
				game.data.bullets[bID] = bullet{}
			}
		}

		if game.data.mode == "evolved" || game.data.mode == "menu" {
			game.evolvedGameModeUpdate(g_debug, game.lastFrame, game.totalTime, player)
		} else if game.data.mode == "development" {
			game.developmentGameModeUpdate(g_debug, game.lastFrame, game.totalTime, player)
		} else if game.data.mode == "pacifism" {
			game.pacifismGameModeUpdate(g_debug, game.lastFrame, game.totalTime, player)
		}
	}

}
