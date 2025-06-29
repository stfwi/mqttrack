package recorder

import (
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

//------------------------------------------------------------------------

type TestRecord struct {
	TimeVal  time.Time
	TopicVal string
	DataVal  []byte
}

func (me TestRecord) Time() time.Time {
	return me.TimeVal
}

func (me TestRecord) Topic() string {
	return me.TopicVal
}

func (me TestRecord) Data() []byte {
	return me.DataVal
}

//------------------------------------------------------------------------

func mktestroot() (string, func()) {
	root := path.Join(os.TempDir(), fmt.Sprintf("_recorder_test-%d.tmp", rand.Int64()))
	if _, err := os.Stat(root); err == nil {
		panic("Random directory already existing unexpectedly:" + root)
	}
	if err := os.Mkdir(root, 0755); err != nil {
		panic("Failed to create test output directory (unexpectedly):" + root)
	}
	fmt.Printf("Created test dir: '%s'\n", root)
	return root, func() {
		fmt.Printf("Removing test dir: '%s'\n", root)
		os.RemoveAll(root)
	}
}

//------------------------------------------------------------------------

var tt time.Duration = 0

const tincr_ms time.Duration = time.Millisecond * 100

func mktime(increment bool) time.Time {
	if increment {
		tt = tt + tincr_ms
	}
	return time.Date(2020, time.January, 1, 1, 0, 0, 0, time.UTC).Add(tt)
}

func mkrecord(topic string, data string) TestRecord {
	return TestRecord{
		TimeVal:  mktime(true),
		DataVal:  []byte(data),
		TopicVal: topic,
	}
}

func readback_csv(test *testing.T, root string, topic string) []string {
	txt, err := os.ReadFile(path.Join(root, topic))
	if err != nil || txt == nil {
		return make([]string, 0)
	}
	lines := strings.Split(strings.TrimRight(string(txt), "\n"), "\n")
	re := regexp.MustCompile(`^[\d\.]+,.*$`)
	for _, line := range lines {
		if !re.Match([]byte(line)) {
			test.Errorf("Line does not match 'timestamp,data': '%s'", line)
		}
	}
	return lines
}

var /*const*/ invalidtime time.Time = time.UnixMilli(-1)

const difftdefault time.Duration = -1
const difftdontcare time.Duration = -2

func parsetime(txt string) time.Time {
	if v, err := strconv.ParseFloat(txt, 64); err != nil {
		return invalidtime
	} else {
		return time.UnixMilli(int64(v * 1000))
	}
}

func writereadbacklast(t *testing.T, rec *Recorder, topic string, value string, maxdifft time.Duration) []string {
	if err := rec.Write(mkrecord(topic, value)); err != nil {
		t.Errorf("Unexpected write fail: %v\n", err)
		return nil
	}
	lines := readback_csv(t, rec.settings.RootDirectory, topic)
	if len(lines) < 1 {
		t.Errorf("Expected at least one output csv line in '%s', got %d", topic, len(lines))
		return nil
	}
	line := lines[len(lines)-1]
	fields := strings.SplitN(line, ",", 2)
	if len(fields) != 2 {
		t.Errorf("Expected two csv fields in '%s', got '%s'", topic, line)
		return nil
	}
	trecord := mktime(false) // same time as mkrecord
	tsaved := parsetime(fields[0])

	if maxdifft != difftdontcare {
		if maxdifft < 0 {
			maxdifft = tincr_ms * 2
		}
		dt := trecord.Sub(tsaved)
		if math.Abs(float64(dt.Milliseconds())) > float64(maxdifft.Milliseconds()) {
			t.Errorf("Storage time deviation too high, %d vs %d, diff=%dms", trecord.UnixMilli(), tsaved.UnixMilli(), dt)
			return nil
		} else {
			log.Printf("Storage time deviation OK: %dms (max=%dms)", dt.Milliseconds(), maxdifft.Milliseconds())
		}
	}
	unescaped := strings.ReplaceAll(fields[1], "\\n", "\n")
	if unescaped != value {
		t.Errorf("Storage time deviation too high, %d vs %d", trecord.UnixMilli(), tsaved.UnixMilli())
		return nil
	}
	// log.Printf("OK WRB: '%s'=>'%s'", topic, value)
	return lines
}

func isfile(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.Mode().Perm().IsRegular()
}

//------------------------------------------------------------------------

func TestUninitialized(t *testing.T) {
	root, cleaner := mktestroot()
	defer cleaner()

	// No root dir
	{
		rec := New(Settings{
			RootDirectory: "",
		})
		if err := rec.Open(); err == nil {
			t.Fatal("Expected error for unspecified data root.")
		}
	}

	// Root dir not existing
	{
		rec := New(Settings{
			RootDirectory: path.Join(root, "NONEXISTONG"),
		})
		if err := rec.Open(); err == nil {
			t.Fatal("Expected error for not existing data root.")
		}
	}

	// Root dir not a dir
	{
		rec := New(Settings{
			RootDirectory: path.Join(root, "NONEXISTONG"),
		})
		os.WriteFile(rec.settings.RootDirectory, []byte("NODIR"), 0644)
		if err := rec.Open(); err == nil {
			t.Fatal("Expected error for not existing data root.")
		}
	}

	// Write without open
	{
		rec := New(Settings{
			RootDirectory: root,
		})
		defer func() {
			if r := recover(); r != nil {
				log.Println("OK: Uninitialized write paniced")
			} else {
				t.Fatal("Uninitialized write did not panic")
			}
		}()
		if err := rec.Write(mkrecord("/valid-record", "valid-value-but-not-opened")); err != nil {
			t.Fatal("Recorder not opened yet, should have paniced.")
		}
	}
}

func TestNormalWrite(t *testing.T) {
	root, cleaner := mktestroot()
	defer cleaner()

	rec := New(Settings{
		RootDirectory: root,
		Verbose:       true,
		TopicFilters: []string{
			"valid*",
			"home/**",
		},
	})
	if err := rec.Open(); err != nil {
		t.Fatal("Recorder open failed (unexpected): ", err)
	}
	defer rec.Close()

	if lines := writereadbacklast(t, &rec, "/valid-record", "valid-string", difftdefault); len(lines) > 0 {
		log.Printf("OK Initial /valid-record")
	}
	if lines := writereadbacklast(t, &rec, "/valid-record", "valid-string", difftdontcare); len(lines) > 0 {
		if len(lines) != 1 {
			t.Errorf("Expected omitting unchanged value write")
		} else {
			log.Printf("OK unchanged value write omitted")
		}
	}
	if lines := writereadbacklast(t, &rec, "/valid-record-withnewlinesescape", "valid\nstring\n", difftdontcare); len(lines) > 0 {
		if len(lines) != 1 {
			t.Errorf("Expected omitting unchanged value write")
		}
	}

	if err := rec.Write(mkrecord("../invalid-topic/../../../../systemfile", "hackit")); err == nil {
		t.Errorf("Expected error for topic containing ../")
	}

	if err := rec.Write(mkrecord("", "valid-value")); err == nil {
		t.Errorf("Expected error for empty topic path")
	}

	if err := rec.Write(mkrecord("not-in-filter", "valid-value")); err != nil {
		t.Errorf("Unexpected for filtered out topic")
	} else if isfile(path.Join(root, "not-in-filter")) {
		t.Errorf("Unexpected topic 'not-in-filter' to be filtered")
	}

	if err := os.Mkdir(path.Join(root, "valid-already-directory"), 0755); err != nil {
		t.Errorf("Failed to create test case directory")
	} else if err := rec.Write(mkrecord("valid-already-directory", "valid-value")); err == nil {
		t.Errorf("Expected for writing a file that is already a directory")
	}

	if err := os.WriteFile(path.Join(root, "valid-already-file"), []byte("ALREADYFILE"), 0644); err != nil {
		t.Errorf("Failed to create test case file")
	} else if err := rec.Write(mkrecord("valid-already-file/topic", "valid-value")); err == nil {
		t.Errorf("Expected for writing a file that is located in a directory that is already a file (that is that is that is)")
	}

}

func TestRotate(t *testing.T) {
	root, cleaner := mktestroot()
	defer cleaner()

	rec := New(Settings{
		RootDirectory:    root,
		RotationFileSize: 1,
		GZipRotated:      true,
		TopicFilters:     nil,
		Verbose:          true,
	})
	rec.Open()
	defer rec.Close()

	os.Mkdir(path.Join(root, "topic-subdir1"), 0755)
	os.Mkdir(path.Join(root, "topic-subdir2"), 0755)

	tp := path.Join(root, "long-string")

	// First, file written, but no rotation yet
	{
		line1kb := strings.Repeat("1234567890", 1024+1)
		if lines := writereadbacklast(t, &rec, "/long-string", line1kb, difftdefault); len(lines) > 0 {
			if isfile(tp + ".1") {
				t.Errorf("File not supposed to be rotated yet: %s", tp+".1")
				return
			}
			if !isfile(tp) {
				t.Errorf("File unexpectedly not written: %s", tp)
				return
			}
			log.Printf("OK File written: %s\n", tp)
		}
	}

	// Second, file written, and previously rotated (old -> <path>.1)
	{
		line1kb := strings.Repeat("1234567890", 1024+2) // +2 -> different value
		if lines := writereadbacklast(t, &rec, "/long-string", line1kb, difftdefault); len(lines) > 0 {
			if !isfile(tp + ".1") {
				t.Errorf("File supposed to be rotated: %s", tp+".1")
				return
			}
			log.Printf("OK File rotated: %s -> %s\n", tp, tp+".1")
			if !isfile(tp) {
				t.Errorf("File unexpectedly not written: %s", tp)
				return
			}
			log.Printf("OK File written: %s\n", tp)
		}
	}

	// Third, file written, rotated (old -> <path>.2), and previous file zipped.
	{
		line1kb := strings.Repeat("1234567890", 1024+3) // +3 -> different value
		if lines := writereadbacklast(t, &rec, "/long-string", line1kb, difftdefault); len(lines) > 0 {
			for i := 1; i < 1000; i++ {
				time.Sleep(time.Millisecond)
				if !zipping.Load() {
					break
				}
			}
			ls := "Files:"
			if des, err := os.ReadDir(root); err == nil {
				for _, de := range des {
					ls = ls + " " + de.Name()
				}
			}
			log.Println(ls)
			if !isfile(tp) {
				t.Errorf("File unexpectedly not written: %s", tp)
				return
			}
			log.Printf("OK File written: %s\n", tp)
			if !isfile(tp + ".2") {
				t.Errorf("File supposed to be rotated: %s", tp+".2")
				return
			}
			log.Printf("OK File rotated: %s --> %s\n", tp, tp+".2")
			if !isfile(tp + ".1.gz") {
				t.Errorf("File supposed to be zipped: %s", tp+".1")
				return
			}
			log.Printf("OK File zipped: %s\n", tp+".1.gz")
		}
	}

	// Omit GZIP if not true in settings
	{
		rec.settings.GZipRotated = false
		fn := path.Join(root, "long-string.3")
		if err := rec.rotate(path.Join(root, "long-string")); err != nil {
			t.Errorf("Unexpected error for gzipping a existing file: %s", fn)
		}
		if !isfile(fn) || isfile(fn+".gz") {
			t.Errorf("Expected not zipping file, as not in config: %s", fn)
		}
		rec.settings.GZipRotated = true
	}

	// Plain rotate checks
	{
		if err := rec.rotate(path.Join(root, "nonexisting-file")); err != nil {
			t.Error("Expected ignoring rotate non-existing files")
		}
		os.Mkdir(path.Join(root, "nonexisting-file-but-dir"), 0755)
		if err := rec.rotate(path.Join(root, "nonexisting-file-but-dir")); err == nil {
			t.Error("Expected error rotate for rotating a regular file that is a dir")
		}
	}

	// Below rotate file size
	{
		line1kb := strings.Repeat("1234567890", 1)
		if lines := writereadbacklast(t, &rec, "/long-string", line1kb, difftdefault); len(lines) > 0 {
			fn := path.Join(root, "long-string")
			if err := rec.rotate(fn); err != nil {
				t.Errorf("Unexpected rotate error: %s", err.Error())
			} else if isfile(fn + ".4") {
				t.Errorf("Should not rotate yet, file size threshold not reached")
			}
		}
	}
}

//------------------------------------------------------------------------
