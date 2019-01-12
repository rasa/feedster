package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	durationMask = "%02d:%02d:%02d"
	// mininum duration in order to be considered valid
	minMilliseconds = 1000
	minFileSize     = 1024
	skipRegex       = "^(avoid|bypass|circumvent|dodge|duck|forget|hide|ignore|neglect|no|omit|overlook|pass|quit|reject|sidestep|shirk|skirt|skip|x)$"
)

// Track contains the MP3 tags to be updated
type Track struct { // Our example struct, you can use "-" to ignore a field
	Filename         string `csv:"filename"`
	AlbumArtist      string `csv:"album_artist,omitempty"`
	AlbumTitle       string `csv:"album_title,omitempty"`
	Artist           string `csv:"artist,omitempty"`
	Composer         string `csv:"composer,omitempty"`
	Copyright        string `csv:"copyright,omitempty"`
	Description      string `csv:"description,omitempty"` // Item.Description
	DiscNumber       string `csv:"disc_number,omitempty"`
	Genre            string `csv:"genre,omitempty"`
	Track            string `csv:"track,omitempty"`
	Subtitle         string `csv:"subtitle,omitempty"` // Item.ISubtitle
	Summary          string `csv:"summary,omitempty"`  // Item.ISummary
	Title            string `csv:"title,omitempty"`    // Item.Title
	Year             string `csv:"year,omitempty"`
	OriginalFilename string
	// DurationMilliseconds is determined by running exiftool or ffprobe on filename
	DurationMilliseconds int64
	// FileSize is the file's size via os.Stat()
	FileSize         int64
	OriginalFileSize int64
	// OriginalModTime is the nanoseconds of the last mod time via os.Stat()
	OriginalModTime int64
	// ModTime is the nanoseconds of the last mod time (less duration) via os.Stat()
	ModTime   int64
	Processed bool
}

// Fields returns a map of csv field names to field values
func (f *Track) Fields() map[string]string {
	val := reflect.ValueOf(f).Elem()
	rv := make(map[string]string)

	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		tag := typeField.Tag
		csv := tag.Get("csv")
		if csv == "" {
			continue
		}
		s := strings.Split(csv, ",")
		fieldName := s[0]
		valueField := val.Field(i)
		rv[fieldName] = fmt.Sprintf("%v", valueField.Interface())
	}
	return rv
}

// Get a field value for the fieldName
func (f *Track) Get(fieldName string) string {
	val := reflect.ValueOf(f).Elem()
	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		tag := typeField.Tag
		csv := tag.Get("csv")
		if csv == "" {
			continue
		}
		s := strings.Split(csv, ",")
		if !strings.EqualFold(s[0], fieldName) {
			continue
		}
		valueField := val.Field(i)
		return fmt.Sprintf("%v", valueField.Interface())
	}
	return ""
}

// Set the fieldName to the fieldValue
func (f *Track) Set(fieldName string, fieldValue string) bool {
	val := reflect.ValueOf(f).Elem()
	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		tag := typeField.Tag
		csv := tag.Get("csv")
		if csv == "" {
			continue
		}
		s := strings.Split(csv, ",")
		if !strings.EqualFold(s[0], fieldName) {
			continue
		}
		fieldValue = strings.TrimSpace(fieldValue)
		val.Field(i).SetString(fieldValue)
		return true
	}
	return false
}

// IsValid returns true if the track should be processed/exported
func (f *Track) IsValid() bool {
	return f.Error() == nil
}

// Error returns the error description, if the track is not valid
func (f *Track) Error() (err error) {
	if f.Filename == "" {
		return fmt.Errorf("Filename is empty")
	}
	if f.Title != "" {
		matched, err := regexp.MatchString(skipRegex, f.Title)
		if err == nil && matched {
			if f.Artist == "" && f.Description == "" && f.Track == "" && f.DiscNumber == "" &&
				f.AlbumTitle == "" && f.Genre == "" && f.AlbumArtist == "" && f.Summary == "" &&
				f.Copyright == "" && f.Composer == "" && f.Year == "" {
				return fmt.Errorf("File is marked to be skipped")
			}
		}
	}
	if !f.Processed {
		return nil
	}
	if f.ModTime == 0 {
		return fmt.Errorf("File does not exist, or is unreadable")
	}
	if f.DurationMilliseconds > 0 && f.DurationMilliseconds < minMilliseconds {
		return fmt.Errorf("File is only %d milliseconds in duration, >=%d required", f.DurationMilliseconds, minMilliseconds)
	}
	if f.FileSize < minFileSize {
		return fmt.Errorf("File is only %d bytes, >=%d is required", f.FileSize, minFileSize)
	}
	return nil
}

