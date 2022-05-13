package starshipkepler

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

var soundFormat *beep.Format
var shotSoundFormat *beep.Format
var shotBuffer *beep.Buffer
var shotBuffer2 *beep.Buffer
var shotBuffer3 *beep.Buffer
var shotBuffer4 *beep.Buffer
var spawnBuffer *beep.Buffer
var spawnBuffer2 *beep.Buffer
var spawnBuffer3 *beep.Buffer
var spawnBuffer4 *beep.Buffer
var spawnBuffer5 *beep.Buffer
var pinkSquareSpawnBuffer *beep.Buffer
var snakeSpawnBuffer *beep.Buffer
var blackholeHitBuffer *beep.Buffer
var blackholeDieBuffer *beep.Buffer
var lifeBuffer *beep.Buffer
var multiplierBuffer *beep.Buffer
var multiplierBuffer2 *beep.Buffer
var multiplierBuffer3 *beep.Buffer
var multiplierBuffer4 *beep.Buffer
var multiplierBuffer5 *beep.Buffer
var multiplierBuffer6 *beep.Buffer
var multiplierBuffer7 *beep.Buffer
var multiplierBuffer8 *beep.Buffer
var multiplierBuffer9 *beep.Buffer
var multiplierBuffer10 *beep.Buffer

var multiplierSounds map[int]*beep.Buffer

var bombBuffer *beep.Buffer
var musicStreamer *beep.StreamSeekCloser
var pacifismMusicStreamer *beep.StreamSeekCloser
var menuMusicStreamer *beep.StreamSeekCloser
var introStreamer *beep.StreamSeekCloser

type soundEffect struct {
	buffer *beep.Buffer
	volume float64
}

var musicStreamers = map[string]beep.StreamSeekCloser{}
var soundEffects = map[string]*soundEffect{}

func prepareStreamer(file string) (*beep.StreamSeekCloser, *beep.Format) {
	sound, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	streamer, format, err := mp3.Decode(sound)
	if err != nil {
		panic(err)
	}

	return &streamer, &format
}

func prepareBuffer(file string) (*beep.Buffer, *beep.Format) {
	sound, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	ext := strings.Split(file, ".")[1]

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case "mp3":
		streamer, format, err = mp3.Decode(sound)
	case "wav":
		streamer, format, err = wav.Decode(sound)
	default:
		errorString := fmt.Sprintf("Unsupported file extension: %s", ext)
		panic(errorString)
	}

	if err != nil {
		panic(err)
	}
	buffer := beep.NewBuffer(format)
	buffer.Append(streamer)
	streamer.Close()

	return buffer, &format
}

func init() {
	// TODO:
	// Unify how sounds are played, and make them driven by configuration

	spawnBuffer2, _ = prepareBuffer("sound/spawn2.mp3")
	spawnBuffer3, _ = prepareBuffer("sound/spawn3.mp3")
	spawnBuffer4, _ = prepareBuffer("sound/spawn4.mp3")
	spawnBuffer5, _ = prepareBuffer("sound/spawn5.mp3")
	snakeSpawnBuffer, _ = prepareBuffer("sound/snake-spawn.mp3")

	pinkSquareSpawnBuffer, _ = prepareBuffer("sound/snake-spawn.mp3")
	blackholeHitBuffer, _ = prepareBuffer("sound/blackhole-hit.mp3")
	blackholeDieBuffer, _ = prepareBuffer("sound/blackhole-die.mp3")

	lifeBuffer, _ = prepareBuffer("sound/player-life.mp3")
	multiplierBuffer, _ = prepareBuffer("sound/multiplierbonus.mp3")
	multiplierBuffer2, _ = prepareBuffer("sound/multiplierbonus2.mp3")
	multiplierBuffer3, _ = prepareBuffer("sound/multiplierbonus3.mp3")
	multiplierBuffer4, _ = prepareBuffer("sound/multiplierbonus4.mp3")
	multiplierBuffer5, _ = prepareBuffer("sound/multiplierbonus5.mp3")
	multiplierBuffer6, _ = prepareBuffer("sound/multiplierbonus6.mp3")
	multiplierBuffer7, _ = prepareBuffer("sound/multiplierbonus7.mp3")
	multiplierBuffer8, _ = prepareBuffer("sound/multiplierbonus8.mp3")
	multiplierBuffer9, _ = prepareBuffer("sound/multiplierbonus9.mp3")
	multiplierBuffer10, _ = prepareBuffer("sound/multiplierbonus10.mp3")
	multiplierSounds = map[int]*beep.Buffer{}
	multiplierSounds[2] = multiplierBuffer2
	multiplierSounds[3] = multiplierBuffer3
	multiplierSounds[4] = multiplierBuffer4
	multiplierSounds[5] = multiplierBuffer5
	multiplierSounds[6] = multiplierBuffer6
	multiplierSounds[7] = multiplierBuffer7
	multiplierSounds[8] = multiplierBuffer8
	multiplierSounds[9] = multiplierBuffer9
	multiplierSounds[10] = multiplierBuffer10

	bombBuffer, _ = prepareBuffer("sound/player-bomb.mp3")

	initSounds()
	initMusic()

	speaker.Init(soundFormat.SampleRate, soundFormat.SampleRate.N(time.Second/10))
}

