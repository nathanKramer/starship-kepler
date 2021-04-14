package starshipkepler

import (
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
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

var soundEffects = map[string]*soundEffect{}

func prepareStreamer(file string) (*beep.StreamSeekCloser, *beep.Format) {
	sound, _ := os.Open(file)
	streamer, format, err := mp3.Decode(sound)
	if err != nil {
		panic(err)
	}

	return &streamer, &format
}

func prepareBuffer(file string) (*beep.Buffer, *beep.Format) {
	sound, _ := os.Open(file)
	streamer, format, err := mp3.Decode(sound)
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

	shotBuffer4, shotSoundFormat = prepareBuffer("sound/shoot-mixed.mp3")
	soundEffects["sound/shoot-mixed.mp3"] = &soundEffect{
		buffer: shotBuffer4,
		volume: -0.9,
	}

	spawnBuffer, _ = prepareBuffer("sound/spawn.mp3")
	soundEffects["sound/spawn.mp3"] = &soundEffect{
		buffer: spawnBuffer,
		volume: -0.9,
	}
	// TODO: Use a real sound effect for this
	soundEffects["menu/increment"] = &soundEffect{
		buffer: spawnBuffer,
		volume: -0.9,
	}

	spawnBuffer2, _ = prepareBuffer("sound/spawn2.mp3")
	spawnBuffer3, _ = prepareBuffer("sound/spawn3.mp3")
	spawnBuffer4, _ = prepareBuffer("sound/spawn4.mp3")
	spawnBuffer5, _ = prepareBuffer("sound/spawn5.mp3")
	snakeSpawnBuffer, _ = prepareBuffer("sound/snakespawn.mp3")

	pinkSquareSpawnBuffer, _ = prepareBuffer("sound/snakespawn.mp3")
	blackholeHitBuffer, _ = prepareBuffer("sound/blackholehit.mp3")
	blackholeDieBuffer, _ = prepareBuffer("sound/blackholedie.mp3")

	lifeBuffer, _ = prepareBuffer("sound/life.mp3")
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

	bombBuffer, _ = prepareBuffer("sound/usebomb.mp3")

	musicStreamer, soundFormat = prepareStreamer("sound/music.mp3")
	pacifismMusicStreamer, soundFormat = prepareStreamer("sound/pacifismmusic.mp3")
	menuMusicStreamer, soundFormat = prepareStreamer("sound/menumusic.mp3")
	introStreamer, soundFormat = prepareStreamer("sound/intro.mp3")
	speaker.Init(soundFormat.SampleRate, soundFormat.SampleRate.N(time.Second/10))
}

func PlayIntroMusic() {
	speaker.Clear()
	s := *introStreamer
	s.Seek(0)
	speaker.Play(s)
	// defer s.Close()
}

func PlayPacifismMusic() {
	speaker.Clear()
	s := *pacifismMusicStreamer
	s.Seek(0)
	speaker.Play(s)
	// defer s.Close()
}

func PlayMenuMusic() {
	speaker.Clear()
	s := *menuMusicStreamer
	s.Seek(0)
	speaker.Play(s)
	// defer s.Close()
}

func PlayMusic() {
	speaker.Clear()
	s := *musicStreamer
	s.Seek(0)
	speaker.Play(s)
	// defer s.Close()
}

func PlaySound(soundName string) {
	soundEffect := soundEffects[soundName]
	sound := soundEffect.buffer.Streamer(0, soundEffect.buffer.Len())

	volume := &effects.Volume{
		Streamer: sound,
		Base:     10,
		Volume:   soundEffect.volume,
		Silent:   false,
	}
	speaker.Play(volume)
}