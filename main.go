package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type Config struct {
	APIURL             string            `json:"api_url"`
	AuthHeader         string            `json:"auth_header"`
	AudioFiles         map[string]string `json:"audio_files"`
	AlertOnEmpty       string            `json:"alert_on_empty"`
	TimeZone           string            `json:"time_zone"`
	LogToFile          bool              `json:"log_to_file"`
	LogFilePath        string            `json:"log_file_path"`
	LogLevel           int               `json:"log_level"`
	RepeatAudioFile    string            `json:"repeat_audio_file"`
	EnableRepeatAudio  bool              `json:"enable_repeat_audio"`
	RepeatIntervalMin  int               `json:"repeat_interval_min"`
	RequestIntervalSec int               `json:"request_interval_sec"`
	LogToConsole       bool              `json:"log_to_console"`
}

type Alert struct {
	RegionID   string `json:"regionId"`
	RegionType string `json:"regionType"`
	Type       string `json:"type"`
	LastUpdate string `json:"lastUpdate"`
}

type APIResponse struct {
	RegionID      string  `json:"regionId"`
	RegionType    string  `json:"regionType"`
	RegionName    string  `json:"regionName"`
	RegionEngName string  `json:"regionEngName"`
	LastUpdate    string  `json:"lastUpdate"`
	ActiveAlerts  []Alert `json:"activeAlerts"`
}

type State struct {
	IsActive        bool   `json:"isActive"`
	EventLastUpdate string `json:"eventLastUpdate"`
	EventRegion     string `json:"eventRegion"`
	ActiveEventType string `json:"activeEventType"`
	Alarmed         bool   `json:"alarmed"`
}

var logger *log.Logger

var repeatPlayedMinutes = make(map[string]map[string]bool)

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, err
	}
	if config.LogFilePath == "" {
		config.LogFilePath = "alert.log"
	}
	return &config, nil
}

func InitLogger(config *Config) {
	var writers []io.Writer
	if config.LogToFile {
		logOutput, err := os.OpenFile(config.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Помилка лог файлу: %v", err)
		}
		writers = append(writers, logOutput)
	}
	if config.LogToConsole {
		writers = append(writers, os.Stdout)
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}
	logger = log.New(io.MultiWriter(writers...), "", log.LstdFlags)
}

func Log(level int, config *Config, message string) {
	if level > config.LogLevel || config.LogLevel == 0 {
		return
	}
	location, err := time.LoadLocation(config.TimeZone)
	if err != nil {
		location = time.Local
	}
	currentTime := time.Now().In(location).Format("2006-01-02 15:04:05")
	logger.Printf("[%s] %s", currentTime, message)
}

func PlayAudio(filePath string) {
	if filePath == "" {
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Помилка загрузки аудіо: %v", err)
		return
	}
	defer file.Close()

	streamer, format, err := mp3.Decode(file)
	if err != nil {
		log.Printf("Помилка декодування mp3: %v", err)
		return
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	done := make(chan struct{})
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		close(done)
	})))
	<-done
}

func LoadState(path string) ([]State, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []State{}, nil
		}
		return nil, err
	}
	defer file.Close()
	var states []State
	err = json.NewDecoder(file).Decode(&states)
	if err != nil {
		return nil, err
	}
	return states, nil
}

func SaveState(path string, states []State) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(states)
}

