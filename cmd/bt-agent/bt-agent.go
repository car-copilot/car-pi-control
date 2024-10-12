package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/rs/zerolog/log"
)

var dbusConn *dbus.Conn
var audioAdapter *adapter.Adapter1
var carAdapter *adapter.Adapter1

func Start_bt_agent() {
	cmd := exec.Command("wpctl", "set-volume", "-l", "1.5", "@DEFAULT_AUDIO_SINK@", "98%")
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err)
	}

	// Connect DBus System bus
	dbusConn, err = dbus.SystemBus()
	if err != nil {
		log.Err(err).Msg("Cannot open Dbus")

	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(
		sigc,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	endSignal := make(chan struct{})
	go func() {
		fmt.Println("pid:", os.Getpid())
		<-sigc
		fmt.Println("terminating..")
		endSignal <- struct{}{}
	}()

	start_audio()
	start_car()

	<-endSignal

	audioAdapter.SetPowered(false)
	carAdapter.SetPowered(false)

	dbusConn.Close()

}

func start_audio() {
	log.Info().Msg("Starting audio adapter")
	ag := agent.NewSimpleAgent()

	ag.AuthorizeService(ag.Path(), "0000110d-0000-1000-8000-00805f9b34fb")
	ag.AuthorizeService(ag.Path(), "0000110e-0000-1000-8000-00805f9b34fb")

	err := agent.ExposeAgent(dbusConn, ag, agent.CapNoInputNoOutput, true)
	if err != nil {
		log.Err(err).Msg("Error exposing agent")
	}

	audioAdapter, err = adapter.GetAdapter("hci1")

	if err != nil {
		log.Err(err).Msg("Cannot get adapter")
	}
	audioAdapter.SetDiscoverable(true)
	audioAdapter.SetDiscoverableTimeout(0)
	audioAdapter.SetName("Audi A4 Sam")
	audioAdapter.SetAlias("Audi A4 Sam")
	audioAdapter.SetPairable(true)

	log.Debug().Msgf("Address: %v", audioAdapter.Properties.Address)
	log.Debug().Msgf("Alias: %v", audioAdapter.Properties.Alias)
	log.Debug().Msgf("Class: %v", audioAdapter.Properties.Class)
	log.Debug().Msgf("UUIDs:\n %v", audioAdapter.Properties.UUIDs)

	audioAdapter.SetPowered(true)
}

func start_car() {
	log.Info().Msg("Starting car copilot adapter")
	carAdapter, err := adapter.GetAdapter("hci0")

	if err != nil {
		log.Err(err).Msg("Cannot get adapter")
	}
	carAdapter.SetDiscoverable(false)
	carAdapter.SetDiscoverableTimeout(1)
	carAdapter.SetName("Car Copilot")
	carAdapter.SetAlias("Car Copilot")
	carAdapter.SetPairable(false)

	log.Debug().Msgf("Address: %v", carAdapter.Properties.Address)
	log.Debug().Msgf("Alias: %v", carAdapter.Properties.Alias)
	log.Debug().Msgf("Class: %v", carAdapter.Properties.Class)
	log.Debug().Msgf("UUIDs:\n %v", carAdapter.Properties.UUIDs)

	carAdapter.SetPowered(true)
}

func main() {
	Start_bt_agent()
}
