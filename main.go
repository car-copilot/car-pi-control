package main

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func get_battery_power_plugged() bool {
	connection, err := net.Dial("tcp", "127.0.0.1:8423")
	if err != nil {
		log.Error().Msg("Error connecting to server")
	}
	defer connection.Close()

	_, err = connection.Write([]byte("get battery_power_plugged"))
	if err != nil {
		panic(err)
	}
	buffer := make([]byte, 128)
	mLen, err := connection.Read(buffer)
	if err != nil {
		log.Error().Msg("Error reading from server")
	}
	out := strings.TrimSpace(string(buffer[:mLen]))
	out = out[len(out)-4:]
	log.Debug().Msgf("Battery power plugged: %s", out)
	return string(out) == "true"

}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	defaultTimer := 1 * time.Minute
	timer := defaultTimer
	for {
		if connected := get_battery_power_plugged(); connected {
			if timer != defaultTimer {
				log.Info().Msg("Connected to power source")
				log.Info().Msg("Resetting timer")
				timer = defaultTimer
			}
			time.Sleep(10 * time.Second)
		} else {
			log.Info().Msgf("Shuting down in %s", timer)
			if timer < (5 * time.Second) {
				log.Info().Msg("Shutdown initiated")
				// Shutdown the computer
				cmd := exec.Command("shutdown", "now")
				err := cmd.Run()
				if err != nil {
					log.Error().Msg("Error shutting down computer")
				}
			}
			timer = timer - 10*time.Second
			time.Sleep(10 * time.Second)
		}
	}
}
