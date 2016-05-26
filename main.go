package main

/*
#cgo CFLAGS: -DUNICODE -DWINVER=0x500
#include <Windows.h>
*/
import "C"

import (
	"fmt"
	"github.com/AllenDang/w32"
	"github.com/gonutz/di8"
	"syscall"
	"unsafe"
)

var (
	pad                di8.Device
	state              di8.JoyState
	leftRightDirection int
	goingForward       bool
)

func main() {
	// open a window
	windowProc := syscall.NewCallback(handleMessage)

	class := w32.WNDCLASSEX{
		Size:      C.sizeof_WNDCLASSEX,
		WndProc:   windowProc,
		Cursor:    w32.LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(w32.IDC_ARROW)))),
		ClassName: syscall.StringToUTF16Ptr("GoWindowClass"),
	}

	atom := w32.RegisterClassEx(&class)
	if atom == 0 {
		panic("RegisterClassEx failed")
	}

	window := w32.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr("GoWindowClass"),
		nil,
		w32.WS_OVERLAPPEDWINDOW|w32.WS_VISIBLE,
		10, 10, 600, 250,
		0, 0, 0, nil,
	)
	if window == 0 {
		panic("CreateWindowEx failed")
	}

	w32.SetWindowText(window, "No More Room in Hell - Gamepad to Mouse+Keyboard")

	// set up DirectInput8
	check(di8.Init())
	defer di8.Close()

	dinput, err := di8.Create(unsafe.Pointer(w32.GetModuleHandle("")))
	check(err)
	defer dinput.Release()

	deviceFound := false
	var deviceInstance di8.DeviceInstance

	// use the first game pad that is found
	dinput.EnumDevices(
		di8.DEVCLASS_GAMECTRL,
		func(instance di8.DeviceInstance) bool {
			deviceInstance = instance
			deviceFound = true
			return false
		},
		di8.EDFL_ATTACHEDONLY,
	)

	if !deviceFound {
		panic("no device found")
	}

	// initialize the game pad
	device, err := dinput.CreateDevice(deviceInstance.GuidInstance)
	check(err)
	defer device.Release()

	check(device.SetCooperativeLevel(
		unsafe.Pointer(window),
		// receive data even when the window is not active
		di8.SCL_EXCLUSIVE|di8.SCL_BACKGROUND),
	)
	check(device.SetPredefinedDataFormat(di8.DataFormatJoystick))
	check(device.SetPredefinedProperty(
		di8.PROP_BUFFERSIZE,
		di8.NewPropDword(0, di8.PH_DEVICE, 32),
	))
	check(device.Acquire())

	pad = device

	// this timer is used to poll the game pad regularly
	C.SetTimer(C.HWND(unsafe.Pointer(window)), 1, 10, nil)

	var msg w32.MSG
	for w32.GetMessage(&msg, 0, 0, 0) > 0 {
		w32.TranslateMessage(&msg)
		w32.DispatchMessage(&msg)
	}
}

