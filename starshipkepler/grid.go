package starshipkepler

import "github.com/faiface/pixel"

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
