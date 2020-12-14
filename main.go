// Program feedster tags mp3s from csv/xls file and gens podcast xml
package main

// see https://github.com/simplepie/simplepie-ng/wiki/Spec:-iTunes-Podcast-RSS
// http://id3.org/d3v2.3.0

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	fpodcast "github.com/rasa/feedster/podcast"
	"github.com/rasa/feedster/version"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/bogem/id3v2"
	"github.com/eduncan911/podcast"
	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	copyrightDescription = "Copyright"
	defaultImageExt      = ".jpg"
	defaultLogLevel      = log.InfoLevel
	defaultMimeType      = "image/jpeg"
	defaultYAML          = "default.yaml"
	feedsterURL          = "https://github.com/rasa/feedster"
	localYAML            = "local.yaml"

	outputFileMask  = "%s%s.xml"
	podcastFileMask = "%s-podcast.yaml"
	tracksFileMask  = "%s-tracks%s"

	imageWidthMin  = 1400
	imageHeightMin = 1400
	imageWidthMax  = 3000
	imageHeightMax = 3000
)

// Default has default settings read from config.yaml (and local.yaml, if it exists)
type Default struct {
	Author         string `yaml:"author,omitempty"`
	BaseURL        string `yaml:"base_url"`
	Category       string `yaml:"category,omitempty"`
	Complete       string `yaml:"complete,omitempty"`
	Copyright      string `yaml:"copyright,omitempty"`
	CopyrightMask  string `yaml:"copyright_mask,omitempty"`
	DiscNumber     string `yaml:"disc_number,omitempty"`
	Email          string `yaml:"email,omitempty"`
	EncodedBy      string `yaml:"encoded_by,omitempty"`
	Exiftool       string `yaml:"exiftool,omitempty"`
	Explicit       string `yaml:"explicit,omitempty"`
	Ffmpeg         string `yaml:"ffmpeg,omitempty"`
	Ffprobe        string `yaml:"ffprobe,omitempty"`
	Generator      string `yaml:"generator,omitempty"`
	Image          string `yaml:"image,omitempty"`
	Language       string `yaml:"language,omitempty"`
	ManagingEditor string `yaml:"managingeditor,omitempty"`
	OutputDir      string `yaml:"output_dir,omitempty"`
	OutputFile     string `yaml:"output_file,omitempty"`
	PodcastFile    string `yaml:"podcast_file,omitempty"`
	RenameMask     string `yaml:"rename_mask,omitempty"`
	TotalDiscs     string `yaml:"total_discs,omitempty"`
	TotalTracks    string `yaml:"total_tracks,omitempty"`
	TrackNo        string `yaml:"track_no,omitempty"`
	TracksFile     string `yaml:"tracks_file,omitempty"`
	TTL            string `yaml:"ttl,omitempty"`
	WebMaster      string `yaml:"webmaster,omitempty"`
	totalDiscs     bool
	totalTracks    bool
}

var initDefaults = &Default{
	Complete:      "no",
	CopyrightMask: "Copyright (c) & (p) %d, %s",
	// This works, but some players do not display the (p) symbol (like VLC):
	// CopyrightMask: "Copyright \u00a9 & \u2117 %d, %s",
	DiscNumber:  "1",
	EncodedBy:   "feedster " + version.VERSION + " (" + feedsterURL + ")",
	Exiftool:    "exiftool",
	Explicit:    "no",
	Ffmpeg:      "ffmpeg",
	Ffprobe:     "ffprobe",
	Generator:   "feedster " + version.VERSION + " (" + feedsterURL + ")",
	Language:    "en-us",
	TotalDiscs:  "true",
	TotalTracks: "true",
	TrackNo:     "1",
	TTL:         "1",
}

var defaults = initDefaults

// search for files in this order
var trackFileExtensions = []string{
	".xlsx",
	".xls",
	".csv",
	".txt",
}

