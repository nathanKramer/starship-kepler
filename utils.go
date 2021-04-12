package main

import (
	"math"

	"github.com/faiface/pixel"
)

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
