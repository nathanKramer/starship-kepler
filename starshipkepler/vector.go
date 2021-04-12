package starshipkepler

import (
	"math"
	"math/rand"

	"github.com/faiface/pixel"
)

func randomVector(magnitude float64) pixel.Vec {
	return pixel.V(rand.Float64()-0.5, rand.Float64()-0.5).Unit().Scaled(magnitude)
}

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