/*
call tree:

main()
	processYAML(yamlFile string)
		readYAML(yamlFile string) (fp fpodcast.Podcast)
			loadDefaults(yamlFile string, genFilenames bool)
			setDefaults(fp *fpodcast.Podcast)
			processImage(fp *fpodcast.Podcast, imageName string, baseURL string) (err error)
		getTracksFilename(yamlFile string) (tracksFile string)
		processTracks(fp fpodcast.Podcast, tracksFile string) (tracks []*Track)
			readCSV(csvFile string) (tracks []*Track)
			readTXT(txtFile string) (tracks []*Track)
			readXLS(xlsFile string) (tracks []*Track)
			preProcessTrack(trackIndex int, track *Track, lastTrack *Track) bool
				setTrackDefaults(track *Track, lastTrack *Track) bool
					setCopyright(track *Track, copyright string, copyrightMask string, year int)
					utils.getDurationViaExiftool(filename string, exiftool string) (durationMilliseconds int64, err error)
					utils.getDurationViaFfmpeg(filename string, ffmpeg string) (durationMilliseconds int64, err error)
					utils.getDurationViaFfprobe(filename string, ffprobe string) (durationMilliseconds int64, err error)
			processTrack(trackIndex int, track *Track, lastTrack *Track, tracks []*Track)
				setTags(tag *id3v2.Tag, track *Track, tracks []*Track)
					totalDiscs(tracks []*Track) (totalDiscs int)
					totalTracks(tracks []*Track, discNumber string) (totalTracks int)
					addTextFrame(tag *id3v2.Tag, id string, text string)
					addFrontCover(filename string) (pic *id3v2.PictureFrame, err error)
		savePodcast(fp fpodcast.Podcast, tracks []*Track)
			createdDate(tracks []*Track) (createdDate time.Time)
			updatedDate(tracks []*Track) (updatedDate time.Time)
			setPodcast(p *podcast.Podcast, fp *fpodcast.Podcast)
			addTrack(p *podcast.Podcast, track *Track)
				track.NewName(renameMask string) (newTrackName string, err error)
				copyImage(fp *fpodcast.Podcast, outputDir string)
			validTracks(tracks []*Track) (rv uint)
*/

/*
func l(level log.Level) bool {
	return log.IsLevelEnabled(level)
}

func debug() bool {
	return log.IsLevelEnabled(log.DebugLevel)
}
*/
func trace() bool {
	return log.IsLevelEnabled(log.TraceLevel)
}

func dump(s string, x interface{}) {
	if !trace() {
		return
	}

	if s != "" {
		log.Trace(s)
	}
	if x == nil {
		return
	}

	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		log.Error("JSON marshaling error: ", err)
		return
	}
	log.Trace(string(b))
}

func setTrackDefaults(track *Track, lastTrack *Track) bool {
	if track.Filename == "" {
		track.Processed = true
		return false
	}

	if track.Title == "" {
		if track.Filename != "" {
			track.Title = basename(path.Base(track.Filename))
		}
	}
	if track.Description == "" {
		// per https://github.com/eduncan911/podcast/blob/master/podcast.go#L270
		track.Description = track.Title
	}

	if lastTrack != nil {
		track.Artist = lastTrack.Artist
		track.AlbumArtist = lastTrack.AlbumArtist
		track.AlbumArtist = track.Artist
		track.AlbumTitle = lastTrack.AlbumTitle
		track.Copyright = lastTrack.Copyright
		track.Genre = lastTrack.Genre
	}

	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %q: %s", track.Filename, err)
	}
	track.OriginalModTime = fi.ModTime().UnixNano()
	track.ModTime = track.OriginalModTime
	track.OriginalFileSize = fi.Size()
	track.FileSize = fi.Size()

	year := fi.ModTime().Year()
	if track.Year != "" {
		y, err := strconv.Atoi(track.Year)
		if err == nil {
			year = y
		}
	} else {
		track.Year = strconv.Itoa(year)
	}

	track.SetCopyright(defaults.Copyright, defaults.CopyrightMask, year)

	track.DurationMilliseconds, err = getDurationViaExiftool(track.Filename, defaults.Exiftool)
	var err2 error
	if err != nil {
		track.DurationMilliseconds, err2 = getDurationViaFfprobe(track.Filename, defaults.Ffprobe)
	}
	var err3 error
	if err2 != nil {
		track.DurationMilliseconds, err3 = getDurationViaFfmpeg(track.Filename, defaults.Ffmpeg)
	}
	if err3 != nil {
		log.Warnf(err.Error())
		log.Warnf(err2.Error())
		log.Warnf(err3.Error())
	}

	if track.Track == "" {
		if lastTrack != nil {
			if lastTrack.Track != "" {
				track.Track = lastTrack.Track
				if track.IsValid() {
					i, _ := strconv.Atoi(track.Track)
					track.Track = strconv.Itoa(i + 1)
				}
			}
		}
	}
	if track.Track == "" {
		track.Track = defaults.TrackNo
	}
	if lastTrack != nil {
		if track.DiscNumber == "" {
			track.DiscNumber = lastTrack.DiscNumber
		} else {
			if track.DiscNumber != lastTrack.DiscNumber {
				track.Track = "1"
			}
		}
	}

	if track.DiscNumber == "" {
		track.DiscNumber = defaults.DiscNumber
	}

	track.Processed = true
	return true
}

