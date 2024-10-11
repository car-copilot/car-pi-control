package carpicontrol

import (
	"os/exec"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/rs/zerolog/log"
)

func Start_bt_agent() {
	cmd := exec.Command("wpctl", "set-volume", "-l", "1.5", "@DEFAULT_AUDIO_SINK@", "98%")
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err)
	}
	go start()
}

func start() {
	defer api.Exit()

	//Connect DBus System bus
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Err(err).Msg("Cannot open Dbus")

	}

	ag := agent.NewSimpleAgent()

	ag.AuthorizeService(ag.Path(), "0000110d-0000-1000-8000-00805f9b34fb")
	ag.AuthorizeService(ag.Path(), "0000110e-0000-1000-8000-00805f9b34fb")

	err = agent.ExposeAgent(conn, ag, agent.CapNoInputNoOutput, true)
	if err != nil {
		log.Err(err).Msg("Error exposing agent")
	}

	a, err := adapter.GetDefaultAdapter()

	if err != nil {
		log.Err(err).Msg("Cannot get adapter")
	}
	a.SetDiscoverable(true)
	a.SetDiscoverableTimeout(0)
	a.SetName("Car Copilot")
	a.SetAlias("Car Copilot")
	a.SetPairable(true)

	log.Debug().Msgf("Address: %v", a.Properties.Address)
	log.Debug().Msgf("Alias: %v", a.Properties.Alias)
	log.Debug().Msgf("Class: %v", a.Properties.Class)
	log.Debug().Msgf("UUIDs:\n %v", a.Properties.UUIDs)

	a.SetPowered(true)

	for {
		time.Sleep(time.Second * 10)
	}
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
