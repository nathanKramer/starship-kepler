package main

import (
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

var shotBuffer *beep.Buffer
var spawnBuffer *beep.Buffer
var spawnBuffer2 *beep.Buffer
var spawnBuffer3 *beep.Buffer
var spawnBuffer4 *beep.Buffer
var spawnBuffer5 *beep.Buffer
var lifeBuffer *beep.Buffer
var multiplierBuffer *beep.Buffer
var bombBuffer *beep.Buffer
var musicStreamer *beep.StreamSeekCloser

func prepareStreamer(file string) (*beep.StreamSeekCloser, *beep.Format) {
	sound, _ := os.Open(file)
	streamer, format, err := mp3.Decode(sound)
	if err != nil {
		panic(err)
	}

	return &streamer, &format
}

func prepareBuffer(file string) *beep.Buffer {
	sound, _ := os.Open(file)
	streamer, format, err := mp3.Decode(sound)
	if err != nil {
		panic(err)
	}
	buffer := beep.NewBuffer(format)
	buffer.Append(streamer)
	streamer.Close()

	return buffer
}

func init() {
	// todo use a data structure probably
	shotBuffer = prepareBuffer("sound/shoot.mp3")

	spawnBuffer = prepareBuffer("sound/spawn.mp3")
	spawnBuffer2 = prepareBuffer("sound/spawn2.mp3")
	spawnBuffer3 = prepareBuffer("sound/spawn3.mp3")
	spawnBuffer4 = prepareBuffer("sound/spawn4.mp3")
	spawnBuffer5 = prepareBuffer("sound/spawn5.mp3")

	lifeBuffer = prepareBuffer("sound/life.mp3")
	multiplierBuffer = prepareBuffer("sound/multiplierbonus.mp3")
	bombBuffer = prepareBuffer("sound/usebomb.mp3")

	var musicFormat *beep.Format
	musicStreamer, musicFormat = prepareStreamer("sound/music.mp3")
	speaker.Init(musicFormat.SampleRate, musicFormat.SampleRate.N(time.Second/10))
}

func playMusic() {
	s := *musicStreamer
	speaker.Play(s)
	// defer s.Close()
}