// Duration returns the duration in hh:mm:ss format
func (f *Track) Duration() string {
	milliseconds := f.DurationMilliseconds
	millis := milliseconds % 1000
	milliseconds /= 1000
	seconds := milliseconds % 60
	milliseconds /= 60
	minutes := milliseconds % 60
	milliseconds /= 60
	hours := milliseconds
	if millis > 0 {
		seconds++
	}
	if seconds > 59 {
		minutes++
		seconds = 0
	}
	if minutes > 59 {
		hours++
		minutes = 0
	}
	return fmt.Sprintf(durationMask, hours, minutes, seconds)
}

// NormalizeFilename normalizes the filename
func (f *Track) NormalizeFilename() {
	if f.OriginalFilename > "" {
		return
	}
	f.OriginalFilename = f.Filename
	f.Filename = normalizeFilename(f.Filename)
}

// SetCopyright sets the copyright string
func (f *Track) SetCopyright(copyright string, copyrightMask string, year int) {
	if copyright != "" {
		// f.Copyright := html.EscapeString(copyright)
		f.Copyright = copyright
	} else {
		// f.Copyright = fmt.Sprintf(html.EscapeString(copyrightMask), year, f.Artist)
		f.Copyright = fmt.Sprintf(copyrightMask, year, f.Artist)
	}
}

// NewName provides a new filename for the track based on the renameMask
func (f *Track) NewName(renameMask string) (newTrackName string, err error) {
	if renameMask == "" {
		return f.Filename, nil
	}

	newTrackName = renameMask
	for k, v := range f.Fields() {
		regex := fmt.Sprintf(`{(%s)([^}]*)}`, k)
		re := regexp.MustCompile(regex)
		b := re.FindStringSubmatch(newTrackName)
		if len(b) < 3 {
			continue
		}
		name := b[1]
		if name != k {
			continue
		}
		log.Tracef("k: %v", k)
		log.Tracef("v: %v", v)
		log.Tracef("regex: %v", regex)
		log.Tracef("name: %q", name)
		log.Tracef("b: %v", b)
		format := b[2]
		if format == "" {
			format = "%s"
		}
		log.Tracef("format: %v", format)
		underline := strings.Contains(format, "_")
		if underline {
			format = strings.Replace(format, "_", "", -1)
		}
		dash := strings.Contains(format, "-")
		if dash {
			format = strings.Replace(format, "-", "", -1)
		}
		lastChar := format[len(format)-1:]
		var s string
		switch lastChar {
		case "t":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return "", fmt.Errorf("Error parsing rename_mask %v: field: %v: value: %v: %v", defaults.RenameMask, k, v, err)
			}
			s = fmt.Sprintf(format, b)
		case "b", "c", "d", "o", "q", "x", "X", "U":
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return "", fmt.Errorf("Error parsing rename_mask %v: field: %v: value: %v: %v", defaults.RenameMask, k, v, err)
			}
			s = fmt.Sprintf(format, i)
		case "e", "E", "f", "F", "g", "G":
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return "", fmt.Errorf("Error parsing rename_mask %v: field: %v: value: %v: %v", defaults.RenameMask, k, v, err)
			}
			s = fmt.Sprintf(format, f)
		default:
			s = fmt.Sprintf(format, v)
		}
		if underline {
			s = strings.Replace(s, " ", "_", -1)
		}
		if dash {
			s = strings.Replace(s, " ", "-", -1)
		}
		log.Tracef("s: %v", s)
		newTrackName = strings.Replace(newTrackName, b[0], s, -1)
	}

	return newTrackName, nil
}