func totalDiscs(tracks []*Track) (totalDiscs int) {
	totalDiscs = 0
	for _, track := range tracks {
		if !track.IsValid() {
			continue
		}
		if track.DiscNumber == "" {
			continue
		}
		i, _ := strconv.Atoi(track.DiscNumber)
		if i > totalDiscs {
			totalDiscs = i
		}
	}
	return totalDiscs
}

func totalTracks(tracks []*Track, discNumber string) (totalTracks int) {
	totalTracks = 0
	for _, track := range tracks {
		if !track.IsValid() {
			continue
		}
		if discNumber != track.DiscNumber {
			continue
		}
		if track.Track == "" {
			continue
		}
		i, _ := strconv.Atoi(track.Track)
		if i > totalTracks {
			totalTracks = i
		}
	}
	return totalTracks
}

func addTextFrame(tag *id3v2.Tag, id string, text string) {
	if text == "" {
		return
	}
	tid := tag.CommonID(id)
	if id == "" {
		log.Warnf("Unknown id3v2 ID %q", id)
	}
	tag.AddTextFrame(tid, id3v2.EncodingUTF8, text)
}

func setTags(tag *id3v2.Tag, track *Track, tracks []*Track) {
	//tag.SetDefaultEncoding(id3v2.EncodingUTF8)
	//tag.SetVersion(4)

	totalDiscs := totalDiscs(tracks)
	totalTracks := totalTracks(tracks, track.DiscNumber)

	discNumber := track.DiscNumber
	if defaults.totalDiscs && discNumber != "" && totalDiscs > 0 {
		discNumber = fmt.Sprintf("%s/%d", discNumber, totalDiscs)
	}

	trackNumber := track.Track
	if defaults.totalTracks && trackNumber != "" && totalTracks > 0 {
		trackNumber = fmt.Sprintf("%s/%d", trackNumber, totalTracks)
	}

	log.Tracef("totalDiscs:  %v", totalDiscs)
	log.Tracef("totalTracks: %v", totalTracks)
	log.Tracef("discNumber:  %v", discNumber)
	log.Tracef("trackNumber: %v", trackNumber)

	// user defined fields:

	tag.SetAlbum(track.AlbumTitle)
	tag.SetArtist(track.Artist)
	tag.SetGenre(track.Genre)
	tag.SetTitle(track.Title)
	tag.SetYear(track.Year)

	addTextFrame(tag, "Band/Orchestra/Accompaniment", track.AlbumArtist)
	addTextFrame(tag, "Album/Movie/Show title", track.AlbumTitle)
	addTextFrame(tag, "Composer", track.Composer)
	addTextFrame(tag, "Copyright message", track.Copyright)
	//panics:
	//tag.AddTextFrame(tag.CommonID("Comments"), id3v2.EncodingUTF8, track.Copyright)
	addTextFrame(tag, "Part of a set", discNumber)
	addTextFrame(tag, "Encoded by", defaults.EncodedBy)
	addTextFrame(tag, "Language", defaults.Language)

	description := ""
	if track.Description != "" {
		description = track.Description
	}
	if track.Subtitle != "" {
		if description != "" {
			description += " / "
		}
		description += track.Subtitle
	}
	addTextFrame(tag, "Subtitle/Description refinement", description)

	addTextFrame(tag, "Track number/Position in set", trackNumber)

	// system defined fields:

	modTime := time.Unix(0, track.ModTime)
	mmdd := fmt.Sprintf("%02d%02d", modTime.Month(), modTime.Day())
	hhmm := fmt.Sprintf("%02d%02d", modTime.Hour(), modTime.Minute())

	addTextFrame(tag, "Date", mmdd)
	addTextFrame(tag, "Time", hhmm)

	addTextFrame(tag, "Original filename", track.OriginalFilename)
	addTextFrame(tag, "Size", strconv.FormatInt(track.OriginalFileSize, 10))
	if track.DurationMilliseconds > 0 {
		addTextFrame(tag, "Length", strconv.FormatInt(track.DurationMilliseconds, 10))
	}

	// Set comment frame.
	comment := id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    bCF47ToISO3(defaults.Language),
		Description: copyrightDescription,
		Text:        track.Copyright,
	}
	tag.AddCommentFrame(comment)

	if defaults.Image == "" {
		return
	}

	pic, err := addFrontCover(defaults.Image)
	if err == nil && pic != nil {
		tag.AddAttachedPicture(*pic)
	}
}

