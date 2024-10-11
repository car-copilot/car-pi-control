package main

import (
	"flag"
	"os"
	"time"

	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var piSugarIP *string
var piSugarPort *string
var sleepTimerDefault *time.Duration
var shutdownTimerDefault *time.Duration
var scheduledShutDownTimer *time.Duration

// var httpClient = http.DefaultClient

func pisugar_send_command(command string) string {
	connection, err := net.Dial("tcp", fmt.Sprintf("%s:%s", *piSugarIP, *piSugarPort))
	if err != nil {
		log.Error().Msg("Error connecting to server")
	}
	defer connection.Close()

	_, err = connection.Write([]byte(command))
	if err != nil {
		panic(err)
	}
	buffer := make([]byte, 128)
	mLen, err := connection.Read(buffer)
	if err != nil {
		log.Error().Msg("Error reading from server")
	}
	return strings.TrimSpace(string(buffer[:mLen]))
}

func get_battery_power_plugged() bool {
	out := pisugar_send_command("get battery_power_plugged")
	out = out[len(out)-4:]
	log.Debug().Msgf("Battery power plugged: %s", out)
	return string(out) == "true"

}

func set_rtc_wake_alarm(wakeupTime time.Time) {
	log.Info().Msgf("Setting wake up alarm to %s", wakeupTime)
	//2024-10-02T20:53:26.000+02:00
	log.Debug().Msgf("Computed date string: %s", wakeupTime.Format("2006-01-02T15:04:05.000-07:00"))
	pisugar_send_command(fmt.Sprintf("rtc_alarm_set %s 5", wakeupTime.Format("2006-01-02T15:04:05.000-07:00")))
	set_time := pisugar_send_command("get rtc_alarm_time")

	log.Info().Msgf("Wake up alarm set to %s", set_time[16:])
}

func sync_time_from_web() {
	log.Info().Msg("Syncing time from web")
	pisugar_send_command("rtc_web")
}

func sync_time_from_rtc() {
	log.Info().Msg("Syncing time from rtc")
	pisugar_send_command("rtc_rtc2pi")
}

func dayInList(day time.Weekday, list []time.Weekday) bool {
	for _, d := range list {
		if d == day {
			return true
		}
	}
	return false
}

func switch_wake_up_alarm() {
	log.Info().Msg("Switching wake up alarm")
	powerOnDays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	powerOnTime := []struct {
		time     time.Duration
		duration time.Duration
	}{
		{time: 8 * time.Hour, duration: 45 * time.Minute},
		{time: 12 * time.Hour, duration: 45 * time.Minute},
		{time: 17 * time.Hour, duration: 45 * time.Minute},
	}
	if dayInList(time.Now().Weekday(), powerOnDays) {
		for i, t := range powerOnTime {
			powerTimeToday := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
			powerTimeToday = powerTimeToday.Add(t.time)
			if time.Now().Before(powerTimeToday) {
				set_rtc_wake_alarm(powerTimeToday)
				scheduledShutDownTimer = &t.duration
			}
			if i == len(powerOnTime)-1 {
				powerOnTimeTomorrow := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
				powerOnTimeTomorrow = powerOnTimeTomorrow.Add(powerOnTime[0].time)
				set_rtc_wake_alarm(powerOnTimeTomorrow)
				scheduledShutDownTimer = &powerOnTime[0].duration
			}
		}
	} else {
		powerOnTimeMonday := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
		powerOnTimeMonday = powerOnTimeMonday.Add(powerOnTime[0].time)
		set_rtc_wake_alarm(powerOnTimeMonday)
		scheduledShutDownTimer = &powerOnTime[0].duration
	}

}

func start() {
	log.Info().Msg("Starting pipewire")
	cmd := exec.Command("su", "-", "obito1903", "-c", "systemctl", "--user", "start", "pipewire", "pipewire-pulse", "wireplumber")
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err)
	}
	log.Info().Msg("Starting bt-agent")
	cmd = exec.Command("su", "-", "obito1903", "-c", "systemctl", "--user", "start", "bt-agent.service")
	err = cmd.Run()
	if err != nil {
		log.Error().Err(err)
	}
}

