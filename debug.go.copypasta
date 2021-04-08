			// spawn entities
			// This is a long procedure to allow spawning enemies for test purposes
			if debug {
				if win.JustPressed(pixelgl.KeyG) {
					enemy := *NewReplicator(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeyH) {
					enemy := *NewSnek(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeyJ) {
					enemy := *NewWanderer(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeyK) {
					enemy := *NewFollower(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeyL) {
					enemy := *NewDodger(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeySemicolon) {
					enemy := *NewPinkSquare(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeyRightBracket) {
					enemy := *NewSnek(
						mp.X,
						mp.Y,
					)
					game.data.newEntities = append(game.data.newEntities, enemy)
				}
				if win.JustPressed(pixelgl.KeyApostrophe) {
					enemy := *NewBlackHole(
						mp.X,
						mp.Y,
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
