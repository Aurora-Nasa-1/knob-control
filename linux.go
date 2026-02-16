//go:build linux

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	evdev "github.com/gvalkov/golang-evdev"
)

type LinuxController struct {
	useDDC          bool
	ddcBusNum       string
	brightnessDelta int32
	brightnessCh    chan struct{}
	
	ddcBuses      []string 
	currentBusIdx int

	currentAppID   string
	currentAppName string
}

func NewLinuxController() *LinuxController {
	lc := &LinuxController{
		brightnessCh: make(chan struct{}, 1),
	}
	go lc.brightnessWorker()
	return lc
}

func (lc *LinuxController) DetectBrightnessMode() error {
	lc.ddcBuses = []string{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	out, err := exec.CommandContext(ctx, "ddcutil", "detect", "--terse").Output()
	if err == nil {
		re := regexp.MustCompile(`/dev/i2c-(\d+)`)
		matches := re.FindAllSubmatch(out, -1)
		for _, m := range matches {
			if len(m) > 1 {
				lc.ddcBuses = append(lc.ddcBuses, string(m[1]))
			}
		}
	}
	
	if len(lc.ddcBuses) > 0 {
		lc.useDDC = true
		lc.ddcBusNum = lc.ddcBuses[0]
		if cfg.Verbose {
			log.Printf("[I] Mode: DDC/CI (Buses: %v)", lc.ddcBuses)
		}
	} else {
		entries, _ := os.ReadDir("/sys/class/backlight")
		if len(entries) > 0 {
			lc.useDDC = false
			if cfg.Verbose {
				log.Println("[I] Mode: Laptop Backlight")
			}
		} else {
			if cfg.Verbose {
				log.Println("[W] No brightness control method found")
			}
		}
	}

	return nil
}

func (lc *LinuxController) ChangeVolume(amt string) error {
	val := strings.TrimLeft(amt, "+-")
	op := "+"
	if strings.HasPrefix(amt, "-") {
		op = "-"
	}
	return exec.Command("wpctl", "set-volume", "--limit", "1.0", "@DEFAULT_AUDIO_SINK@", val+op).Run()
}

func (lc *LinuxController) ChangeBrightness(amt string) error {
	s := strings.TrimRight(amt, "%")
	val, _ := strconv.Atoi(s)
	atomic.AddInt32(&lc.brightnessDelta, int32(val))

	select {
	case lc.brightnessCh <- struct{}{}:
	default:
	}
	return nil
}

func (lc *LinuxController) brightnessWorker() {
	for range lc.brightnessCh {
		for {
			delta := atomic.SwapInt32(&lc.brightnessDelta, 0)
			if delta == 0 {
				break
			}
			
			op := "+"
			absVal := delta
			if delta < 0 {
				op = "-"
				absVal = -delta
			}
			valStr := fmt.Sprintf("%d", absVal)

			if lc.useDDC {
				args := []string{"setvcp", "10", op, valStr}
				if lc.ddcBusNum != "" {
					args = append(args, "--bus", lc.ddcBusNum)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				exec.CommandContext(ctx, "ddcutil", args...).Run()
				cancel()
			} else {
				exec.Command("brightnessctl", "set", valStr+"%"+op).Run()
			}
		}
	}
}

func (lc *LinuxController) HandleMuteLogic() error {
	sinks, def, err := lc.getSinks()
	if err != nil {
		return err
	}

	if len(sinks) <= 1 {
		exec.Command("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle").Run()
		lc.ShowNotification("Mute Toggled", "")
	} else {
		lc.switchDevice(sinks, def)
	}
	return nil
}

func (lc *LinuxController) switchDevice(sinks []string, cur string) {
	idx := 0
	for i, name := range sinks {
		if name == cur {
			idx = (i + 1) % len(sinks)
			break
		}
	}
	next := sinks[idx]
	exec.Command("pactl", "set-default-sink", next).Run()
	exec.Command("pactl", "set-sink-mute", next, "0").Run()
	lc.ShowNotification("Output Switched", next)
}

func (lc *LinuxController) getSinks() ([]string, string, error) {
	defOut, err := exec.Command("pactl", "get-default-sink").Output()
	if err != nil {
		return nil, "", err
	}
	def := strings.TrimSpace(string(defOut))

	out, err := exec.Command("pactl", "list", "short", "sinks").Output()
	if err != nil {
		return nil, "", err
	}

	var sinks []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	excludes := []string{"hdmi", "digital-video", "spdif"}

	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		name := f[1]

		if !cfg.IncludeHDMI {
			excl := false
			lname := strings.ToLower(name)
			for _, k := range excludes {
				if strings.Contains(lname, k) {
					excl = true
					break
				}
			}
			if excl {
				continue
			}
		}
		sinks = append(sinks, name)
	}
	return sinks, def, nil
}

func (lc *LinuxController) ShowNotification(title, body string) error {
	return exec.Command("notify-send", "-t", "1000", "-h", "string:x-canonical-private-synchronous:vol-knob", title, body).Start()
}

func (lc *LinuxController) SwitchScreen() error {
	if !lc.useDDC || len(lc.ddcBuses) <= 1 {
		lc.ShowNotification("Display", "Single Display Mode")
		return nil
	}

	lc.currentBusIdx = (lc.currentBusIdx + 1) % len(lc.ddcBuses)
	lc.ddcBusNum = lc.ddcBuses[lc.currentBusIdx]
	
	lc.ShowNotification("Display Switched", fmt.Sprintf("Bus: %s", lc.ddcBusNum))
	return nil
}

type SinkInput struct {
	ID   string
	Name string
}

func (lc *LinuxController) getSinkInputs() ([]SinkInput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	out, err := exec.CommandContext(ctx, "pactl", "list", "sink-inputs").Output()
	if err != nil {
		return nil, err
	}

	var inputs []SinkInput
	var currentID string
	var currentName string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Sink Input #") {
			if currentID != "" {
				if currentName == "" { currentName = "Unknown" }
				inputs = append(inputs, SinkInput{ID: currentID, Name: currentName})
			}
			fmt.Sscanf(line, "Sink Input #%s", &currentID)
			currentName = ""
		} else if strings.HasPrefix(line, "application.name = ") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				currentName = strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		}
	}
	if currentID != "" {
		if currentName == "" { currentName = "Unknown" }
		inputs = append(inputs, SinkInput{ID: currentID, Name: currentName})
	}
	
	return inputs, nil
}

