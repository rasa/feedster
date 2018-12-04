// Program feedmp3s tags mp3s from csv file, gens xml too!
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bogem/id3v2"
	"github.com/gocarina/gocsv"
	"github.com/rasa/feedster/version"
)

const (
	defaultAlbumNo  = "1"
	defaultCopyright = "Copyright © & ℗ %d, %s"
	defaultCopyrightDescription = "Copyright"
	defaultCSV    	= "default.csv"
	defaultEncodedBy = ""
	defaultJPG    	= "default.jpg"
	defaultLanguage = "eng"
	defaultMime		= "image/jpeg"
	defaultTrackNo  = "1"
	defaultXML    	= "default.xml"
)

// Track contains the MP3 tags to be updated
type Track struct { // Our example struct, you can use "-" to ignore a field
	Filename    string `csv:"filename"`
	AlbumArtist string `csv:"album_artist,omitempty"`
	AlbumNo     string `csv:"album_no,omitempty"`
	AlbumPrefix string `csv:"album_prefix,omitempty"`
	AlbumTitle  string `csv:"album_title,omitempty"`
	Artist      string `csv:"track_artist,omitempty"`
	Copyright   string `csv:"copyright,omitempty"`
	Genre       string `csv:"genre,omitempty"`
	No          string `csv:"track_no,omitempty"`
	Prefix      string `csv:"track_prefix,omitempty"`
	Title       string `csv:"track_title,omitempty"`
	Year        string `csv:"year,omitempty"`
}

func findNewestFile(dir string, mask string) (name string, err error) {
    files, err := ioutil.ReadDir(dir)
    if err != nil {
        log.Fatalf("Cannot read directory %s: %s", dir, err)
    }
    var modTime time.Time
    var names []string
    for _, fi := range files {
		if mask != "" {
			matched, err := path.Match(mask, fi.Name())
			if err != nil {
				log.Printf("Match failed on %s for %s", mask, fi.Name)
				return "", err
			}
			if ! matched {
				continue
			}
		}
        if ! fi.Mode().IsRegular() {
			continue
		}
		if fi.ModTime().Before(modTime) {
			continue
		}
		if ! fi.ModTime().After(modTime) {
			continue
		}
		modTime = fi.ModTime()
		names = names[:0]
		names = append(names, fi.Name())
    }
    if len(names) > 0 {
        return names[0], nil
    }
	return "", fmt.Errorf("No files found matching %s", mask)
}

func setTrackDefaults(track *Track, lastTrack *Track, year int) bool {
	if track.Title == "" {
		log.Printf("Skipping track, as the track title is empty")
		return false
	}
	if track.Artist == "" {
		track.Artist = lastTrack.Artist
	}
	if track.AlbumArtist == "" {
		track.AlbumArtist = lastTrack.AlbumArtist
	}
	if track.AlbumArtist == "" {
		track.AlbumArtist = track.Artist
	}
	if track.AlbumNo == "" {
		track.AlbumNo = lastTrack.AlbumNo
	}
	if track.AlbumNo == "" {
		track.AlbumNo = defaultAlbumNo
	}
	if track.AlbumPrefix == "" {
		track.AlbumPrefix = lastTrack.AlbumPrefix
	}
	if track.AlbumTitle == "" {
		track.AlbumTitle = lastTrack.AlbumTitle
	}
	if track.Copyright == "" {
		track.Copyright = lastTrack.Copyright
	}
	if track.Copyright == "" {
		track.Copyright = fmt.Sprintf(defaultCopyright, year, track.Artist)
	}
	if track.Genre == "" {
		track.Genre = lastTrack.Genre
	}
	if track.No == "" {
		i, err := strconv.Atoi(lastTrack.No)
		if err == nil {
			track.No = strconv.Itoa(i + 1)
		}
	}
	if track.No == "" {
		track.No = defaultTrackNo
	}
	if track.Prefix == "" {
		track.Prefix = lastTrack.Prefix
	}
	if track.Year == "" {
		track.Year = lastTrack.Year
	}
	if track.Year == "" {
		track.Year = strconv.Itoa(year)
	}
	return true
}