func ProcessAlerts(config *Config) {
	// 1. Запрос к API
	req, err := http.NewRequest("GET", config.APIURL, nil)
	if err != nil {
		Log(3, config, "Помилка запиту до сервера: "+err.Error())
		return
	}
	req.Header.Set("Authorization", config.AuthHeader)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		Log(3, config, "Помилка виконання API запиту: "+err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log(3, config, "Помилка читання відповіді API: "+err.Error())
		return
	}
	var apiResponse []APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		Log(3, config, "Помилка розпознавання запиту API: "+err.Error())
		return
	}
	if len(apiResponse) == 0 {
		Log(2, config, "Пуста відповідь API")
		return
	}

	// 2. Определяем параметры события
	isActive := false
	eventLastUpdate := apiResponse[0].LastUpdate
	eventRegion := apiResponse[0].RegionID
	eventRegionEng := apiResponse[0].RegionEngName
	eventRegionName := apiResponse[0].RegionName
	var activeEventType string
	if len(apiResponse[0].ActiveAlerts) > 0 {
		isActive = true
		earliestAlert := apiResponse[0].ActiveAlerts[0]
		for _, alert := range apiResponse[0].ActiveAlerts {
			alertTime, _ := time.Parse(time.RFC3339, alert.LastUpdate)
			earliestTime, _ := time.Parse(time.RFC3339, earliestAlert.LastUpdate)
			if alertTime.Before(earliestTime) {
				earliestAlert = alert
			}
		}
		eventLastUpdate = earliestAlert.LastUpdate
		eventRegion = earliestAlert.RegionID
		activeEventType = earliestAlert.Type
	} else {
		activeEventType = ""
	}

	if config.LogLevel >= 2 {
		eventTimeStr := eventLastUpdate
		if eventLastUpdate != "" {
			if t, err := time.Parse(time.RFC3339, eventLastUpdate); err == nil {
				loc, err := time.LoadLocation(config.TimeZone)
				if err == nil {
					eventTimeStr += " (" + t.In(loc).Format("2006-01-02 15:04:05") + ")"
				}
			}
		}
		regionStr := eventRegion
		if eventRegionEng := eventRegionEng; eventRegionEng != "" {
			regionStr += " (" + eventRegionEng + ")"
		}
		Log(2, config, "isActive="+boolToStr(isActive)+", eventLastUpdate="+eventTimeStr+", eventRegion="+regionStr+", activeEventType="+activeEventType)
	}

	ProcessStateWithRegion(config, isActive, eventLastUpdate, eventRegion, activeEventType, eventRegionName)
}

func ProcessStateWithRegion(config *Config, isActive bool, eventLastUpdate, eventRegion, activeEventType, regionName string) {
	statePath := "state.json"
	states, err := LoadState(statePath)
	if err != nil {
		Log(3, config, "Помилка завантаження state: "+err.Error())
		return
	}

	var currentState *State
	for i := range states {
		if states[i].EventRegion == eventRegion {
			currentState = &states[i]
			break
		}
	}

	if currentState == nil {
		newState := State{
			IsActive:        isActive,
			EventLastUpdate: eventLastUpdate,
			EventRegion:     eventRegion,
			ActiveEventType: activeEventType,
			Alarmed:         false,
		}
		if isActive {
			Log(1, config, "Початок повітряної тривоги ("+activeEventType+") в регіоні: "+regionName)
			PlayAudio(config.AudioFiles[activeEventType])
			newState.Alarmed = true
		} else {
			newState.Alarmed = true
		}
		states = append(states, newState)
		_ = SaveState(statePath, states)
		return
	}

	currentState.EventLastUpdate = eventLastUpdate
	currentState.ActiveEventType = activeEventType

	if currentState.IsActive != isActive {
		if isActive {
			Log(1, config, "Початок повітряної тривоги ("+activeEventType+") в регіоні: "+regionName)
			PlayAudio(config.AudioFiles[activeEventType])
		} else {
			Log(1, config, "Кінець повітряної тривоги в регіоні: "+regionName)
			PlayAudio(config.AlertOnEmpty)
		}
		currentState.IsActive = isActive
		currentState.Alarmed = true
		_ = SaveState(statePath, states)
		return
	}

	if !currentState.Alarmed {
		if isActive {
			Log(1, config, "Повітряна тривога триває")
			PlayAudio(config.AudioFiles[activeEventType])
		} else {
			PlayAudio(config.AlertOnEmpty)
		}
		currentState.Alarmed = true
		_ = SaveState(statePath, states)
		return
	}

	if isActive && config.EnableRepeatAudio {
		checkAndPlayRepeatAudio(config, currentState, eventLastUpdate)
	}

	_ = SaveState(statePath, states)
}

