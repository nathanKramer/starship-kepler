package starshipkepler

import "time"

type page struct {
	startTime time.Time
	lines     []string
}

type chapter struct {
	pages []page
}

var chapter1 = chapter{
	pages: []page{
		{lines: []string{"It's been so long..."}},
		{lines: []string{"Eons..."}},
		{lines: []string{"I've seen so much..."}},
		{lines: []string{
			"Great vistas of colour and light and time...",
			"Inconceivable abstractions, made real...",
			"Hulking, infinitesemal, fractal, eldritch, sublime...",
		}},
		{lines: []string{"What I would give to share this with someone."}},
	},
}
