package starshipkepler

import (
	"fmt"
	"math"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
)

const uiClickWait = 0.125
const uiJoyThreshold = 0.7

const uiActionShoot = pixelgl.MouseButton1
const uiActionAct = pixelgl.MouseButton2
const uiActionActSelf = pixelgl.MouseButton3
const uiActionSwitchMode = pixelgl.KeyLeftControl
const uiActionBoost = pixelgl.KeyLeftShift
const uiActionBomb = pixelgl.KeySpace
const uiActionStop = pixelgl.KeyLeftAlt

type uiContext struct {
	currJoystick pixelgl.Joystick
	MousePos     pixel.Vec
}

func initJoystick(win *pixelgl.Window) pixelgl.Joystick {
	currJoystick := pixelgl.Joystick1
	for i := pixelgl.Joystick1; i <= pixelgl.JoystickLast; i++ {
		if win.JoystickPresent(i) {
			currJoystick = i
			fmt.Printf("Joystick Connected: %d\n", i)
			break
		}
	}
	return currJoystick
}

func NewUi(win *pixelgl.Window) *uiContext {
	return &uiContext{
		currJoystick: initJoystick(win),
	}
}

func uiUp(win *pixelgl.Window, gamePadDir pixel.Vec) bool {
	return win.JustPressed(pixelgl.KeyUp) || gamePadDir.Y > uiJoyThreshold
}

func uiDown(win *pixelgl.Window, gamePadDir pixel.Vec) bool {
	return (win.JustPressed(pixelgl.KeyDown) || gamePadDir.Y < -uiJoyThreshold)
}

func uiChangeSelection(win *pixelgl.Window, gamePadDir pixel.Vec, last time.Time, lastUiAction time.Time) int {
	uiChange := 0

	if last.Sub(lastUiAction).Seconds() > uiClickWait {
		if uiUp(win, gamePadDir) {
			uiChange = -1
		} else if uiDown(win, gamePadDir) {
			uiChange = 1
		}
	}

	return uiChange
}

func uiConfirm(win *pixelgl.Window, currJoystick pixelgl.Joystick) bool {
	return win.JustPressed(pixelgl.KeyEnter) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonA)
}

func uiCancel(win *pixelgl.Window, currJoystick pixelgl.Joystick) bool {
	return win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonB)
}

func uiPause(win *pixelgl.Window, currJoystick pixelgl.Joystick) bool {
	return win.JustPressed(pixelgl.KeyEscape) || win.JoystickJustPressed(currJoystick, pixelgl.ButtonStart)
}

func uiThumbstickVector(win *pixelgl.Window, joystick pixelgl.Joystick, axisX pixelgl.GamepadAxis, axisY pixelgl.GamepadAxis) pixel.Vec {
	v := pixel.V(0.0, 0.0)
	if win.JoystickPresent(joystick) {
		x := win.JoystickAxis(pixelgl.Joystick(pixelgl.Joystick1), axisX)
		y := win.JoystickAxis(pixelgl.Joystick(pixelgl.Joystick1), axisY) * -1

		if math.Abs(x) < 0.2 {
			x = 0
		}
		if math.Abs(y) < 0.2 {
			y = 0
		}

		v = pixel.V(x, y)
	}
	return v
}
