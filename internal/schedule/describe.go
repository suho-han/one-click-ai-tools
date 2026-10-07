package schedule

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// CurrentSchedule describes the persisted platform-scheduler state of one
// task. Interval is one of the ParseInterval results ("weekly", "daily",
// "12h", "6h", "1h") and is empty when the task is disabled or the interval
// cannot be recovered from the platform (hand-edited schedules, Windows).
// Hour (0-23) is only meaningful for weekly/daily.
type CurrentSchedule struct {
	Enabled  bool
	Interval string
	Hour     int
}

// Describe reads back what Enable last wrote, straight from the platform
// scheduler (launchd plist / crontab / schtasks), so callers can show the
// real state even when it was changed outside oct's config file.
func Describe(task Task) (CurrentSchedule, error) {
	s, err := GetScheduler()
	if err != nil {
		return CurrentSchedule{}, err
	}
	return s.Describe(task)
}

func (m *MacOS) Describe(task Task) (CurrentSchedule, error) {
	home, err := homeDirPath()
	if err != nil {
		return CurrentSchedule{}, err
	}
	data, err := os.ReadFile(launchAgentPath(home, m.LabelPrefix, task))
	if os.IsNotExist(err) {
		return CurrentSchedule{}, nil
	}
	if err != nil {
		return CurrentSchedule{}, fmt.Errorf("read launch agent: %w", err)
	}
	interval, hour, ok := parseLaunchdSchedule(data)
	if !ok {
		// The agent exists, so the task is scheduled; its interval just is not
		// one oct recognizes (e.g. a hand-edited plist).
		return CurrentSchedule{Enabled: true}, nil
	}
	return CurrentSchedule{Enabled: true, Interval: interval, Hour: hour}, nil
}

func (l *Linux) Describe(task Task) (CurrentSchedule, error) {
	out, err := linuxCrontabList()
	if err != nil {
		// Mirror Status: any crontab read failure counts as "not scheduled".
		return CurrentSchedule{}, nil
	}
	entry, ok := findCrontabEntry(string(out), task)
	if !ok {
		return CurrentSchedule{}, nil
	}
	interval, hour, ok := parseCronSchedule(entry)
	if !ok {
		return CurrentSchedule{Enabled: true}, nil
	}
	return CurrentSchedule{Enabled: true, Interval: interval, Hour: hour}, nil
}

func (w *Windows) Describe(task Task) (CurrentSchedule, error) {
	// schtasks /V output is locale-dependent, so only the enabled/disabled
	// bit is recoverable reliably.
	status, err := w.Status(task)
	if err != nil {
		return CurrentSchedule{}, err
	}
	return CurrentSchedule{Enabled: status == "enabled"}, nil
}

// parseLaunchdSchedule maps a rendered plist back to the interval/hour pair
// Enable wrote: StartInterval seconds for the N-hour cadences, a
// StartCalendarInterval dict (Weekday present = weekly) for weekly/daily.
// ok is false when neither key is present or the value is unrecognized.
func parseLaunchdSchedule(data []byte) (interval string, hour int, ok bool) {
	fields, err := parsePlistIntegers(data)
	if err != nil {
		return "", 0, false
	}
	if seconds := fields["StartInterval"]; seconds > 0 {
		switch seconds {
		case startIntervalSeconds(OneHourInterval):
			return OneHourInterval, 0, true
		case startIntervalSeconds(SixHourInterval):
			return SixHourInterval, 0, true
		case startIntervalSeconds(TwelveHourInterval):
			return TwelveHourInterval, 0, true
		default:
			return "", 0, false
		}
	}
	hourValue, hasHour := fields["Hour"]
	if !hasHour {
		return "", 0, false
	}
	if fields["Weekday"] > 0 {
		return WeeklyInterval, hourValue, true
	}
	return DailyInterval, hourValue, true
}

// parsePlistIntegers extracts every <key>integer</integer> pair from a small
// plist like the launch agent template renders (flat walk: the only nested
// dict is StartCalendarInterval, whose Hour/Minute/Weekday keys are wanted).
func parsePlistIntegers(data []byte) (map[string]int, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	fields := make(map[string]int)
	key := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fields, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "key" {
				key = strings.TrimSpace(readElementCharData(dec))
			} else if t.Name.Local == "integer" && key != "" {
				if n, err := strconv.Atoi(strings.TrimSpace(readElementCharData(dec))); err == nil {
					fields[key] = n
				}
				key = ""
			}
		}
	}
}

// readElementCharData consumes one element's character data (the decoder is
// positioned on the element's start tag).
func readElementCharData(dec *xml.Decoder) string {
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return b.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return b.String()
		}
	}
}

// findCrontabEntry returns the oct-managed crontab line for task.
func findCrontabEntry(crontab string, task Task) (string, bool) {
	marker := cronMarker(task)
	for _, line := range strings.Split(crontab, "\n") {
		if strings.Contains(line, marker) {
			return line, true
		}
	}
	return "", false
}

// parseCronSchedule maps the five-field cron expression Enable writes back to
// an interval/hour pair; ok is false for any other (e.g. hand-edited) shape.
func parseCronSchedule(entry string) (interval string, hour int, ok bool) {
	fields := strings.Fields(entry)
	if len(fields) < 5 {
		return "", 0, false
	}
	minute, dom, month, dow, hourField := fields[0], fields[2], fields[3], fields[4], fields[1]
	if dom != "*" || month != "*" {
		return "", 0, false
	}
	switch {
	case minute == "0" && hourField == "*/12":
		return TwelveHourInterval, 0, true
	case minute == "0" && hourField == "*/6":
		return SixHourInterval, 0, true
	case minute == "0" && hourField == "*":
		return OneHourInterval, 0, true
	case minute == "0" && dow == "1" && isCronHour(hourField):
		h, _ := strconv.Atoi(hourField)
		return WeeklyInterval, h, true
	case minute == "0" && dow == "*" && isCronHour(hourField):
		h, _ := strconv.Atoi(hourField)
		return DailyInterval, h, true
	default:
		return "", 0, false
	}
}

func isCronHour(field string) bool {
	h, err := strconv.Atoi(field)
	return err == nil && h >= 0 && h <= 23
}