func shutdown() {
	log.Info().Msg("Shutdown initiated")
	switch_wake_up_alarm()
	cmd := exec.Command("shutdown", "now")
	err := cmd.Run()
	if err != nil {
		log.Error().Msg("Error shutting down computer")
	}
}

func rfkill_get_devices_count() int {

	cmd := exec.Command("rfkill", "--raw")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error().Err(err)
	}
	if err := cmd.Start(); err != nil {
		log.Error().Err(err)
	}
	buf := new(strings.Builder)
	_, err = io.Copy(buf, stdout)
	if err != nil {
		log.Error().Err(err)
	}
	s := buf.String()
	log.Debug().Msgf("rfkill out put:\n%s", s)
	count := 0
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count - 1
}

func rfkill_block_all(dcount int) {
	for i := 0; i < dcount; i++ {
		log.Info().Msgf("Blocking device %d", i)
		cmd := exec.Command(fmt.Sprintf("rfkill block %d", i))
		err := cmd.Run()
		if err != nil {
			log.Error().Err(err)
		}
	}
}

func sleep() {
	log.Info().Msg("Sleep initiated")
	log.Info().Msg("Turning Off wifi & bt")
	rfkill_block_all(rfkill_get_devices_count())
	log.Info().Msg("Stopping bt-agent")
	cmd := exec.Command("su", "-", "obito1903", "-c", "systemctl", "--user", "stop", "bt-agent.service")
	err := cmd.Run()
	if err != nil {
		log.Error().Err(err)
	}
	log.Info().Msg("Stopping pipewire")
	cmd = exec.Command("su", "-", "obito1903", "-c", "systemctl", "--user", "stop", "pipewire", "pipewire-pulse", "wireplumber")
	err = cmd.Run()
	if err != nil {
		log.Error().Err(err)
	}
}

// Program loop
func run() {
	sleepTimer := *sleepTimerDefault
	shutdownTimer := shutdownTimerDefault
	previoslyConnected := false
	sleepOn := false
	for {
		if connected := get_battery_power_plugged(); connected {
			if !previoslyConnected {
				log.Info().Msg("Connected to power source")
				log.Info().Msg("Resetting timers")
				sleepTimer = *sleepTimerDefault
				shutdownTimer = shutdownTimerDefault
				sleepOn = false
				start()
			}
			previoslyConnected = true
		} else {
			if sleepTimer > (5 * time.Second) {
				log.Info().Msgf("Sleeping in %s", sleepTimer)
			} else {
				if !sleepOn {
					sleep()
					sleepOn = true
				}
				if !previoslyConnected {
					if *scheduledShutDownTimer > (5 * time.Second) {
						log.Info().Msgf("Shutting down in %s", scheduledShutDownTimer)
					} else {
						shutdown()
					}
				} else {
					if *shutdownTimer > (5 * time.Second) {
						log.Info().Msgf("Shutting down in %s", shutdownTimer)
						newTimer := *shutdownTimer - 10*time.Second
						shutdownTimer = &newTimer
					} else {
						shutdown()
					}
				}
			}
			sleepTimer = sleepTimer - 10*time.Second
		}
		newTimer := *scheduledShutDownTimer - 10*time.Second
		scheduledShutDownTimer = &newTimer
		time.Sleep(10 * time.Second)
	}
}

// Prepare the rpi at boot, syncing rtc to the pi, updating wake up alarm and enter sleep mode if not connected
func init_pi() {
	sync_time_from_rtc()
	switch_wake_up_alarm()

	if connected := get_battery_power_plugged(); connected {
		sleep()
	}
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).Level(zerolog.InfoLevel)

	sleepTimerDefault = flag.Duration("sleeptimer", 1*time.Minute, "Time to wait before low power mode")
	shutdownTimerDefault = flag.Duration("shutdowntimer", 10*time.Minute, "Time to wait before shutdown after sleep")
	piSugarIP = flag.String("address", "127.0.0.1", "Address of the server to connect to")
	piSugarPort = flag.String("port", "8423", "Port of the server to connect to")
	debug := flag.Bool("debug", false, "Enable debug output")
	flag.Parse()

	if *debug {
		log.Logger = log.Level(zerolog.DebugLevel)
	}

	log.Info().Msg("Starting car-pi-control")

	init_pi()
	run()
}
