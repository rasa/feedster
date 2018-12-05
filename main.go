// Program feedmp3s tags mp3s from csv file, gens xml too!
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bogem/id3v2"
	"github.com/eduncan911/podcast"
	"github.com/gocarina/gocsv"
	"github.com/rasa/feedster/version"
)

const (
	defaultDiscNumber           = "1"
	defaultCopyright            = "Copyright © & ℗ %d, %s"
	defaultCopyrightDescription = "Copyright"
	defaultCSV                  = "default.csv"
	defaultEncodedBy            = ""
	defaultJPG                  = "default.jpg"
	// You must choose a three-letter language code from ISO 639-2 code list:
	// https://www.loc.gov/standards/iso639-2/php/code_list.php
	defaultLanguage = "eng"
	defaultMime     = "image/jpeg"
	defaultTrackNo  = "1"
	defaultURL      = "https://walford.com/pranayama-fall-2018/"
	defaultXML      = "default.xml"
)

// Track contains the MP3 tags to be updated
type Track struct { // Our example struct, you can use "-" to ignore a field
	Filename        string `csv:"filename"`
	AlbumArtist     string `csv:"album_artist,omitempty"`
	AlbumPrefix     string `csv:"album_prefix,omitempty"`
	AlbumTitle      string `csv:"album_title,omitempty"`
	Artist          string `csv:"artist,omitempty"`
	Composer        string `csv:"composer,omitempty"`
	Copyright       string `csv:"copyright,omitempty"`
	Description     string `csv:"description,omitempty"`
	DiscNumber      string `csv:"disc_number,omitempty"`
	Genre           string `csv:"genre,omitempty"`
	Track           string `csv:"track,omitempty"`
	Prefix          string `csv:"prefix,omitempty"`
	Subtitle        string `csv:"subtitle,omitempty"`
	Summary         string `csv:"summary,omitempty"`
	Title           string `csv:"title,omitempty"`
	Year            string `csv:"year,omitempty"`
	Duration        string
	DurationSeconds int64
	size            int64
	modTime         time.Time
}