func addFrontCover(filename string) (pic *id3v2.PictureFrame, err error) {
	log.Debugf("Reading %q", filename)
	_, err = os.Stat(filename)
	if err != nil {
		log.Warnf("Cannot read %q: %s", filename, err)
		return nil, nil
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mimeType := mime.TypeByExtension(ext)

	if mimeType == "" {
		log.Warnf("Unknown mime type for image %q: %s", filename, err)
		mimeType = defaultMimeType
	}

	// See https://godoc.org/github.com/bogem/id3v2#PictureFrame
	artwork, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Cannot read %q: %s", filename, err)
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

func preProcessTrack(trackIndex int, track *Track, lastTrack *Track) bool {
	if track.Filename != "" {
		log.Infof("Preprocessing row %2d: %q", trackIndex, track.Filename)
		track.NormalizeFilename()
	}

	if !track.IsValid() {
		log.Infof("Skipping row %2d: %q: %s", trackIndex, track.Filename, track.Error())
		return false
	}

	setTrackDefaults(track, lastTrack)

	if !track.IsValid() {
		log.Infof("Skipping row %2d: %q: %s", trackIndex, track.Filename, track.Error())
		return false
	}

	return true
}

func processTrack(trackIndex int, track *Track, lastTrack *Track, tracks []*Track) {
	log.Infof("Processing track %2d: %q", trackIndex, track.Filename)

	tag, err := id3v2.Open(track.Filename, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("Cannot open %q: %s", track.Filename, err)
	}

	setTags(tag, track, tracks)

	// Write it to file.
	err = tag.Save()
	if err != nil {
		log.Fatalf("Cannot save tags for %q: %s", track.Filename, err)
	}
	tag.Close()
	if track.OriginalModTime != 0 {
		modTime := time.Unix(0, track.OriginalModTime)
		err = os.Chtimes(track.Filename, modTime, modTime)
		if err != nil {
			log.Warnf("Cannot set time for %q: %s", track.Filename, err)
		}
	}

	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %q: %s", track.Filename, err)
	}
	track.FileSize = fi.Size()
}

func setDefaults(fp *fpodcast.Podcast) {
	fp.IAuthor = defaults.Author
	fp.Category = defaults.Category
	fp.IComplete = defaults.Complete
	fp.Copyright = defaults.Copyright
	fp.IExplicit = defaults.Explicit
	fp.Generator = defaults.Generator
	fp.Language = defaults.Language
	fp.ManagingEditor = defaults.ManagingEditor
	if defaults.TTL != "" {
		fp.TTL, _ = strconv.Atoi(defaults.TTL)
	}
	fp.WebMaster = defaults.WebMaster

	fp.IOwner = &fpodcast.Author{Name: defaults.Author, Email: defaults.Email}
}

func setPodcast(p *podcast.Podcast, fp *fpodcast.Podcast) {
	p.Title = fp.Title
	p.Link = fp.Link
	p.Description = fp.Description

	// p.Category = fp.Category
	re := regexp.MustCompile(`^([^,]*),(.*)$`)
	b := re.FindStringSubmatch(fp.Category)
	var subCategories []string
	if len(b) > 0 {
		fp.Category = b[1]
		subCategories = append(subCategories, b[2])
	}
	p.AddCategory(fp.Category, subCategories)

	p.Cloud = fp.Cloud
	p.Copyright = fp.Copyright
	p.Docs = fp.Docs

	if fp.Generator != "" {
		p.Generator = fp.Generator
	}

	p.Language = fp.Language

	if fp.LastBuildDate != "" {
		p.LastBuildDate = fp.LastBuildDate
	}

	p.ManagingEditor = fp.ManagingEditor

	if fp.PubDate != "" {
		p.PubDate = fp.PubDate
	}

	p.Rating = fp.Rating
	p.SkipHours = fp.SkipHours
	p.SkipDays = fp.SkipDays
	p.TTL = fp.TTL
	p.WebMaster = fp.WebMaster

	// This formats the author as: ex@example.com (Author Name)
	// p.AddAuthor(fp.IOwner.Name, fp.IOwner.Email)
	p.IAuthor = fp.IAuthor
	p.AddSubTitle(fp.ISubtitle)
	p.IBlock = fp.IBlock
	p.IDuration = fp.IDuration
	p.IExplicit = fp.IExplicit
	p.IComplete = fp.IComplete
	p.INewFeedURL = fp.INewFeedURL

	if fp.Image != nil {
		if fp.Image.URL != "" {
			p.AddImage(fp.Image.URL)
		}
	}

	if fp.AtomLink != nil {
		if fp.AtomLink.HREF != "" {
			p.AddAtomLink(fp.AtomLink.HREF)
		}
	}

	if fp.ISummary != nil {
		p.AddSummary(fp.ISummary.Text)
	}

	if fp.IOwner != nil {
		p.IOwner = &podcast.Author{Name: fp.IOwner.Name, Email: fp.IOwner.Email}
	}
}

func addTrack(p *podcast.Podcast, track *Track) {
	log.Debugf("Adding track %q", track.Filename)
	pubDate := time.Unix(0, track.ModTime)
	item := podcast.Item{
		Title:       track.Title,
		Description: track.Description,
		ISubtitle:   track.Subtitle,
		PubDate:     &pubDate,
	}
	// @TODO(rasa) change to p.Image.URL
	item.AddImage(defaults.BaseURL + defaults.Image)
	if track.DurationMilliseconds > 0 {
		item.IDuration = track.Duration()
	}
	if track.Summary != "" {
		item.AddSummary(track.Summary)
	}

	newTrackName, err := track.NewName(defaults.RenameMask)
	if err != nil {
		log.Fatal(err)
	}
	if newTrackName != "" {
		if !strings.EqualFold(track.Filename, newTrackName) {
			newPath := defaults.OutputDir + newTrackName
			log.Infof("Copying %q to %q", track.Filename, newPath)
			err = copyFile(track.Filename, newPath)
			if err != nil {
				log.Fatalf("Cannot copy %q to %q: %s", track.Filename, newPath, err)

			}
			modTime := time.Unix(0, track.ModTime)
			log.Debugf("Setting time for %q to %v", newPath, modTime)
			err = os.Chtimes(newPath, modTime, modTime)
			if err != nil {
				log.Warnf("Cannot set time for %q: %s", newPath, err)
			}

			track.Filename = newTrackName
		}
	}

	// add a Download to the Item
	item.AddEnclosure(defaults.BaseURL+track.Filename, podcast.MP3, track.FileSize)

	// add the Item and check for validation errors
	_, err = p.AddItem(item)
	if err != nil {
		log.Fatalf("Cannot add track %q: %s", track.Filename, err)
	}
}

func processImage(fp *fpodcast.Podcast, imageName string, baseURL string) (err error) {
	if imageName == "" {
		if fp.Image == nil {
			return nil
		}
		if fp.Image.URL == "" {
			return nil
		}
	}

	if imageName != "" {
		if fp.Image == nil {
			fp.Image = &fpodcast.Image{}
		}
		if fp.Image.URL == "" {
			fp.Image.URL = baseURL + imageName
		}
	}

	basename := path.Base(fp.Image.URL)
	log.Debugf("Processing image %q", basename)
	reader, err := os.Open(basename)
	if err != nil {
		log.Warnf("Cannot open %q: %s", basename, err)
		return err
	}
	defer reader.Close()

	im, _, err := image.DecodeConfig(reader)
	if err != nil {
		log.Warnf("Cannot read %q: %s", basename, err)
		return err
	}

	fp.Image.Width = im.Width
	fp.Image.Height = im.Height

	if fp.Image.Width < imageWidthMin {
		err = fmt.Errorf("%q: image width (%d) needs to be %d or greater", basename, fp.Image.Width, imageWidthMin)
		log.Warn(err)
	}
	if fp.Image.Width > imageWidthMax {
		err = fmt.Errorf("%q: image width (%d) needs to be %d or less", basename, fp.Image.Width, imageWidthMax)
		log.Warn(err)
	}
	if fp.Image.Height < imageHeightMin {
		err = fmt.Errorf("%q: image height (%d) needs to be %d or greater", basename, fp.Image.Height, imageHeightMin)
		log.Warn(err)
	}
	if fp.Image.Height > imageHeightMax {
		err = fmt.Errorf("%q: image height (%d) needs to be %d or less", basename, fp.Image.Height, imageHeightMax)
		log.Warn(err)
	}

	return err
}

func copyImage(fp *fpodcast.Podcast, outputDir string) {
	basename := path.Base(fp.Image.URL)
	newPath := outputDir + basename
	log.Infof("Copying %q to %q", basename, newPath)
	err := copyFile(basename, newPath)
	if err != nil {
		log.Fatalf("Cannot copy %q to %q: %s", basename, newPath, err)
	}
}

func readCSV(csvFile string) (tracks []*Track) {
	log.Infof("Reading %q", csvFile)
	csvFD, err := os.Open(csvFile)
	if err != nil {
		log.Fatalf("Cannot read %q: %s", csvFile, err)
	}
	defer csvFD.Close()

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = ','
		r.Comment = '#'
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		r.TrimLeadingSpace = true
		return r
	})

	err = gocsv.Unmarshal(csvFD, &tracks)
	if err != nil { // Load track from file
		log.Fatalf("Cannot process %q: %s", csvFile, err)
	}

	return tracks
}

