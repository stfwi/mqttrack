package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"mqttrack/mqttnode"
	"mqttrack/recorder"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const DEFAULT_CONFIG_FILE string = "conf/mqttrack.json"

type AppSettings struct {
	MQTT     mqttnode.Settings `json:"mqtt"`
	Recorder recorder.Settings `json:"recorder"`
	LogFile  string            `json:"logfile"`
}

func (me *AppSettings) Load(path string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	conf := AppSettings{}
	err = json.Unmarshal(text, &conf)
	if err != nil {
		return err
	}
	conf.SetMissingFieldDefaults()
	*me = conf
	return nil
}

func (me *AppSettings) SetMissingFieldDefaults() {
	if me.MQTT.Protocol == "" {
		me.MQTT.Protocol = "mqtts"
	}
	if me.MQTT.Port == 0 {
		me.MQTT.Port = 1883
	}
	if me.LogFile == "" {
		me.LogFile = "stdout"
	}
}

func (me *AppSettings) SetExampleValues() {
	me.MQTT = mqttnode.Settings{
		Protocol:       "mqtts (prefer) OR mqtt",
		BrokerIP:       "192.168.xxx.xxx|fe80::xxxx|DNS",
		Port:           1883,
		ClientID:       "tracker",
		AuthUser:       "broker-login-user",
		AuthPassword:   "broker-login-pass*****************",
		CAFile:         "conf/mqtts-authority-cert.pem",
		ClientCertFile: "conf/mqtts-with-client-certs/my-cert.pem",
		ClientKeyFile:  "conf/mqtts-with-client-certs/key-for-my-cert.pem",
		ClientKeyPass:  "**password-for-my-key-for-my-cert***",
		Topics:         []string{"#", "or/specific/topic1", "or/specific/topic2"},
	}
	me.Recorder = recorder.Settings{
		RootDirectory: "./data",
		TopicFilters: []string{
			"home/**/power",
			"plug?/energy",
			"switch/*/enable",
			"home/doors/**",
		},
	}
	me.LogFile = "stdout OR stderr OR file path"
}

func processCLIArgs() (bool, AppSettings, error) {
	// TODO: Long options don't seem to be on the menu in vanilla GO.
	var isVerboseShort bool
	flag.BoolVar(&isVerboseShort, "v", false, "Verbose logging of each received message.")
	var genExampleSettings bool
	flag.BoolVar(&genExampleSettings, "config-example", false, "Print a configuration example and exit.")
	var configFile string
	flag.StringVar(&configFile, "c", DEFAULT_CONFIG_FILE, "Config file path to use.")
	flag.Parse()

	var settings AppSettings

	if genExampleSettings {
		settings.SetExampleValues()
		jst, err := json.MarshalIndent(settings, "", " ")
		if err != nil {
			return false, settings, errors.New("failed to compose example config")
		} else {
			println(string(jst))
			os.Exit(0)
		}
	}

	err := settings.Load(configFile)
	if err != nil {
		return false, settings, err
	}

	return isVerboseShort, settings, nil
}

func main() {

	// Settings
	isverbose, settings, err := processCLIArgs()
	if err != nil {
		log.Fatal(err)
	}
	settings.SetMissingFieldDefaults()

	// Logging
	switch settings.LogFile {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	default:
		f, err := os.OpenFile(settings.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal("Failed to open log file: ", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}
	if isverbose {
		settings.Recorder.Verbose = true
	}

	// Module init
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	recorder := recorder.New(settings.Recorder)
	err = recorder.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer recorder.Close()

	node, err := mqttnode.Connect(&settings.MQTT)
	if err != nil {
		log.Fatal(err)
	}
	defer node.Disconnect()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Process loop
	for quit := false; !quit; {
		select {
		case data := <-node.Data:
			if isverbose {
				log.Print("Incoming: " + data.Topic() + " = " + string(data.Data()))
			}
			recorder.Write(data)
			continue
		case con := <-node.Connection:
			switch con.Type {
			case mqttnode.ConnectionEstablished:
				log.Print("Connected to broker")
			case mqttnode.SubscribeFailed:
				log.Print("Subscribe failed: ", con.Error.Error())
				quit = true
			case mqttnode.ConnectionLost:
				log.Print("Connection lost: ", con.Error.Error())
			}
			continue
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Println("Terminating due to TERM signal.")
			quit = true
			continue
		}
	}
}
