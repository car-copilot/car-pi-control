package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
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

func set_config(card int) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	var data = strings.NewReader(fmt.Sprintf("output_device_cardnum=%d&update_output_device=novalue&mixer_type=null&camilladsp_volume_range=60&i2sdevice=None&i2soverlay=None&drvoptions=none&autoplay=1&extmeta=0&ashufflesvc=0&ashuffle_mode=Track&ashuffle_window=7&ashuffle_filter=None&volume_step_limit=5&volume_mpd_max=100&volume_db_display=1&usb_volknob=0&rotaryenc=0&rotenc_params=100+2+3+23+24&mpdcrossfade=0&mpd_httpd=0&mpd_httpd_port=8000&mpd_httpd_encoder=lame&cdsp_mode=Audi.yml", card)
	req, err := http.NewRequest("POST", "http://"+*address+"/snd-config.php", data)
	if err != nil {
		log.Fatal().Err(err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "PHPSESSID=ho7vk67sqrjua8sme0pqhsjgdq")
	req.Header.Set("Origin", "http://"+*address)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "http://"+*address+"/snd-config.php")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal().Err(err)
	}
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).Level(zerolog.InfoLevel)

	defaultTimer := flag.Duration("timer", 1*time.Minute, "Time to wait before shutting down")
	address = flag.String("address", "127.0.0.1", "Address of the server to connect to")
	piSugarPort = flag.String("port", "8423", "Port of the server to connect to")
	flag.Parse()

	go func() {
		time.Sleep(20 * time.Second)
		set_config(0)
		set_volume(98)
	}()

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