func readTXT(txtFile string) (tracks []*Track) {
	log.Infof("Reading %q", txtFile)
	csvFD, err := os.Open(txtFile)
	if err != nil {
		log.Fatalf("Cannot read %q: %s", txtFile, err)
	}
	defer csvFD.Close()

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = '\t'
		r.Comment = '#'
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		// see https://github.com/golang/go/blob/master/src/encoding/csv/reader.go#L134
		r.TrimLeadingSpace = false
		return r
	})

	err = gocsv.Unmarshal(csvFD, &tracks)
	if err != nil { // Load track from file
		log.Fatalf("Cannot process %q: %s", txtFile, err)
	}

	return tracks
}

func readXLS(xlsFile string) (tracks []*Track) {
	log.Infof("Reading %q", xlsFile)
	xlsx, err := excelize.OpenFile(xlsFile)
	if err != nil {
		log.Fatalf("Cannot read %q: %s", xlsFile, err)
	}

	var sheetName string
	for _, sheetName = range xlsx.GetSheetMap() {
		// use the first sheet in the workbook
		break
	}
	if sheetName == "" {
		log.Fatalf("Cannot find any sheets in %q", xlsFile)
	}

	nameToColMap := make(map[int]string)

	rows, err := xlsx.GetRows(sheetName)
	if err != nil {
		log.Fatalf("Cannot get rows for sheet %q in %q: %s", sheetName, xlsFile, err)
	}
	for i, row := range rows {
		if i == 0 {
			for j, colCell := range row {
				colCell = strings.Trim(colCell, " ")
				if colCell != "" {
					nameToColMap[j] = colCell
				}
			}
			continue
		}
		track := &Track{}

		for j, colCell := range row {
			if trace() {
				if j > 0 {
					fmt.Print("\t")
				}
				fmt.Print(colCell)
			}
			track.Set(nameToColMap[j], colCell)
		}
		if trace() {
			fmt.Println()
		}
		tracks = append(tracks, track)
	}

	return tracks
}