func initSounds() {
	shotBuffer, shotSoundFormat = prepareBuffer("sound/shoot.mp3")
	soundEffects["sound/shoot.mp3"] = &soundEffect{
		buffer: shotBuffer,
		volume: -0.9,
	}

	shotBuffer2, shotSoundFormat = prepareBuffer("sound/shoot2.mp3")
	soundEffects["sound/shoot2.mp3"] = &soundEffect{
		buffer: shotBuffer2,
		volume: -0.7,
	}

	shotBuffer3, shotSoundFormat = prepareBuffer("sound/shoot3.mp3")
	shotBuffer3.Streamer(0, shotBuffer3.Len())
	soundEffects["sound/shoot3.mp3"] = &soundEffect{
		buffer: shotBuffer3,
		volume: -1.2,
	}

	shotBuffer4, shotSoundFormat = prepareBuffer("sound/shoot4.mp3")
	soundEffects["sound/shoot-mixed.mp3"] = &soundEffect{
		buffer: shotBuffer4,
		volume: -0.9,
	}

	spawnBuffer, _ = prepareBuffer("sound/menu-step.wav")
	soundEffects["menu/step"] = &soundEffect{
		buffer: spawnBuffer,
		volume: -0.9,
	}

	spawnBuffer, _ = prepareBuffer("sound/menu-confirm.wav")
	soundEffects["menu/confirm"] = &soundEffect{
		buffer: spawnBuffer,
		volume: -0.9,
	}

	buffer, _ := prepareBuffer("sound/player-die.wav")
	soundEffects["player/die"] = &soundEffect{
		buffer: buffer,
		volume: -0.8,
	}

	buffer, _ = prepareBuffer("sound/game-over.wav")
	soundEffects["game/over"] = &soundEffect{
		buffer: buffer,
		volume: -1.0,
	}

	buffer, _ = prepareBuffer("sound/ward-spawn.wav")
	soundEffects["ward/spawn"] = &soundEffect{
		buffer: buffer,
		volume: -0.8,
	}

	buffer, _ = prepareBuffer("sound/ward-die.wav")
	soundEffects["ward/die"] = &soundEffect{
		buffer: buffer,
		volume: -0.8,
	}

	buffer, _ = prepareBuffer("sound/entity-die.wav")
	soundEffects["entity/die"] = &soundEffect{
		buffer: buffer,
		volume: -1.3,
	}

	spawnBuffer, _ = prepareBuffer("sound/spawn.mp3")
	soundEffects["sound/spawn.mp3"] = &soundEffect{
		buffer: spawnBuffer,
		volume: -0.9,
	}
}

func initMusic() {
	musicStreamer, _ := prepareStreamer("sound/music-evolved.mp3")
	musicStreamers["evolved"] = *musicStreamer

	musicStreamer, _ = prepareStreamer("sound/music-pacifism.mp3")
	musicStreamers["pacifism"] = *musicStreamer

	musicStreamer, _ = prepareStreamer("sound/music-menu.mp3")
	musicStreamers["menu"] = *musicStreamer

	musicStreamer, soundFormat = prepareStreamer("sound/music-intro.mp3")
	musicStreamers["intro"] = *musicStreamer
}

func updateMusic(songName string) {
	if musicStreamers[songName].Position() == musicStreamers[songName].Len() {
		PlaySong(songName)
	}
}

func PlaySong(songName string) {
	speaker.Clear()
	s, ok := musicStreamers[songName]
	if !ok {
		errorString := fmt.Sprintf("Unknown sound: %s", songName)
		panic(errorString)
	}

	s.Seek(0)
	speaker.Play(s)
}

func PlaySound(soundName string) {
	soundEffect, ok := soundEffects[soundName]
	if !ok {
		errorString := fmt.Sprintf("Unknown sound: %s", soundName)
		panic(errorString)
	}

	sound := soundEffect.buffer.Streamer(0, soundEffect.buffer.Len())

	volume := &effects.Volume{
		Streamer: sound,
		Base:     10,
		Volume:   soundEffect.volume,
		Silent:   false,
	}

	// fmt.Printf("[SoundPlayer] %s\n", soundName)
	speaker.Play(volume)
}
