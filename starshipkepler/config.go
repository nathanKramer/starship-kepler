package starshipkepler

import (
	"image/color"

	"golang.org/x/image/colornames"
)

const worldWidth = 1700.0
const worldHeight = 1080.0

var g_debug = false

const particlesOn = true

const gameTitle = "Starship Kepler"

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
