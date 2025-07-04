# MQTTrack - CSV File Tree Topic Recorder

![CI](https://github.com/stfwi/mqttrack/actions/workflows/ci-build.yml/badge.svg)


Simple tracker/recorder for MQTT message data.

  - Stores MQTT topic payloads with timestamps (CSV format `timestamp,data`) in singulated
    files for each topic.

  - Creates a directory structure according to the topic paths.

  - Updates the tracking files on value change only (and on startup)

  - Optionally rotates and archives (gzip) CSV files depending on file size settings.

  - Optionally allows `fnmatch` wildcard filtering in addition to the MQTT subscription selection.

  - JSON config file format to facilitate API based config changes.

### Building and Depencencies

  - Dep: Minimal GO version go1.24.2.
  - Dep: Optional GNU Make
  - Dep: `eclipse/paho.mqtt.golang`, `gorilla/websocket`
  - Dep: `baulk/bloat/tree/master/utils/fnmatch`
  - Dep: `tidwall/jsonc`

  - Build: `cd src && go build` or `make dist`

### Usage

  - Chose and create a root directory for the data tracking files.
  - Create a config JSON file (to generate an example: `--config-example`)
  - Run the service, specifying the config file `mqttrack -c <config file path>`
  - Verbose output (to see the incoming messages use `mqttrack -v -c ....`)

### Example Config

  ```jsonc
  {
    "mqtt": {
      // Connection
      "protocol": "mqtts (prefer) OR mqtt",
      "broker_ip": "192.168.xxx.xxx|fe80::xxxx|DNS",
      "port": 1883,
      // MQTT ID and broker authentication
      "client_id": "tracker",
      "auth_user": "broker-login-user",
      "auth_password": "broker-login-pass*****************",
      // Certificates
      "ca_cert_file": "conf/mqtts-authority-cert.pem",
      "client_cert_file": "conf/mqtts-with-client-certs/my-cert.pem",
      "client_key_file": "conf/mqtts-with-client-certs/key-for-my-cert.pem",
      "validate_certs": false,
      // Subscriptions
      "topics": [
        "#", // <-- default everything if no topics specified
        "or/specific/topic1",
        "or/specific/topic2"
      ]
    },
    "recorder": {
      // Data storage path, rotate at 1MB file size,
      // compress previous record archive file.
      "rootdir": "./data",
      "rotate_at_size": 1024,
      "gzip_rotated": true,
      // Recorder filters using `fnmatch` patterns
      // (extended wildcards). Prefer a good subscription
      // setting first to reduce unnecessary load.
      "filters": [
        "home/**/power",
        "plug?/energy",
        "switch/*/enable",
        "home/doors/**"
      ],
    },
    // Logging
    "logfile": "stdout OR stderr OR file path"
  }
  ```

### Example output directory structure and record file

This structure was created by the application for the MQTT topics
`plug1/current`, `plug1/enable` ...

  ```sh
  data
  ├── plug1
  │   ├── current
  │   ├── enable
  │   ├── energy
  │   ├── online
  │   └── power
  ├── plug2
  │   ├── current
  │   ├── enable
  │   ├── energy
  │   ├── online
  │   └── power
  └── plug4
      ├── current
      ├── enable
      ├── energy
      ├── online
      └── power
  ```

The data format looks as in e.g. `plug2/power`:

  ```csv
  1750280696.01,159
  1750282195.96,128.2
  1750284220.89,172
  1750284235.89,164.7
  1750284250.90,124.4
  1750284265.89,123.3
  1750284280.89,125.7
  1750284295.89,126.7
  1750284310.89,126.8
  1750284325.89,122.6
  1750284340.89,124.8
  1750284355.89,128.3
  ```

### Code Quality

- *This is a first GO learning project. Later refactorings are likely.*

73 .-.-.-
