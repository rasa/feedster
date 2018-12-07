package main

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	minMilliseconds = 1
	minFileSize     = 1024
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
	// duration is determined by running ffprobe on filename
	Duration             string
	DurationMilliseconds int64
	// fileSize is determined via os.Stat()
	FileSize int64
	// modTime is determined via os.Stat()
	ModTime int64
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
		val.Field(i).SetString(fieldValue)
		return true
	}
	return false
}

// IsValid returns true if the track should be processed/exported
func (f *Track) IsValid() bool {
	if f.Filename == "" {
		return false
	}
	if f.Title == "" {
		return false
	}
	if f.DurationMilliseconds < minMilliseconds {
		return false
	}
	if f.FileSize < minFileSize {
		return false
	}
	if f.ModTime == 0 {
		return false
	}
	return true
}

func (f *Track) Error() (err error) {
	if f.Filename == "" {
		return fmt.Errorf("Filename is empty")
	}
	if f.Title == "" {
		return fmt.Errorf("Title is empty")
	}
	if f.DurationMilliseconds < minMilliseconds {
		return fmt.Errorf("File is too short in duration")
	}
	if f.FileSize < minFileSize {
		return fmt.Errorf("File is too small in size")
	}
	if f.ModTime == 0 {
		return fmt.Errorf("File does not exist, or is unreadable")
	}
	return nil
}
