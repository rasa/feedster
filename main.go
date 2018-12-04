// Program feedmp3s tags mp3s from csv file, gens xml too!
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/bogem/id3v2"
	"github.com/gocarina/gocsv"
	"github.com/rasa/feedster/version"
)

const (
	DEFAULT_LANGUAGE = "eng"
	DEFAULT_IMAGE = "folder.jpg"
)

type Track struct { // Our example struct, you can use "-" to ignore a field
	Filename    string `csv:"filename"`
	AlbumArtist string `csv:"album_artist,omitempty"`
	AlbumNo     string `csv:"album_no,omitempty"`
	AlbumPrefix string `csv:"album_prefix,omitempty"`
	AlbumTitle  string `csv:"album_title,omitempty"`
	Artist      string `csv:"track_artist,omitempty"`
	Copyright	string `csv:"copyright,omitempty"`
	Genre		string `csv:"genre,omitempty"`
	No          string `csv:"track_no,omitempty"`
	Prefix      string `csv:"track_prefix,omitempty"`
	Title       string `csv:"track_title,omitempty"`
	Year		string `csv:"year,omitempty"`
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

	var filename string = "playlist.csv"
	if len(os.Args) >= 2 {
		filename = os.Args[1]
	}
	csvFile, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer csvFile.Close()

	tracks := []*Track{}

	if err := gocsv.UnmarshalFile(csvFile, &tracks); err != nil { // Load track from file
		panic(err)
	}
	last_track := &Track{}

	for _, track := range tracks {
		fmt.Printf("%+v\n", track)
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
		defer tag.Close()
		
		if track.Artist == "" {
			track.Artist = last_track.Artist
		}
		if track.AlbumArtist == "" {
			track.AlbumArtist = last_track.AlbumArtist
		}
		if track.AlbumArtist == "" {
			track.AlbumArtist = track.Artist
		}
		if track.AlbumNo == "" {
			track.AlbumNo = last_track.AlbumNo
		}
		if track.AlbumNo == "" {
			track.AlbumNo = "1"
		}
		if track.AlbumPrefix == "" {
			track.AlbumPrefix = last_track.AlbumPrefix
		}
		if track.AlbumTitle == "" {
			track.AlbumTitle = last_track.AlbumTitle
		}
		if track.Copyright == "" {
			track.Copyright = last_track.Copyright
		}
		if track.Copyright == "" {
			track.Copyright = fmt.Sprintf("Copyright © & ℗ %d, %s", year, track.Artist)
		}
		if track.Genre == "" {
			track.Genre = last_track.Genre
		}
		if track.No == "" {
			i, err := strconv.Atoi(last_track.No)
			if err == nil {
				track.No = strconv.Itoa(i + 1)
			}
		}
		if track.No == "" {
			track.No = "1"
		}
		if track.Prefix == "" {
			track.Prefix = last_track.Prefix
		}
		if track.Title == "" {
			log.Printf("title is empty")
			continue
		}
		if track.Year == "" {
			track.Year = last_track.Year
		}
		if track.Year == "" {
			track.Year = strconv.Itoa(year)
		}
		
		//tag.SetDefaultEncoding(id3v2.EncodingUTF8)
		//tag.SetVersion(4)
		
		tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, track.AlbumArtist)
		tag.SetAlbum(track.AlbumTitle)
		tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, track.AlbumNo)
		tag.AddTextFrame(tag.CommonID("Original filename"), id3v2.EncodingUTF8, track.Filename)
		tag.AddTextFrame(tag.CommonID("Album/Movie/Show title"), id3v2.EncodingUTF8, track.AlbumTitle)
		tag.SetArtist(track.Artist)
		tag.AddTextFrame(tag.CommonID("Copyright message"), id3v2.EncodingUTF8, track.Copyright)
		//panic:
		//tag.AddTextFrame(tag.CommonID("Comments"), id3v2.EncodingUTF8, track.Copyright)
		tag.SetGenre(track.Genre)
		tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, track.No)
		tag.SetTitle(track.Title)
		tag.SetYear(track.Year)

		tag.AddTextFrame(tag.CommonID("Composer"), id3v2.EncodingUTF8, track.Artist)
		tag.AddTextFrame(tag.CommonID("Encoded by"), id3v2.EncodingUTF8, "")
		
		tag.AddTextFrame(tag.CommonID("Language"), id3v2.EncodingUTF8, DEFAULT_LANGUAGE)
		// Set comment frame.
		comment := id3v2.CommentFrame{
			Encoding:    id3v2.EncodingUTF8,
			Language:    "eng",
			Description: "Copyright",
			Text:        track.Copyright,
		}
		tag.AddCommentFrame(comment)
		_, err = os.Stat(DEFAULT_IMAGE)
		if err == nil {
			// See https://godoc.org/github.com/bogem/id3v2#PictureFrame
			artwork, err := ioutil.ReadFile(DEFAULT_IMAGE)
			if err != nil {
				log.Fatalf("Cannot read %s: %s", DEFAULT_IMAGE, err)
			}

			pic := id3v2.PictureFrame{
				Encoding:    id3v2.EncodingUTF8,
				MimeType:    "image/jpeg",
				PictureType: id3v2.PTFrontCover,
				Description: "Front cover",
				Picture:     artwork,
			}
			tag.AddAttachedPicture(pic)
		}
		// Write it to file.
		if err = tag.Save(); err != nil {
			log.Fatal("Error while saving a tag: ", err)
		}
		last_track = track
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
