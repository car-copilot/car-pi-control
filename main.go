package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var address *string
var piSugarPort *string
var httpClient = http.DefaultClient

func get_battery_power_plugged(piSugarAddress string) bool {
	connection, err := net.Dial("tcp", piSugarAddress)
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

func set_volume(volume int) {
	form := url.Values{}
	form.Add("volknob", fmt.Sprint(volume))
	form.Add("event", "knob_change")

	resp, err := httpClient.PostForm("http://"+*address+"/command/playback.php?cmd=upd_volume", form)
	if err != nil || resp.StatusCode != 200 {
		log.Error().Msg("Error setting volume")
	}
	defer resp.Body.Close()

	log.Info().Msgf("Volume set to %d", volume)

}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).Level(zerolog.InfoLevel)

	defaultTimer := flag.Duration("timer", 1*time.Minute, "Time to wait before shutting down")
	address = flag.String("address", "127.0.0.1", "Address of the server to connect to")
	piSugarPort = flag.String("port", "8423", "Port of the server to connect to")
	flag.Parse()

	time.Sleep(20 * time.Second)
	set_volume(98)

	timer := *defaultTimer
	for {
		if connected := get_battery_power_plugged(fmt.Sprintf("%s:%s", *address, *piSugarPort)); connected {
			if timer != *defaultTimer {
				log.Info().Msg("Connected to power source")
				log.Info().Msg("Resetting timer")
				timer = *defaultTimer
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
