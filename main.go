package main

import (
	"flag"
	"log"
	"sync"
	"time"
)

type ControlMode int

const (
	ModeAudio ControlMode = iota
	ModeBrightness
	ModeAppVolume
)

type Config struct {
	StepSize      string
	IncludeHDMI   bool
	DevicePath    string
	SearchKeyword string
	Verbose       bool
}

var (
	cfg         Config
	currentMode = ModeAudio
	
	muteTimer  *time.Timer
	clickCount int
	mu         sync.Mutex

	controller PlatformController
)

func main() {
	flag.StringVar(&cfg.StepSize, "step", "2%", "Adjustment step")
	flag.BoolVar(&cfg.IncludeHDMI, "hdmi", false, "Include HDMI/DP outputs")
	flag.StringVar(&cfg.DevicePath, "device", "", "Device path")
	flag.StringVar(&cfg.SearchKeyword, "keyword", "Consumer Control", "Search keyword")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose logging")
	flag.Parse()

	controller = GetPlatformController()
	if err := controller.DetectBrightnessMode(); err != nil {
		log.Printf("[W] Brightness detection failed: %v", err)
	}

	inputHandler := GetInputHandler()
	events := make(chan InputEvent)
	
	if err := inputHandler.Start(events); err != nil {
		log.Fatalf("[E] Input handler failed: %v", err)
	}
	defer inputHandler.Stop()

	log.Println("Started.")

	for event := range events {
		switch event.Type {
		case EventVolumeUp:
			handleAdjustment("up")
		case EventVolumeDown:
			handleAdjustment("down")
		case EventMute:
			if event.Value == 1 {
				handleMutePress()
			}
		}
	}
}

func handleAdjustment(dir string) {
	prefix := "+"
	if dir == "down" {
		prefix = "-"
	}

	amt := prefix + cfg.StepSize
	switch currentMode {
	case ModeAudio:
		controller.ChangeVolume(amt)
	case ModeBrightness:
		controller.ChangeBrightness(amt)
	case ModeAppVolume:
		controller.ChangeAppVolume(amt)
	}
}

func handleMutePress() {
	mu.Lock()
	defer mu.Unlock()

	clickCount++
	if muteTimer != nil {
		muteTimer.Stop()
	}

	var t *time.Timer
	t = time.AfterFunc(400*time.Millisecond, func() {
		mu.Lock()
		defer mu.Unlock()
		
		if muteTimer != t {
			return
		}

		switch clickCount {
		case 1:
			switch currentMode {
			case ModeAudio:
				controller.HandleMuteLogic()
			case ModeBrightness:
				controller.SwitchScreen()
			case ModeAppVolume:
				controller.SwitchApp()
			}
		case 2:
			toggleControlMode()
		case 3:
			switchToAppVolumeMode()
		default:
			log.Printf("Ignored %d clicks", clickCount)
		}

		clickCount = 0
		muteTimer = nil
	})
	muteTimer = t
}

func toggleControlMode() {
	if currentMode == ModeAudio {
		currentMode = ModeBrightness
		controller.ShowNotification("Mode: Brightness", "Control: Backlight/DDC")
	} else {
		currentMode = ModeAudio
		controller.ShowNotification("Mode: Audio", "Control: System Volume")
	}
}

func switchToAppVolumeMode() {
	if currentMode != ModeAppVolume {
		currentMode = ModeAppVolume
		controller.ShowNotification("Mode: App Volume", "Control: Application Volume")
	} else {
		controller.ShowNotification("Mode: App Volume", "Already Active")
	}
}