func setTags(tag *id3v2.Tag, track *Track) {
	//tag.SetDefaultEncoding(id3v2.EncodingUTF8)
	//tag.SetVersion(4)

	tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, track.AlbumArtist)
	tag.SetAlbum(track.AlbumTitle)
	tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, track.AlbumNo)
	tag.AddTextFrame(tag.CommonID("Album/Movie/Show title"), id3v2.EncodingUTF8, track.AlbumTitle)
	tag.SetArtist(track.Artist)
	tag.AddTextFrame(tag.CommonID("Copyright message"), id3v2.EncodingUTF8, track.Copyright)
	//panics:
	//tag.AddTextFrame(tag.CommonID("Comments"), id3v2.EncodingUTF8, track.Copyright)
	tag.SetGenre(track.Genre)
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, track.No)
	tag.SetTitle(track.Title)
	tag.SetYear(track.Year)

	// Set comment frame.
	comment := id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    defaultLanguage,
		Description: defaultCopyrightDescription,
		Text:        track.Copyright,
	}
	tag.AddCommentFrame(comment)

	tag.AddTextFrame(tag.CommonID("Composer"), id3v2.EncodingUTF8, track.Artist)
	if defaultEncodedBy != "" {
		tag.AddTextFrame(tag.CommonID("Encoded by"), id3v2.EncodingUTF8, defaultEncodedBy)
	}
	tag.AddTextFrame(tag.CommonID("Language"), id3v2.EncodingUTF8, defaultLanguage)
	tag.AddTextFrame(tag.CommonID("Original filename"), id3v2.EncodingUTF8, track.Filename)
}

func addFrontCover(filename string, mimeType string) (pic *id3v2.PictureFrame, err error) {
	_, err = os.Stat(filename)
	if err != nil {
		log.Printf("Cannot read %s: %s", filename, err)
		return nil, nil
	}
	
	// See https://godoc.org/github.com/bogem/id3v2#PictureFrame
	artwork, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Cannot read %s: %s", filename, err)
	}

	pic = &id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    mimeType,
		PictureType: id3v2.PTFrontCover,
		Description: "Front cover",
		Picture:     artwork,
	}
	return pic, nil
}

func processTrack(track *Track, lastTrack *Track) {
	//fmt.Printf("%+v\n", track)
	log.Printf("Processing %v\n", track.Filename)
	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}
	t := fi.ModTime()
	year := t.Year()
	tag, err := id3v2.Open(track.Filename, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}

	setTrackDefaults(track, lastTrack, year)
	setTags(tag, track)

	pic, err := addFrontCover(defaultJPG, defaultMime)
	if pic != nil {
		tag.AddAttachedPicture(*pic)
	}
	// Write it to file.
	err = tag.Save()
	if err != nil {
		log.Fatal("Error while saving a tag: ", err)
	}
	tag.Close()
}

func main() {
	basename := filepath.Base(os.Args[0])
	progname := strings.TrimSuffix(basename, filepath.Ext(basename))

	fmt.Printf("%s: Version %s (%s)\n", progname, version.VERSION, version.GITCOMMIT)
	fmt.Printf("Built with %s for %s/%s (%d CPUs/%d GOMAXPROCS)\n",
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		runtime.NumCPU(),
		runtime.GOMAXPROCS(-1))

	var filename = defaultCSV
	if len(os.Args) >= 2 {
		filename = os.Args[1]
	}
	csvFile, err := os.Open(filename)
	if err != nil {
		log.Panicf("Cannot open %s: %s", filename, err)
	}
	defer csvFile.Close()

	tracks := []*Track{}

	err = gocsv.UnmarshalFile(csvFile, &tracks)
	if err != nil { // Load track from file
		log.Panicf("Cannot read %s: %s", filename, err)
	}

	lastTrack := &Track{}

	for _, track := range tracks {
		processTrack(track, lastTrack)
		lastTrack = track
	}
}

/*
map[
*AlbumArtist:	TPE2:[{Encoding:UTF-8 encoded Unicode Text:album artist }]
AlbumNo:	TPOS:[{Encoding:ISO-8859-1 Text:2 }]
AlbumTitle:	TALB:[{Encoding:UTF-8 encoded Unicode Text:album }]
Artist:	TPE1:[{Encoding:UTF-8 encoded Unicode Text:artist }]
*Copyright*:	COMM:[{Encoding:UTF-8 encoded Unicode Language:eng Description: Text:comment }]]
*Composer:	TCOM:[{Encoding:UTF-8 encoded Unicode Text:composer }]
EncodedBy:	TENC:[{Encoding:UTF-8 encoded Unicode Text:SONY IC RECORDER MP3 3.1.8 }]
Genre:	TCON:[{Encoding:UTF-8 encoded Unicode Text:meditative }]
No:	TRCK:[{Encoding:ISO-8859-1 Text:1 }]
Title:	TIT2:[{Encoding:UTF-8 encoded Unicode Text:title }]
Year:	TDRC:[{Encoding:ISO-8859-1 Text:year }]
*/