func checkAndPlayRepeatAudio(config *Config, state *State, eventLastUpdate string) {
	location, err := time.LoadLocation(config.TimeZone)
	if err != nil {
		location = time.Local
	}
	start, err := time.Parse(time.RFC3339, eventLastUpdate)
	if err != nil {
		return
	}
	now := time.Now().In(location).UTC().Truncate(time.Minute)
	interval := time.Duration(config.RepeatIntervalMin) * time.Minute
	next := start.UTC().Truncate(time.Minute).Add(interval)

	region := state.EventRegion
	if repeatPlayedMinutes[region] == nil {
		repeatPlayedMinutes[region] = make(map[string]bool)
	}

	for next.Before(now) || next.Equal(now) {
		minuteKey := next.Format("2006-01-02T15:04")
		if next.Equal(now) {
			if !repeatPlayedMinutes[region][minuteKey] {
				PlayAudio(config.RepeatAudioFile)
				Log(1, config, "Повітряна тривога триває (повтор)")
				repeatPlayedMinutes[region][minuteKey] = true
			}
			break
		}
		next = next.Add(interval)
	}
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func main() {
	config, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Помилка завантаження файлу конфігурації: %v", err)
	}
	InitLogger(config)

	firstRun := true
	for {
		isActive, regionName := ProcessAlertsWithFirstRun(config, firstRun)
		if firstRun && config.LogLevel != 0 {
			stateStr := "не активна"
			if isActive {
				stateStr = "активна"
			}
			Log(1, config, "Моніторинг тривог для регіону "+regionName+". Поточний стан: "+stateStr)
			firstRun = false
		}
		time.Sleep(time.Duration(config.RequestIntervalSec) * time.Second)
	}
}

// Обертка для ProcessAlerts, возвращает isActive и regionName для первого запуска
func ProcessAlertsWithFirstRun(config *Config, firstRun bool) (bool, string) {
	// ...код ProcessAlerts до определения isActive, eventRegionName...
	req, err := http.NewRequest("GET", config.APIURL, nil)
	if err != nil {
		Log(3, config, "Помилка запиту до сервера: "+err.Error())
		return false, ""
	}
	req.Header.Set("Authorization", config.AuthHeader)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		Log(3, config, "Помилка виконання API запиту: "+err.Error())
		return false, ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log(3, config, "Помилка читання відповіді API: "+err.Error())
		return false, ""
	}
	var apiResponse []APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		Log(3, config, "Помилка розпознавання запиту API: "+err.Error())
		return false, ""
	}
	if len(apiResponse) == 0 {
		Log(2, config, "Пуста відповідь API")
		return false, ""
	}

	isActive := false
	eventLastUpdate := apiResponse[0].LastUpdate
	eventRegion := apiResponse[0].RegionID
	eventRegionEng := apiResponse[0].RegionEngName
	eventRegionName := apiResponse[0].RegionName
	var activeEventType string
	if len(apiResponse[0].ActiveAlerts) > 0 {
		isActive = true
		earliestAlert := apiResponse[0].ActiveAlerts[0]
		for _, alert := range apiResponse[0].ActiveAlerts {
			alertTime, _ := time.Parse(time.RFC3339, alert.LastUpdate)
			earliestTime, _ := time.Parse(time.RFC3339, earliestAlert.LastUpdate)
			if alertTime.Before(earliestTime) {
				earliestAlert = alert
			}
		}
		eventLastUpdate = earliestAlert.LastUpdate
		eventRegion = earliestAlert.RegionID
		activeEventType = earliestAlert.Type
	} else {
		activeEventType = ""
	}

	if config.LogLevel >= 2 {
		eventTimeStr := eventLastUpdate
		if eventLastUpdate != "" {
			if t, err := time.Parse(time.RFC3339, eventLastUpdate); err == nil {
				loc, err := time.LoadLocation(config.TimeZone)
				if err == nil {
					eventTimeStr += " (" + t.In(loc).Format("2006-01-02 15:04:05") + ")"
				}
			}
		}
		regionStr := eventRegion
		if eventRegionEng != "" {
			regionStr += " (" + eventRegionEng + ")"
		}
		Log(2, config, "isActive="+boolToStr(isActive)+", eventLastUpdate="+eventTimeStr+", eventRegion="+regionStr+", activeEventType="+activeEventType)
	}

	ProcessStateWithRegion(config, isActive, eventLastUpdate, eventRegion, activeEventType, eventRegionName)
	return isActive, eventRegionName
}