func normalizeFilename(filename string) (newname string) {
	filename = strings.Replace(filename, `\`, "/", -1)
	dir := filepath.Dir(filename)
	if len(dir) > 0 {
		if dir == "." {
			dir = ""
		} else {
			if dir[len(dir)-1:] != "/" {
				dir += "/"
			}
		}
	}
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimRight(base, ext)
	newname = dir + name + strings.ToLower(ext)
	return newname
}

func normalizeTrack(track *Track) (err error) {
	oldname := track.Filename
	newname := normalizeFilename(oldname)
	if oldname != newname {
		fi, err := os.Stat(oldname)
		if err != nil {
			_, err = os.Stat(newname)
			if err == nil {
				track.Filename = newname
				return nil
			}
			log.Fatalf("Failed to read %s: %s", oldname, err)
		}
		oldTime := fi.ModTime()
		err = os.Rename(oldname, newname)
		if err != nil {
			log.Fatalf("Cannot rename %s to %s: %s", oldname, newname, err)
		}
		fi, err = os.Stat(newname)
		if err != nil {
			log.Fatalf("Failed to read %s: %s", newname, err)
		}
		newTime := fi.ModTime()
		if oldTime.Unix() != newTime.Unix() {
			err = os.Chtimes(newname, oldTime, oldTime)
			if err != nil {
				log.Fatalf("Failed to set times on %s: %s", newname, err)
			}
		}
	}
	track.Filename = newname
	return nil
}

func hhmmssToUint64(hhmmss string) (seconds int64) {
	// there's surely an easier way than this, right?
	re := regexp.MustCompile(`(\d\d):(\d\d):(\d\d)`)
	b := re.FindStringSubmatch(hhmmss)
	if len(b) < 4 {
		return 0
	}
	hms := fmt.Sprintf("%sh%sm%ss", b[1], b[2], b[3])
	hours, _ := time.ParseDuration(hms)
	return int64(hours.Seconds())
}

func findNewestFile(dir string, mask string) (name string, err error) {
	// inspiration: https://stackoverflow.com/a/45579190
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
				log.Printf("Match failed on %s for %s", mask, fi.Name())
				return "", err
			}
			if !matched {
				continue
			}
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		if fi.ModTime().Before(modTime) {
			continue
		}
		if !fi.ModTime().After(modTime) {
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
		log.Println("Skipping track, as the track title is empty for", track.Filename)
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
	if track.DiscNumber == "" {
		track.DiscNumber = lastTrack.DiscNumber
	}
	if track.DiscNumber == "" {
		track.DiscNumber = defaultDiscNumber
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
	if track.Description == "" {
		track.Description = track.Title
	}
	if track.Genre == "" {
		track.Genre = lastTrack.Genre
	}
	if track.Summary == "" {
		track.Summary = track.Title
	}
	if track.Track == "" {
		i, err := strconv.Atoi(lastTrack.Track)
		if err == nil {
			track.Track = strconv.Itoa(i + 1)
		}
	}
	if track.Track == "" {
		track.Track = defaultTrackNo
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

	cmd := "ffprobe"
	out, err := exec.Command(cmd, track.Filename).CombinedOutput()
	if err != nil {
		log.Fatalf("Command failed: %s: %s", cmd, err)
	}
	re := regexp.MustCompile(`Duration:\s*(\d\d:\d\d:\d\d)\.(\d\d)`)

	b := re.FindSubmatch(out)
	if len(b) > 2 {
		track.Duration = string(b[1])
		track.DurationSeconds = hhmmssToUint64(track.Duration)
		if string(b[2]) != "00" {
			track.DurationSeconds++
		}
	}
	return true
}

func setTags(tag *id3v2.Tag, track *Track) {
	//tag.SetDefaultEncoding(id3v2.EncodingUTF8)
	//tag.SetVersion(4)

	tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, track.AlbumArtist)
	tag.SetAlbum(track.AlbumTitle)
	tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, track.DiscNumber)
	tag.AddTextFrame(tag.CommonID("Album/Movie/Show title"), id3v2.EncodingUTF8, track.AlbumTitle)
	tag.SetArtist(track.Artist)
	tag.AddTextFrame(tag.CommonID("Copyright message"), id3v2.EncodingUTF8, track.Copyright)
	//panics:
	//tag.AddTextFrame(tag.CommonID("Comments"), id3v2.EncodingUTF8, track.Copyright)
	tag.SetGenre(track.Genre)
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, track.Track)
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
	log.Println("Processing", track.Filename)
	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}
	track.modTime = fi.ModTime()
	track.size = fi.Size()
	year := track.modTime.Year()
	tag, err := id3v2.Open(track.Filename, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}

	setTrackDefaults(track, lastTrack, year)
	setTags(tag, track)

	pic, err := addFrontCover(defaultJPG, defaultMime)
	if err == nil && pic != nil {
		tag.AddAttachedPicture(*pic)
	}
	// Write it to file.
	err = tag.Save()
	if err != nil {
		log.Fatalf("Cannot save tags for %s: %s", track.Filename, err)
	}
	tag.Close()
	err = os.Chtimes(track.Filename, track.modTime, track.modTime)
	if err != nil {
		log.Fatalf("Cannot set time for %s: %s", track.Filename, err)
	}
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

	var firstTime = time.Date(2099, time.December, 31, 23, 59, 59, 999999999, time.UTC)
	var lastTime time.Time

	for _, track := range tracks {
		if track.Filename == "" {
			continue
		}
		if track.Title == "" {
			continue
		}
		normalizeTrack(track)
		processTrack(track, lastTrack)
		lastTrack = track
		if firstTime.After(track.modTime) {
			firstTime = track.modTime
		}
		if lastTime.Before(track.modTime) {
			lastTime = track.modTime
		}
	}

	createdDate := firstTime
	updatedDate := lastTime
	// pubDate     := updatedDate.AddDate(0, 0, 3)

	log.Printf("Creating %s\n", defaultXML)
	file, err := os.Create(defaultXML)
	if err != nil {
		log.Fatalf("Cannot create %s: %s", defaultXML, err)
	}
	defer file.Close()

	// instantiate a new Podcast
	p := podcast.New(
		"Sample Podcasts",
		defaultURL,
		"An example Podcast",
		&createdDate, 
		&updatedDate,
	)

	// add some channel properties
	p.ISubtitle = "A simple Podcast"
	p.AddSummary(`link <a href="http://example.com">example.com</a>`)
	p.AddImage(defaultURL + defaultJPG)
	p.AddAuthor("Jane Doe", "jane.doe@example.com")
	p.AddAtomLink(defaultURL + "atom.rss")

	for _, track := range tracks {
		if track.Filename == "" {
			continue
		}
		if track.Title == "" {
			continue
		}
		// d := pubDate.AddDate(0, 0, int(i + 1))

		// create an Item
		item := podcast.Item{
			Title:       track.Title,
			Description: track.Description,
			ISubtitle:   track.Subtitle,
			PubDate:     &track.modTime,
		}
		item.AddImage(defaultURL + defaultJPG)
		item.AddDuration(track.DurationSeconds)
		item.AddSummary(track.Summary)
		// add a Download to the Item
		item.AddEnclosure(defaultURL+track.Filename, podcast.MP3, track.size)

		// add the Item and check for validation errors
		_, err := p.AddItem(item)
		if err != nil {
			log.Printf("Cannot add track %s: %s", track.Filename, err)
		}
	}

	// Podcast.Encode writes to an io.Writer
	err = p.Encode(file)
	if err != nil {
		log.Printf("Cannot write to %s: %s", file.Name(), err)
	}
}

/*
map[
*AlbumArtist:	TPE2:[{Encoding:UTF-8 encoded Unicode Text:album artist }]
DiscNumber:	TPOS:[{Encoding:ISO-8859-1 Text:2 }]
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
