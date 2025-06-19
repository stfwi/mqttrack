package mqttnode

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ConnectionEventType int

const (
	ConnectionEstablished ConnectionEventType = iota
	ConnectionLost
	SubscribeFailed
)

type ConnectionEvent struct {
	Time  time.Time
	Type  ConnectionEventType
	Error error
}

type DataEvent struct {
	time  time.Time
	topic string
	data  []byte
}

func (me DataEvent) Time() time.Time {
	return me.time
}

func (me DataEvent) Topic() string {
	return me.topic
}

func (me DataEvent) Data() []byte {
	return me.data
}

type Settings struct {
	Protocol       string   `json:"protocol"`
	BrokerIP       string   `json:"broker_ip"`
	Port           uint16   `json:"port"`
	ClientID       string   `json:"client_id"`
	AuthUser       string   `json:"auth_user"`
	AuthPassword   string   `json:"auth_password"`
	CAFile         string   `json:"ca_cert_file"`
	ClientCertFile string   `json:"client_cert_file"`
	ClientKeyFile  string   `json:"client_key_file"`
	ValidateCerts  bool     `json:"validate_certs"`
	Topics         []string `json:"topics"`
}

type Node struct {
	settings   Settings
	client     mqtt.Client
	Data       chan DataEvent
	Connection chan ConnectionEvent
}

func (me *Node) getClientOptions(settings *Settings) (*mqtt.ClientOptions, error) {
	var isTls = false
	var proto = ""
	switch strings.ToLower(settings.Protocol) {
	case "mqtt":
		proto = "mqtt"
		isTls = false
	case "mqtts":
		proto = "mqtts"
		isTls = true
	case "ws":
		proto = "ws"
		isTls = false
		return nil, fmt.Errorf("invalid protocol setting '%s', web sockets are not supported yet, sorry", settings.Protocol)
	case "wss":
		proto = "wss"
		isTls = true
		return nil, fmt.Errorf("invalid protocol setting '%s', web sockets are not supported yet, sorry", settings.Protocol)
	default:
		return nil, fmt.Errorf("invalid protocol setting '%s', allowed are 'mqtt', 'mqtts'", settings.Protocol)
	}
	if settings.Port == 0 {
		return nil, fmt.Errorf("invalid port setting '%d', normally used are 1883 (mqtt) or 8883 (with cerificate checks) are 'mqtt', 'mqtts'", settings.Port)
	}
	clientId := ""
	if settings.AuthUser != "" && settings.ClientID == "" {
		clientId = settings.AuthUser
	}

	var opts *mqtt.ClientOptions = mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("%s://%s:%d", proto, settings.BrokerIP, settings.Port))
	opts.SetClientID(clientId)
	opts.SetUsername(settings.AuthUser)
	opts.SetPassword(settings.AuthPassword)
	if isTls {
		var rootcas *x509.CertPool = nil
		if settings.CAFile != "" {
			if ca, err := os.ReadFile(settings.CAFile); err != nil {
				log.Fatalln("CA file: ", err.Error())
			} else {
				rootcas = x509.NewCertPool()
				rootcas.AppendCertsFromPEM(ca)
			}
		}

		var certs []tls.Certificate = nil
		if settings.ClientCertFile != "" {
			if settings.ClientKeyFile == "" {
				log.Fatalln("Invalid TLS config: If a client certificate is specified, the key file for it must also be given.")
			} else if cert, err := tls.LoadX509KeyPair(settings.ClientCertFile, settings.ClientKeyFile); err != nil {
				log.Fatalln("Client certificate:", err.Error())
			} else {
				certs = []tls.Certificate{cert}
			}
		}

		// @todo: This needs review from a TLS Pro, not sure if I'm
		// doing that right.
		clientauth := tls.NoClientCert
		if settings.ValidateCerts {
			clientauth = tls.RequireAndVerifyClientCert
		}
		tlsc := tls.Config{
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
			RootCAs:      rootcas,
			ClientCAs:    rootcas,
			Certificates: certs,
			ClientAuth:   clientauth,
		}

		opts = opts.SetTLSConfig(&tlsc)
	}

	return opts, nil
}

func (me *Node) subscribeTo(topic string, qos byte) error {
	if token := me.client.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
		me.Data <- DataEvent{
			time:  time.Now(),
			topic: msg.Topic(),
			data:  msg.Payload(),
		}
	}); token.Wait() && token.Error() != nil {
		me.Connection <- ConnectionEvent{
			Time:  time.Now(),
			Type:  SubscribeFailed,
			Error: errors.New("failed to subscribe to topic " + topic + ":" + token.Error().Error()),
		}
		return token.Error()
	}
	return nil
}

func Connect(settings *Settings) (Node, error) {
	me := Node{
		settings:   *settings,
		client:     nil,
		Data:       make(chan DataEvent),
		Connection: make(chan ConnectionEvent),
	}

	opts, err := me.getClientOptions(settings)
	if err != nil {
		return me, err
	}

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		me.Connection <- ConnectionEvent{
			Time:  time.Now(),
			Type:  ConnectionEstablished,
			Error: nil,
		}
		if len(me.settings.Topics) == 0 {
			me.subscribeTo("#", 0)
		} else {
			for _, topic := range me.settings.Topics {
				me.subscribeTo(topic, 0)
			}
		}
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		me.Connection <- ConnectionEvent{
			Time:  time.Now(),
			Type:  ConnectionLost,
			Error: err,
		}
	})

	me.client = mqtt.NewClient(opts)
	if token := me.client.Connect(); token.Wait() && token.Error() != nil {
		return me, fmt.Errorf("failed to connect: %s", token.Error())
	}
	return me, nil
}

func (me *Node) Disconnect() {
	if me.client == nil {
		return
	}
	if !me.client.IsConnected() {
		me.client.Disconnect(0) // could be connecting or reconnecting at the moment.
	} else {
		token := me.client.Unsubscribe("#")
		token.WaitTimeout(150)
		me.client.Disconnect(250)
	}
}
