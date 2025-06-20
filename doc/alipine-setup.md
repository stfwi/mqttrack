
# MQTTrack Service Setup on ALPINE RPi5

Documentation record for later generation of install script etc.

Root here `/opt/mqttracker`, conf target later in `/etc/mqttrack`.

  ```sh
    /opt/mqttracker/
    ├── conf
    │   ├── mqtt-ca.pem
    │   ├── mqttrack.json
    │   ├── tracker.key
    │   └── tracker.pem
    ├── data -> /mnt/persistent/mqttrack
    └── mqttrack
  ```

***Permissions:***

  ```sh
  root@r5:/opt/mqttracker/conf# ll
  total 16K
  drwxrwxr-x    2 root     root         120 Jun 20 00:38 ./
  drwxrwxr-x    3 root     root         100 Jun 20 00:57 ../
  -r--------    1 mqttrack mqttrack    1.9K Jun 19 23:12 mqtt-ca.pem
  -rw-r--r--    1 root     root         621 Jun 20 00:26 mqttrack.json
  -r--------    1 mqttrack mqttrack    1.7K Jun 19 23:12 tracker.key
  -r--------    1 mqttrack mqttrack    1.6K Jun 19 23:12 tracker.pem
  ```

***Manual setup:***

  ```sh
  #!/bin/sh
  set -e
  [ "$(whoami)" == "root" ]
  mkdir -p /opt/mqttracker
  addgroup -S mqttrack
  adduser -S -G mqttrack -h /opt/mqttracker/data mqttrack
  cd /opt/mqttracker/conf
  chown mqttrack:mqttrack mqtt-ca.pem tracker.key tracker.pem
  chmod 400 mqtt-ca.pem tracker.key tracker.pem
  mkdir -p /mnt/persistent/mqttrack
  chown -R mqttrack:mqttrack /mnt/persistent/mqttrack
  chmod 755 /mnt/persistent/mqttrack
  ln -s /mnt/persistent/mqttrack /opt/mqttracker/data
  ```

***MQTTrack config on Alpine, mosquitto on localhost listening***

  ```jsonc
  {
  "mqtt": {
    "protocol": "mqtts",
    "broker_ip": "127.0.0.1",
    "port": 1883,
    "client_id": "tracker-cid",
    "auth_user": "tracker",
    "auth_password": "*********************",
    "ca_cert_file": "/opt/mqttracker/conf/mqtt-ca.pem",
    "client_cert_file": "/opt/mqttracker/conf/tracker.pem",
    "client_key_file": "/opt/mqttracker/conf/tracker.key",
    "validate_cert": false, // localhost
    "topics": [
      "#"
    ]
  },
  "recorder": {
    "rootdir": "/opt/mqttracker/data",
    "rotate_at_size": 1024,
    "gzip_rotated": true,
    "filters": [
    ]
  },
  "logfile": "stdout"
  }
  ```

***Open RC service file***

  ```sh
  #!/sbin/openrc-run
  # MQTTrack service
  supervisor=supervise-daemon
  description="MQTTrack CSV recorder service"

  command="/opt/mqttracker/mqttrack"
  command_args="-c /opt/mqttracker/conf/mqttrack.json "
  command_user="mqttrack:mqttrack"
  output_logger="/usr/bin/logger"

  respawn_max=5
  respawn_period=5
  respawn_delay=1s

  depend() {
    want net
    want mosquitto
  }
  ```