func createdDate(tracks []*Track) (createdDate time.Time) {
	createdDate = time.Date(2099, time.December, 31, 23, 59, 59, 999999999, time.UTC)

	for _, track := range tracks {
		if !track.IsValid() {
			continue
		}

		modTime := time.Unix(0, track.ModTime)
		if createdDate.After(modTime) {
			createdDate = modTime
		}
	}

	return createdDate
}

func updatedDate(tracks []*Track) (updatedDate time.Time) {
	for _, track := range tracks {
		if !track.IsValid() {
			continue
		}

		modTime := time.Unix(0, track.ModTime)
		if updatedDate.Before(modTime) {
			updatedDate = modTime
		}
	}

	return updatedDate
}

func validTracks(tracks []*Track) (rv uint) {
	for _, track := range tracks {
		if track.IsValid() {
			rv++
		}
	}

	return rv
}

func loadDefaults(yamlFile string, genFilenames bool) {
	log.Infof("Reading %q", yamlFile)
	configData, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		log.Fatalf("Cannot read %q: %s", yamlFile, err)
	}

	err = yaml.Unmarshal(configData, defaults)
	if err != nil {
		log.Fatalf("Cannot process %q: %s", yamlFile, err)
	}

	base := basename(yamlFile)

	if defaults.OutputDir == "" {
		defaults.OutputDir = base
	}

	fi, err := os.Stat(defaults.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debugf("Creating directory %q", defaults.OutputDir)
			err = os.MkdirAll(defaults.OutputDir, os.ModePerm)
		}
		if err != nil {
			log.Fatalf("Cannot create directory %q: %s", defaults.OutputDir, err)
		}
	} else {
		if !fi.Mode().IsDir() {
			log.Fatalf("Cannot create directory %q: %s", defaults.OutputDir, "A file of the same name already exists")
		}
	}

	defaults.OutputDir += "/"

	if defaults.Image == "" {
		defaults.Image = base + defaultImageExt
	}

	if genFilenames {
		defaults.OutputFile = fmt.Sprintf(outputFileMask, defaults.OutputDir, base)
		defaults.PodcastFile = fmt.Sprintf(podcastFileMask, base)
	}

	if len(defaults.BaseURL) > 0 {
		if defaults.BaseURL[len(defaults.BaseURL)-1:] != "/" {
			defaults.BaseURL += "/"
		}
	}

	defaults.totalDiscs, err = strconv.ParseBool(defaults.TotalDiscs)
	if err != nil {
		log.Fatalf("Cannot parse total_discs in %q: %s", defaults.PodcastFile, err)
	}
	defaults.totalTracks, err = strconv.ParseBool(defaults.TotalTracks)
	if err != nil {
		log.Fatalf("Cannot parse total_tracks in %q: %s", defaults.PodcastFile, err)
	}

	defaults.Exiftool = normalizeDirectory(defaults.Exiftool)
	defaults.Ffmpeg = normalizeDirectory(defaults.Ffmpeg)
	defaults.Ffprobe = normalizeDirectory(defaults.Ffprobe)
	defaults.Image = normalizeDirectory(defaults.Image)
	defaults.OutputDir = normalizeDirectory(defaults.OutputDir)
	defaults.OutputFile = normalizeDirectory(defaults.OutputFile)
	defaults.PodcastFile = normalizeDirectory(defaults.PodcastFile)
	defaults.TracksFile = normalizeDirectory(defaults.TracksFile)
}

