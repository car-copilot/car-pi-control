package main

import (
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var piSugarIP *string
var piSugarPort *string
var sleepTimerDefault time.Duration
var shutdownTimerDefault time.Duration
var scheduledShutDownTimer time.Duration

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
	pisugar_send_command(fmt.Sprintf("set rtc_wake_alarm %s 5", wakeupTime.Format("2006-01-02T15:04:05.000-07:00")))
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
		{time: time.Duration(8) * time.Hour, duration: time.Duration(45) * time.Minute},
		{time: time.Duration(12) * time.Hour, duration: time.Duration(45) * time.Minute},
		{time: time.Duration(17) * time.Hour, duration: time.Duration(45) * time.Minute},
	}
	if dayInList(time.Now().Weekday(), powerOnDays) {
		for i, t := range powerOnTime {
			hour, _ := math.Modf(t.time.Hours())
			minute, _ := math.Modf(t.time.Minutes())
			powerTimeToday := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), int(hour), int(minute), 0, 0, time.Now().Location())
			if time.Now().Before(powerTimeToday) {
				set_rtc_wake_alarm(powerTimeToday)
			}
			if i == len(powerOnTime)-1 {
				hour, _ := math.Modf(powerOnTime[0].time.Hours())
				minute, _ := math.Modf(powerOnTime[0].time.Minutes())
				powerOnTimeTomorrow := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), int(hour), int(minute), 0, 0, time.Now().Location())
				set_rtc_wake_alarm(powerOnTimeTomorrow)
			}
		}
	}

}

func start() {
	log.Info().Msg("Starting")
}

func shutdown() {
	log.Info().Msg("Shutdown initiated")
	switch_wake_up_alarm()
	// cmd := exec.Command("shutdown", "now")
	// err := cmd.Run()
	// if err != nil {
	// 	log.Error().Msg("Error shutting down computer")
	// }
}

func sleep() {
	log.Info().Msg("Sleep initiated")
}

func run() {
	sleepTimer := sleepTimerDefault
	shutdownTimer := shutdownTimerDefault
	previoslyConnected := false
	for {
		if connected := get_battery_power_plugged(); connected {
			if !previoslyConnected {
				log.Info().Msg("Connected to power source")
				log.Info().Msg("Resetting timers")
				sleepTimer = sleepTimerDefault
				shutdownTimer = shutdownTimerDefault
				start()
			}
			previoslyConnected = true
		} else {
			if sleepTimer > (5 * time.Second) {
				log.Info().Msgf("Sleeping down in %s", sleepTimer)
			} else {
				sleep()
				if shutdownTimer > (5 * time.Second) {
					log.Info().Msgf("Shutting down in %s", shutdownTimer)
				} else {
					shutdown()
				}
			}
			sleepTimer = sleepTimer - 10*time.Second
			previoslyConnected = false
		}
		scheduledShutDownTimer = scheduledShutDownTimer - 10*time.Second
		time.Sleep(10 * time.Second)
	}
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).Level(zerolog.InfoLevel)

	sleepTimerDefault = *flag.Duration("timer", 1*time.Minute, "Time to wait before sleep down")
	piSugarIP = flag.String("address", "127.0.0.1", "Address of the server to connect to")
	piSugarPort = flag.String("port", "8423", "Port of the server to connect to")
	flag.Parse()

	sync_time_from_rtc()

	go run()

}
