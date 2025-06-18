package recorder

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"unicode"
)

type Record interface {
	Time() time.Time
	Topic() string
	Data() []byte
}

type Settings struct {
	RootDirectory string `json:"rootdir"`
}

type Recorder struct {
	settings Settings
	cache    map[string]Record
	isopen   bool
}

func New(settings Settings) Recorder {
	return Recorder{
		settings: settings,
		cache:    make(map[string]Record),
		isopen:   false,
	}
}

func (me *Recorder) Open() error {
	dir := me.settings.RootDirectory
	if dir == "" {
		return fmt.Errorf("output root directory of the recorder is not set")
	} else if st, err := os.Stat(dir); err != nil {
		return fmt.Errorf("data root directory does not exist or not accessible: %s", dir)
	} else if !st.IsDir() {
		return fmt.Errorf("data root is not a directory: %s", dir)
	}
	me.isopen = true
	return nil
}

func (me *Recorder) Close() {
	me.isopen = false
	me.cache = make(map[string]Record)
}

func (me *Recorder) Write(data Record) error {
	if !me.isopen {
		panic("Recorder not initialized")
	}

	topic := strings.Trim(data.Topic(), "/.")
	if topic == "" || strings.Contains(topic, "..") || strings.ContainsFunc(topic, func(ch rune) bool {
		return !unicode.IsPrint(ch) || unicode.IsControl(ch)
	}) {
		return fmt.Errorf("invalid topic path: '%s'", topic)
	}

	ct := me.cache[topic]
	unchanged := ct != nil && bytes.Equal(ct.Data(), data.Data())
	me.cache[topic] = data
	if unchanged {
		// Todo: Minimal interval to log also same values.
		// Todo: Maybe floating point compare with threshold?
		return nil
	}

	filePath := path.Join(me.settings.RootDirectory, topic)
	dir := path.Dir(filePath)

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create topic directory '%s': %s", dir, err.Error())
	}

	fos, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed write topic file '%s': %s", filePath, err.Error())
	}
	defer fos.Close()

	ts := float64(data.Time().UnixMilli()) * 1e-3
	da := data.Data()
	if bytes.ContainsAny(da, "\n") {
		// Todo: There must be a better way for this, something like array.map() with insert option,
		// -> slices.Whatever()?
		dac := make([]byte, 0, len(da)*2)
		for _, v := range da {
			if v != '\n' {
				dac = append(dac, v)
			} else {
				dac = append(dac, '\\')
				dac = append(dac, 'n')
			}
		}
		da = dac
	}

	s := fmt.Sprintf("%13.2f,%s\n", ts, da)
	n, err := fos.WriteString(s)
	if err != nil {
		return fmt.Errorf("failed to write topic file '%s', %s", topic, err.Error())
	} else if n != len(s) {
		return fmt.Errorf("failed to write all bytes of topic file '%s'", topic)
	}

	return nil
}