func processYAML(yamlFile string) {
	fp := readYAML(yamlFile)
	tracksFile := getTracksFilename(yamlFile)
	tracks := processTracks(fp, tracksFile)
	savePodcast(fp, tracks)
}

func readYAML(yamlFile string) (fp fpodcast.Podcast) {
	if yamlFile == "" {
		log.Fatal("Input file name is empty")
	}

	defaults = initDefaults

	yamlFile = normalizeDirectory(yamlFile)

	loadDefaults(yamlFile, true)

	dump("defaults@1=", defaults)

	_, err := os.Stat(localYAML)
	if err == nil {
		loadDefaults(localYAML, false)
		dump("defaults@2=", defaults)
	}

	defaults.BaseURL = strings.Trim(defaults.BaseURL, " ")
	if defaults.BaseURL == "" {
		log.Fatalf("No base_url defined in %q", yamlFile)
	}

	if defaults.PodcastFile == "" {
		log.Fatalf("No podcast_file defined in %q", yamlFile)
	}

	if defaults.OutputFile == "" {
		log.Fatalf("No output_file defined in %q", yamlFile)
	}

	log.Infof("Reading %q", defaults.PodcastFile)
	yamlData, err := ioutil.ReadFile(defaults.PodcastFile)
	if err != nil {
		log.Fatalf("Cannot read %q: %s", defaults.PodcastFile, err)
	}

	setDefaults(&fp)

	dump("fp@1=", fp)

	err = yaml.Unmarshal(yamlData, &fp)
	if err != nil {
		log.Fatalf("Cannot process %q: %s", defaults.PodcastFile, err)
	}
	dump("fp@2=", fp)

	// don't exit on image errors
	_ = processImage(&fp, defaults.Image, defaults.BaseURL)

	dump("fp@3=", fp)
	return fp
}

