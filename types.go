package main

type PlatformController interface {
	DetectBrightnessMode() error
	ChangeVolume(amt string) error
	ChangeBrightness(amt string) error
	HandleMuteLogic() error
	SwitchScreen() error
	SwitchApp() error
	ChangeAppVolume(amt string) error
	ShowNotification(title, body string) error
}

type InputHandler interface {
	Start(events chan<- InputEvent) error
	Stop() error
}

type InputEventType int

const (
	EventVolumeUp InputEventType = iota
	EventVolumeDown
	EventMute
)

type InputEvent struct {
	Type  InputEventType
	Value int
}