func (lc *LinuxController) SwitchApp() error {
	inputs, err := lc.getSinkInputs()
	if err != nil {
		return err
	}
	
	if len(inputs) == 0 {
		lc.ShowNotification("App Volume", "No active apps found")
		lc.currentAppID = ""
		return nil
	}

	idx := -1
	if lc.currentAppID != "" {
		for i, inp := range inputs {
			if inp.ID == lc.currentAppID {
				idx = i
				break
			}
		}
	}
	
	newIdx := (idx + 1) % len(inputs)
	target := inputs[newIdx]
	
	lc.currentAppID = target.ID
	lc.currentAppName = target.Name
	
	lc.ShowNotification("App Selected", target.Name)
	return nil
}

func (lc *LinuxController) ChangeAppVolume(amt string) error {
	if lc.currentAppID == "" {
		if err := lc.SwitchApp(); err != nil || lc.currentAppID == "" {
			return nil
		}
	}
	
	val := strings.TrimLeft(amt, "+-")
	op := "+"
	if strings.HasPrefix(amt, "-") {
		op = "-"
	}

	err := exec.Command("pactl", "set-sink-input-volume", lc.currentAppID, val+op).Run()
	if err != nil {
	}
	return err
}


type LinuxInputHandler struct {
	dev *evdev.InputDevice
}

func NewLinuxInputHandler() *LinuxInputHandler {
	return &LinuxInputHandler{}
}

func (li *LinuxInputHandler) Start(events chan<- InputEvent) error {
	target := cfg.DevicePath
	if target == "" {
		var err error
		target, err = li.findDevicePath(cfg.SearchKeyword)
		if err != nil {
			return fmt.Errorf("device not found: %v", err)
		}
		if cfg.Verbose {
			fmt.Printf("[I] Device: %s\n", target)
		}
	}

	dev, err := evdev.Open(target)
	if err != nil {
		return fmt.Errorf("open failed: %v", err)
	}
	li.dev = dev

	if err = dev.Grab(); err != nil {
		return fmt.Errorf("grab failed: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		li.Stop()
		os.Exit(0)
	}()

	go func() {
		for {
			event, err := dev.ReadOne()
			if err != nil {
				close(events)
				break
			}

			if event.Type == evdev.EV_KEY && (event.Value == 1 || event.Value == 2) {
				switch event.Code {
				case evdev.KEY_VOLUMEUP:
					events <- InputEvent{Type: EventVolumeUp, Value: int(event.Value)}
				case evdev.KEY_VOLUMEDOWN:
					events <- InputEvent{Type: EventVolumeDown, Value: int(event.Value)}
				case evdev.KEY_MUTE:
					events <- InputEvent{Type: EventMute, Value: int(event.Value)}
				}
			}
		}
	}()

	return nil
}

func (li *LinuxInputHandler) Stop() error {
	if li.dev != nil {
		return li.dev.Release()
	}
	return nil
}

func (li *LinuxInputHandler) findDevicePath(kw string) (string, error) {
	devs, err := evdev.ListInputDevices()
	if err != nil {
		return "", err
	}
	kw = strings.ToLower(kw)
	for _, d := range devs {
		if strings.Contains(strings.ToLower(d.Name), kw) {
			return d.Fn, nil
		}
	}
	return "", fmt.Errorf("not found: %s", kw)
}

func GetPlatformController() PlatformController {
	return NewLinuxController()
}

func GetInputHandler() InputHandler {
	return NewLinuxInputHandler()
}