func getTracksFilename(yamlFile string) (tracksFile string) {
	tracksFile = defaults.TracksFile
	if tracksFile == "" {
		base := basename(yamlFile)

		for _, ext := range trackFileExtensions {
			tracksFile = fmt.Sprintf(tracksFileMask, base, ext)
			_, err := os.Stat(tracksFile)
			log.Debugf("Searching for %q", tracksFile)
			if err == nil {
				log.Debugf("Found %q", tracksFile)
				break
			}
			tracksFile = ""
		}
	}

	if tracksFile == "" {
		log.Fatalf("No tracks_file defined in %q", yamlFile)
	}
	return
}

func processTracks(fp fpodcast.Podcast, tracksFile string) (tracks []*Track) {
	ext := strings.ToLower(filepath.Ext(tracksFile))

	switch ext {
	case ".xls", ".xlsx":
		tracks = readXLS(tracksFile)
	case ".csv":
		tracks = readCSV(tracksFile)
	case ".txt":
		tracks = readTXT(tracksFile)
	default:
		log.Fatalf("Unsupported format for tracks file %q: %q", tracksFile, ext)
	}

	dump("tracks@1=", tracks)

	var lastTrack *Track

	for i, track := range tracks {
		if preProcessTrack(i+1, track, lastTrack) {
			lastTrack = track
		}
	}
	skipped := len(tracks) - int(validTracks(tracks))
	log.Infof("Preprocessed %d tracks (%d of %d rows were skipped)", validTracks(tracks), skipped, len(tracks))

	dump("tracks@2=", tracks)

	lastTrack = nil

	trackIndex := 1
	for _, track := range tracks {
		if !track.IsValid() {
			continue
		}
		processTrack(trackIndex, track, lastTrack, tracks)
		lastTrack = track
		trackIndex++
	}

	log.Infof("Processed %d tracks", validTracks(tracks))

	dump("tracks@3=", tracks)
	return
}

func savePodcast(fp fpodcast.Podcast, tracks []*Track) {
	log.Infof("Creating %q", defaults.OutputFile)
	xmlFD, err := os.Create(defaults.OutputFile)
	if err != nil {
		log.Fatalf("Cannot create %q: %s", defaults.OutputFile, err)
	}
	defer xmlFD.Close()

	// pubDate     := updatedDate.AddDate(0, 0, 3)

	pubDate := createdDate(tracks)
	lastBuildDate := updatedDate(tracks)

	// instantiate a new Podcast
	p := podcast.New(
		fp.Title,
		fp.Link,
		fp.Description,
		&pubDate,
		&lastBuildDate,
	)

	setPodcast(&p, &fp)

	dump("p=", p)

	for _, track := range tracks {
		if !track.IsValid() {
			continue
		}
		addTrack(&p, track)
		// d := pubDate.AddDate(0, 0, int(i + 1))
	}

	copyImage(&fp, defaults.OutputDir)

	// Podcast.Encode writes to an io.Writer
	err = p.Encode(xmlFD)
	if err != nil {
		log.Fatalf("Cannot write to %q: %s", xmlFD.Name(), err)
	}
	log.Infof("Saved %d tracks to %q", validTracks(tracks), defaults.OutputFile)
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

	logLevel := flag.Int("log", int(defaultLogLevel), "set log verbosity\n(6=trace, 5=debug, 4=info, 3=warn, 2=error, 1=fatal)")
	logCaller := flag.Bool("logcaller", false, "log file/function/line")

	flag.Parse()

	if *logLevel != int(defaultLogLevel) {
		log.SetLevel(log.Level(*logLevel))
	}

	if *logCaller {
		log.SetReportCaller(true)
	}

	if flag.NArg() == 0 {
		processYAML(defaultYAML)
		return
	}

	for _, arg := range flag.Args() {
		processYAML(arg)
	}
}