func handleMessage(window w32.HWND, message uint32, w, l uintptr) uintptr {

	// TODO for the di8 lib: querying the POV with GetState... reports either
	// 0 for north or -1 for everything else, see where the problem is

	if message == w32.WM_TIMER {
		/*
			these are the controls for "No More Room In Hell"

			action        Keyboard      Joypad
			------------------------------------------
			walk        - WASD        - POV up
			look around - mouse move  - POV left/right
			run         - shift       - L2
			crouch      - ctrl        - button 4
			shoot/hit   - left mouse  - button 3
			use/take    - E           - L1
			reload      - R           - button 1
			push away   - V           - R1
			guns menu   - 1           - button 2
			ammo menu   - 2           - Select
			drop/zoom   - right mouse - R2
			last gun    - Q           - Start
		*/

		const (
			runButton        = di8.JOFS_BUTTON6
			crouchButton     = di8.JOFS_BUTTON3
			shootButton      = di8.JOFS_BUTTON2
			useButton        = di8.JOFS_BUTTON4
			reloadButton     = di8.JOFS_BUTTON0
			pushAwayButton   = di8.JOFS_BUTTON5
			gunsMenuButton   = di8.JOFS_BUTTON1
			ammoMenuButton   = di8.JOFS_BUTTON8
			zoomButton       = di8.JOFS_BUTTON7
			lastWeaponButton = di8.JOFS_BUTTON9

			leftRightAxis = di8.JOFS_X
			upDownAxis    = di8.JOFS_Y
		)

		var inputs []w32.INPUT

		pressKey := func(vk uint16, down bool) {
			var flags uint32 = C.KEYEVENTF_SCANCODE
			if !down {
				flags |= C.KEYEVENTF_KEYUP
			}
			inputs = append(
				inputs,
				w32.INPUT{
					Type: w32.INPUT_KEYBOARD,
					Ki: w32.KEYBDINPUT{
						WVk:     vk,
						WScan:   uint16(C.MapVirtualKey(C.UINT(vk), 0)),
						DwFlags: flags,
					},
				},
			)
		}

		sendMouseMove := func(dx int32) {
			inputs = append(inputs, w32.INPUT{
				Type: w32.INPUT_MOUSE,
				Mi: w32.MOUSEINPUT{
					Dx:      dx,
					DwFlags: w32.MOUSEEVENTF_MOVE,
				},
			},
			)
		}

		sendLeftMouseButtonEvent := func(down bool) {
			var flags uint32 = w32.MOUSEEVENTF_LEFTUP
			if down {
				flags = w32.MOUSEEVENTF_LEFTDOWN
			}
			inputs = append(inputs, w32.INPUT{
				Type: w32.INPUT_MOUSE,
				Mi: w32.MOUSEINPUT{
					DwFlags: flags,
				},
			},
			)
		}

		sendRightMouseButtonEvent := func(down bool) {
			var flags uint32 = w32.MOUSEEVENTF_RIGHTUP
			if down {
				flags = w32.MOUSEEVENTF_RIGHTDOWN
			}
			inputs = append(inputs, w32.INPUT{
				Type: w32.INPUT_MOUSE,
				Mi: w32.MOUSEINPUT{
					DwFlags: flags,
				},
			},
			)
		}

		data, err := pad.GetDeviceData(32)
		if err != nil {
			fmt.Println("error getting device data:", err)
		}

		for i := range data {
			down := data[i].Data != 0
			switch data[i].Ofs {
			case runButton:
				pressKey(w32.VK_LSHIFT, down)
			case crouchButton:
				pressKey(w32.VK_LCONTROL, down)
			case useButton:
				pressKey('E', down)
			case reloadButton:
				pressKey('R', down)
			case pushAwayButton:
				pressKey('V', down)
			case gunsMenuButton:
				pressKey('1', down)
			case ammoMenuButton:
				pressKey('2', down)
			case lastWeaponButton:
				pressKey('Q', down)

			case shootButton:
				sendLeftMouseButtonEvent(down)
			case zoomButton:
				sendRightMouseButtonEvent(down)

			case leftRightAxis:
				leftRightDirection = 0
				if data[i].Data < 1000 {
					leftRightDirection = -1
				}
				if data[i].Data > 64000 {
					leftRightDirection = 1
				}
			case upDownAxis:
				newGoingForward := data[i].Data < 1000
				if goingForward != newGoingForward {
					pressKey('W', newGoingForward)
				}
				goingForward = newGoingForward
			}
		}

		if leftRightDirection != 0 {
			sendMouseMove(int32(leftRightDirection) * 20)
		}

		if len(inputs) > 0 {
			w32.SendInput(inputs)
		}

		return 1
	}

	if message == w32.WM_DESTROY {
		w32.PostQuitMessage(0)
		return 1
	}

	return w32.DefWindowProc(window, message, w, l)
}

func check(err error) {
	if err != nil {
		panic(err.Error())
	}
}
