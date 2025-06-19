package recorder

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"mqttrack/fnmatch"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

const MaxNumRotateErrors uint = 20

type Record interface {
	Time() time.Time
	Topic() string
	Data() []byte
}

type Settings struct {
	RootDirectory    string   `json:"rootdir"`
	RotationFileSize uint     `json:"rotate_at_size"`
	GZipRotated      bool     `json:"gzip_rotated"`
	TopicFilters     []string `json:"filters"`
	Verbose          bool     `json:"-"`
}

type Recorder struct {
	settings        Settings
	cache           map[string]Record
	isopen          bool
	numRotateErrors atomic.Uint32
}

func New(settings Settings) Recorder {
	return Recorder{
		settings:        settings,
		cache:           make(map[string]Record),
		isopen:          false,
		numRotateErrors: atomic.Uint32{}, // Log spam prevention
	}
}

func (me *Recorder) rotate(filepath string) error {
	if me.settings.RotationFileSize <= 0 || uint(me.numRotateErrors.Load()) > MaxNumRotateErrors {
		return nil
	}

	if st, err := os.Stat(filepath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else {
			return err
		}
	} else if !st.Mode().IsRegular() {
		me.numRotateErrors.Add(1)
		return fmt.Errorf("record file unexpectedly not a file: %s", filepath)
	} else if st.Size()/1024 < int64(me.settings.RotationFileSize) {
		return nil
	}

	if ls, err := os.ReadDir(path.Dir(filepath)); err != nil {
		me.numRotateErrors.Add(1)
		return fmt.Errorf("reading directory for record rotating failed: %s", path.Dir(filepath))
	} else {
		rotindex := 0
		re := regexp.MustCompile("^" + regexp.QuoteMeta(path.Base(filepath)) + "\\.(\\d+)(\\.gz)?")
		for _, fp := range ls {
			if !fp.Type().IsRegular() {
				continue
			}
			match := re.FindStringSubmatch(fp.Name())
			if match == nil {
				continue
			} else {
				ext, _ := strconv.Atoi(match[1]) // String guaranteed digits
				if ext > rotindex {
					rotindex = ext
				}
			}
		}
		rotindex += 1
		newpath := fmt.Sprintf("%s.%d", filepath, rotindex)
		me.logVerbose("Rotating: ", filepath, "->", newpath)
		if err := os.Rename(filepath, newpath); err != nil {
			me.numRotateErrors.Add(1)
			return fmt.Errorf("renaming record file failed %s->%s: %s", filepath, newpath, err.Error())
		}
		lastrec := fmt.Sprintf("%s.%d", filepath, rotindex-1)
		if st, err := os.Stat(lastrec); err != nil {
			return nil
		} else if st.Mode().IsRegular() {
			if err := me.gzip(lastrec); err != nil {
				return err
			}
		}
	}

	return nil
}

func (me *Recorder) filter(topic string) bool {
	if len(me.settings.TopicFilters) == 0 {
		return true
	}
	for _, pattern := range me.settings.TopicFilters {
		if fnmatch.Match(pattern, topic, fnmatch.FNM_NOESCAPE) {
			return true
		}
	}
	return false
}

func (me *Recorder) gzip(filepath string) error {
	if !me.settings.GZipRotated || uint(me.numRotateErrors.Load()) > MaxNumRotateErrors {
		return nil
	}
	me.logVerbose("GZipping ", filepath)
	cmd := exec.Command("gzip", "-9", filepath)
	if cmd.Err != nil {
		me.numRotateErrors.Add(1)
		return cmd.Err
	}
	go func() {
		if err := cmd.Run(); err != nil {
			me.numRotateErrors.Add(1)
		}
	}()
	return nil
}

func (me *Recorder) logVerbose(v ...any) {
	if me.settings.Verbose {
		log.Print(v...)
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
	} else {
		me.logVerbose("Recorder opened.")
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

	if !me.filter(topic) {
		me.logVerbose("Topic filtered out: ", topic)
		return nil
	}

	ct := me.cache[topic]
	unchanged := ct != nil && bytes.Equal(ct.Data(), data.Data())
	me.cache[topic] = data
	if unchanged {
		// Todo: Minimal interval to log also same values.
		// Todo: Maybe floating point compare with threshold?
		me.logVerbose("Topic unchanged: " + topic)
		return nil
	}

	filePath := path.Join(me.settings.RootDirectory, topic)
	dir := path.Dir(filePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create topic directory '%s': %s", dir, err.Error())
	}

	if err := me.rotate(filePath); err != nil {
		log.Print(err.Error())
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
		me.logVerbose("Topic data need newline escaping: " + topic)
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
	if n, err := fos.WriteString(s); err != nil {
		return fmt.Errorf("failed to write topic file '%s', %s", topic, err.Error())
	} else if n != len(s) {
		return fmt.Errorf("failed to write all bytes of topic file '%s'", topic)
	}

	return nil
}
