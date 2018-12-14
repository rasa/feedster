// Program feedster tags mp3s from csv/xls file and gens podcast xml
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/bogem/id3v2"
	"github.com/eduncan911/podcast"
	"github.com/gocarina/gocsv"
	fpodcast "github.com/rasa/feedster/podcast"
	"github.com/rasa/feedster/version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	copyrightDescription = "Copyright"
	defaultImageExt      = ".jpg"
	defaultLogLevel      = log.InfoLevel
	defaultMimeType      = "image/jpeg"
	defaultYAML          = "default.yaml"
	durationMask         = "%02d:%02d:%02d"
	localYAML            = "local.yaml"

	outputFileMask  = "%s.xml"
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
	Explicit       string `yaml:"explicit,omitempty"`
	Ffprobe        string `yaml:"ffprobe,omitempty"`
	Generator      string `yaml:"generator,omitempty"`
	Image          string `yaml:"image,omitempty"`
	Language       string `yaml:"language,omitempty"`
	ManagingEditor string `yaml:"managingeditor,omitempty"`
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
	Complete:      "yes",
	CopyrightMask: "Copyright (c) & (p) %d, %s", // or: Copyright © & ℗ %d, %s
	DiscNumber:    "1",
	Explicit:      "no",
	Ffprobe:       "ffprobe",
	//Image:         defaultImage,
	Language: "en-us",
	// OutputFile:    "default.xml",
	// PodcastFile:   "podcast.yaml",
	TotalDiscs:  "true",
	TotalTracks: "true",
	TrackNo:     "1",
	// TracksFile:     "tracks.csv",
	TTL: "1",
}

var defaults = initDefaults

var trackFileExtensions = []string{
	".csv",
	".xlsx",
	".xls",
}

/*

call tree:

main
	logInit
	processYAML
		loadDefaults
		setDefaults
		processImage
		readCSV or readXLS
		preProcessTrack
			normalizeTrack
			setTrackDefaults
		processTrack
			normalizeTrack
			setTags
				totalDiscs
				totalTracks
				addTextFrame
			addFrontCover
		createdDate
		updatedDate
		setPodcast
		addTrack
			newName
			copyFile
		validTracks
*/

func l(level log.Level) bool {
	return log.IsLevelEnabled(level)
}

func debug() bool {
	return log.IsLevelEnabled(log.DebugLevel)
}

func trace() bool {
	return log.IsLevelEnabled(log.TraceLevel)
}

func dump(s string, x interface{}) {
	if !debug() {
		return
	}

	if s != "" {
		log.Infoln(s)
	}
	if x == nil {
		return
	}

	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		log.Errorln("marshal error: ", err)
		return
	}
	log.Info(string(b))
}

func normalizeTrack(track *Track) (err error) {
	track.OriginalFilename = track.Filename
	track.Filename = normalizeFilename(track.Filename)
	return nil
}

func setCopyright(track *Track, defaults *Default, year int) {
	if track.Copyright == "" {
		if defaults.Copyright != "" {
			// track.Copyright := html.EscapeString(defaults.Copyright)
			track.Copyright = defaults.Copyright
		} else {
			// track.Copyright = fmt.Sprintf(html.EscapeString(defaults.CopyrightMask), year, track.Artist)
			track.Copyright = fmt.Sprintf(defaults.CopyrightMask, year, track.Artist)
		}
	}
}

func setTrackDefaults(track *Track, lastTrack *Track) bool {
	if track.Title == "" {
		if track.Filename != "" {
			track.Title = basename(path.Base(track.Filename))
		}
	}
	if track.Description == "" {
		// per https://github.com/eduncan911/podcast/blob/master/podcast.go#L270
		track.Description = track.Title
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
	if track.Track == "" {
		i, err := strconv.Atoi(lastTrack.Track)
		if err == nil {
			track.Track = strconv.Itoa(i + 1)
		}
	}
	if track.Track == "" {
		track.Track = defaults.TrackNo
	}
	if track.DiscNumber == "" {
		track.DiscNumber = lastTrack.DiscNumber
	} else {
		if track.DiscNumber != lastTrack.DiscNumber {
			track.Track = "1"
		}
	}
	if track.DiscNumber == "" {
		track.DiscNumber = defaults.DiscNumber
	}
	if track.AlbumTitle == "" {
		track.AlbumTitle = lastTrack.AlbumTitle
	}
	if track.Copyright == "" {
		track.Copyright = lastTrack.Copyright
	}

	if track.Genre == "" {
		track.Genre = lastTrack.Genre
	}

	var year int

	if track.Filename == "" {
		if track.Year != "" {
			year, _ = strconv.Atoi(track.Year)
		} else {
			year := time.Now().Year()
			track.Year = strconv.Itoa(year)
		}
		setCopyright(track, defaults, year)
		track.DurationMilliseconds = 0
		track.Duration = "00:00:00"
		return false
	}

	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}
	track.ModTime = fi.ModTime().UnixNano()
	track.OriginalFileSize = fi.Size()
	track.FileSize = fi.Size()

	if track.Year == "" {
		year = fi.ModTime().Year()
		track.Year = strconv.Itoa(year)
	}

	setCopyright(track, defaults, year)

	cmd := defaults.Ffprobe
	out, err := exec.Command(cmd, track.Filename).CombinedOutput()
	if err != nil {
		log.Fatalf("Command failed: %s: %s", cmd, err)
	}

	re := regexp.MustCompile(`Duration:\s*([\d]+):([\d]+):([\d]+)`)
	b := re.FindSubmatch(out)

	if len(b) <= 3 {
		log.Warnf("Failed to get duration for %s", track.Filename)
		track.DurationMilliseconds = 0
		track.Duration = "00:00:00"
		return false
	}

	hours, _ := strconv.Atoi(string(b[1]))
	minutes, _ := strconv.Atoi(string(b[2]))
	seconds, _ := strconv.Atoi(string(b[3]))
	hundredths := int(0)

	re = regexp.MustCompile(`Duration:\s*[\d]+:[\d]+:[\d]+\.([\d]+)`)
	b = re.FindSubmatch(out)

	if len(b) == 2 {
		hundredths, _ = strconv.Atoi(string(b[1]))
	}

	track.DurationMilliseconds = int64(1000*((hours*3600)+(minutes*60)+seconds) + (hundredths * 10))

	if hundredths > 0 {
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

	track.Duration = fmt.Sprintf(durationMask, hours, minutes, seconds)

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
		log.Warnf("Unknown id3v2 ID: '%s'\n", id)
	}
	tag.AddTextFrame(tid, id3v2.EncodingUTF8, text)
}

func setTags(tag *id3v2.Tag, track *Track, defaults *Default, tracks []*Track) {
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

	log.Tracef("totalDiscs=%d\n", totalDiscs)
	log.Tracef("totalTracks=%d\n", totalTracks)
	log.Tracef("discNumber=%s\n", discNumber)
	log.Tracef("trackNumber=%s\n", trackNumber)

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

	subtitle := ""
	if track.Subtitle != "" {
		subtitle = track.Subtitle
	}
	if track.Description != "" {
		if subtitle != "" {
			subtitle += " / "
		}
		subtitle += track.Description
	}
	addTextFrame(tag, "Subtitle/Description refinement", subtitle)

	addTextFrame(tag, "Track number/Position in set", trackNumber)

	// system defined fields:

	t := time.Unix(0, track.ModTime)
	MMDD := fmt.Sprintf("%02d%02d", t.Month(), t.Day())
	HHMM := fmt.Sprintf("%02d%02d", t.Hour(), t.Minute())

	addTextFrame(tag, "Date", MMDD)
	addTextFrame(tag, "Time", HHMM)

	addTextFrame(tag, "Original filename", track.OriginalFilename)
	addTextFrame(tag, "Size", strconv.FormatInt(track.OriginalFileSize, 10))
	addTextFrame(tag, "Length", strconv.FormatInt(track.DurationMilliseconds, 10))

	// Set comment frame.
	comment := id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    bCF47ToISO3(defaults.Language),
		Description: copyrightDescription,
		Text:        track.Copyright,
	}
	tag.AddCommentFrame(comment)
}

func addFrontCover(filename string) (pic *id3v2.PictureFrame, err error) {
	_, err = os.Stat(filename)
	if err != nil {
		log.Warnf("Cannot read %s: %s", filename, err)
		return nil, nil
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mimeType := mime.TypeByExtension(ext)

	if mimeType == "" {
		log.Warnf("Unknown image format %s: %s", filename, err)
		mimeType = defaultMimeType
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

func preProcessTrack(trackIndex int, track *Track, lastTrack *Track, tracks []*Track) {
	if track.Filename != "" {
		log.Infof("Preprocessing track %d: %s\n", trackIndex, track.Filename)
		normalizeTrack(track)
	}

	setTrackDefaults(track, lastTrack)

	if track.Filename == "" {
		log.Infof("Skipping track %d: %s\n", trackIndex, "Filename is empty")
	}
}

func processTrack(trackIndex int, track *Track, lastTrack *Track, tracks []*Track) {
	log.Infof("Processing track %d: %s\n", trackIndex, track.Filename)

	tag, err := id3v2.Open(track.Filename, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}

	setTags(tag, track, defaults, tracks)

	if defaults.Image != "" {
		pic, err := addFrontCover(defaults.Image)
		if err == nil && pic != nil {
			tag.AddAttachedPicture(*pic)
		}
	}

	// Write it to file.
	err = tag.Save()
	if err != nil {
		log.Fatalf("Cannot save tags for %s: %s", track.Filename, err)
	}
	tag.Close()
	if track.ModTime != 0 {
		modTime := time.Unix(0, track.ModTime)
		err = os.Chtimes(track.Filename, modTime, modTime)
		if err != nil {
			log.Warnf("Cannot set time for %s: %s", track.Filename, err)
		}
	}

	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}
	track.FileSize = fi.Size()
}

func setDefaults(fp *fpodcast.Podcast, defaults *Default) {
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
		p.AddAuthor(fp.IOwner.Name, fp.IOwner.Email)
	}
}

func newName(track *Track, defaults *Default) (newName string, err error) {
	if defaults.RenameMask == "" {
		return track.Filename, nil
	}

	newName = defaults.RenameMask
	for k, v := range track.Fields() {
		regex := fmt.Sprintf(`{(%s)([^}]*)}`, k)
		re := regexp.MustCompile(regex)
		b := re.FindStringSubmatch(newName)
		if len(b) < 3 {
			continue
		}
		name := b[1]
		if name != k {
			continue
		}
		log.Tracef("k=%v\n", k)
		log.Tracef("v=%v\n", v)
		log.Tracef("regex=%v\n", regex)
		log.Tracef("name=%s\n", name)
		log.Tracef("b=%v\n", b)
		format := b[2]
		if format == "" {
			format = "%s"
		}
		log.Tracef("format=%v\n", format)
		underline := strings.Index(format, "_") > -1
		if underline {
			format = strings.Replace(format, "_", "", -1)
		}
		dash := strings.Index(format, "-") > -1
		if dash {
			format = strings.Replace(format, "-", "", -1)
		}
		lastChar := format[len(format)-1:]
		var s string
		switch lastChar {
		case "t":
			b, err := strconv.ParseBool(v)
			if err != nil {
				log.Warnln(err)
				return "", err
			}
			s = fmt.Sprintf(format, b)
		case "b", "c", "d", "o", "q", "x", "X", "U":
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				log.Warnln(err)
				return "", err
			}
			s = fmt.Sprintf(format, i)
		case "e", "E", "f", "F", "g", "G":
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				log.Warnln(err)
				return "", err
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
		log.Tracef("s=%v\n", s)
		newName = strings.Replace(newName, b[0], s, -1)
	}

	return newName, nil
}

func addTrack(p *podcast.Podcast, track *Track, defaults *Default) {
	pubDate := time.Unix(0, track.ModTime)
	item := podcast.Item{
		Title:       track.Title,
		Description: track.Description,
		ISubtitle:   track.Subtitle,
		PubDate:     &pubDate,
	}
	// @TODO(rasa) change to p.Image.URL
	item.AddImage(defaults.BaseURL + defaults.Image)
	if track.Duration != "" {
		item.IDuration = track.Duration
	}
	item.AddSummary(track.Summary)

	newName, err := newName(track, defaults)
	if err == nil {
		if newName != "" {
			if !strings.EqualFold(track.Filename, newName) {
				log.Infof("Copying %s to %s\n", track.Filename, newName)
				err = copyFile(track.Filename, newName)
				if err != nil {
					os.Exit(1)
				}
				track.Filename = newName
			}
		}
	}

	// add a Download to the Item
	item.AddEnclosure(defaults.BaseURL+track.Filename, podcast.MP3, track.FileSize)

	// add the Item and check for validation errors
	_, err = p.AddItem(item)
	if err != nil {
		log.Fatalf("Cannot add track %s: %s", track.Filename, err)
	}
}

func processImage(fp *fpodcast.Podcast, defaults *Default) (err error) {
	if defaults.Image == "" {
		if fp.Image == nil {
			return nil
		}
		if fp.Image.URL == "" {
			return nil
		}
	}

	if defaults.Image != "" {
		if fp.Image == nil {
			fp.Image = &fpodcast.Image{}
		}
		if fp.Image.URL == "" {
			fp.Image.URL = defaults.BaseURL + defaults.Image
		}
	}

	basename := path.Base(fp.Image.URL)
	reader, err := os.Open(basename)
	if err != nil {
		log.Warnf("Cannot open %s: %s", basename, err)
		return err
	}
	defer reader.Close()

	im, _, err := image.DecodeConfig(reader)
	if err != nil {
		log.Warnf("Cannot read %s: %v\n", basename, err)
		return err
	}

	fp.Image.Width = im.Width
	fp.Image.Height = im.Height

	if fp.Image.Width < imageWidthMin {
		err = fmt.Errorf("image %s's width (%d) needs to be %d or greater", basename, fp.Image.Width, imageWidthMin)
		log.Warnln(err)
	}
	if fp.Image.Width > imageWidthMax {
		err = fmt.Errorf("image %s's width (%d) needs to be %d or less", basename, fp.Image.Width, imageWidthMax)
		log.Warnln(err)
	}
	if fp.Image.Height < imageHeightMin {
		err = fmt.Errorf("image %s's height (%d) needs to be %d or greater", basename, fp.Image.Height, imageHeightMin)
		log.Warnln(err)
	}
	if fp.Image.Height > imageHeightMax {
		err = fmt.Errorf("image %s's height (%d) needs to be %d or less", basename, fp.Image.Height, imageHeightMax)
		log.Warnln(err)
	}

	return err
}

func readCSV(csvFile string) (tracks []*Track) {
	log.Infof("Reading %s\n", csvFile)
	csvFD, err := os.Open(csvFile)
	if err != nil {
		log.Fatalf("Cannot read %s: %s", csvFile, err)
	}
	defer csvFD.Close()

	err = gocsv.Unmarshal(csvFD, &tracks)
	if err != nil { // Load track from file
		log.Fatalf("Cannot process %s: %s", csvFile, err)
	}

	return tracks
}

func readXLS(xlsFile string) (tracks []*Track) {
	log.Infof("Reading %s\n", xlsFile)
	xlsx, err := excelize.OpenFile(xlsFile)
	if err != nil {
		log.Fatalf("Cannot read %s: %s", xlsFile, err)
	}

	var sheetName string
	for _, sheetName = range xlsx.GetSheetMap() {
		// use the first sheet in the workbook
		break
	}
	if sheetName == "" {
		log.Fatalf("Cannot find any sheets in %s", xlsFile)
	}

	nameToColMap := make(map[int]string)

	rows := xlsx.GetRows(sheetName)
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
	log.Infof("Reading %s\n", yamlFile)
	configData, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		log.Fatalf("Cannot read %s: %s", yamlFile, err)
	}

	err = yaml.Unmarshal(configData, defaults)
	if err != nil {
		log.Fatalf("Cannot process %s: %s", yamlFile, err)
	}

	base := basename(yamlFile)

	if defaults.Image == "" {
		defaults.Image = base + defaultImageExt
	}

	if genFilenames {
		defaults.OutputFile = fmt.Sprintf(outputFileMask, base)
		defaults.PodcastFile = fmt.Sprintf(podcastFileMask, base)
	}

	if len(defaults.BaseURL) > 0 {
		if defaults.BaseURL[len(defaults.BaseURL)-1:] != "/" {
			defaults.BaseURL += "/"
		}
	}

	defaults.totalDiscs, err = strconv.ParseBool(defaults.TotalDiscs)
	if err != nil {
		log.Fatalf("Parse error parsing total_discs in %s: %s", defaults.PodcastFile, err)
	}
	defaults.totalTracks, err = strconv.ParseBool(defaults.TotalTracks)
	if err != nil {
		log.Fatalf("Parse error parsing total_tracks in %s: %s", defaults.PodcastFile, err)
	}
}

func processYAML(yamlFile string) {
	if yamlFile == "" {
		log.Fatalln("Input file name is empty")
	}

	defaults = initDefaults

	loadDefaults(yamlFile, true)

	dump("defaults@1=", defaults)

	_, err := os.Stat(localYAML)
	if err == nil {
		loadDefaults(localYAML, false)
		dump("defaults@2=", defaults)
	}

	defaults.BaseURL = strings.Trim(defaults.BaseURL, " ")
	if defaults.BaseURL == "" {
		log.Fatalf("No base_url defined in %s", yamlFile)
	}

	if defaults.PodcastFile == "" {
		log.Fatalf("No podcast_file defined in %s", yamlFile)
	}

	if defaults.OutputFile == "" {
		log.Fatalf("No output_file defined in %s\n", yamlFile)
	}

	log.Infof("Reading %s\n", defaults.PodcastFile)
	yamlData, err := ioutil.ReadFile(defaults.PodcastFile)
	if err != nil {
		log.Fatalf("Cannot read %s: %s", defaults.PodcastFile, err)
	}

	var fp fpodcast.Podcast

	setDefaults(&fp, defaults)

	dump("fp@1=", fp)

	err = yaml.Unmarshal(yamlData, &fp)
	if err != nil {
		log.Fatalf("Cannot process %s: %s", defaults.PodcastFile, err)
	}
	dump("fp@2=", fp)

	// don't exit on image errors
	_ = processImage(&fp, defaults)

	dump("fp@3=", fp)

	var tracks []*Track

	tracksFile := defaults.TracksFile
	if tracksFile == "" {
		base := basename(yamlFile)

		for _, ext := range trackFileExtensions {
			tracksFile = fmt.Sprintf(tracksFileMask, base, ext)
			_, err := os.Stat(tracksFile)
			log.Debugf("Searching for %s\n", tracksFile)
			if err == nil {
				log.Debugf("Found %s\n", tracksFile)
				break
			}
			tracksFile = ""
		}
	}

	if tracksFile == "" {
		log.Fatalf("No tracks_file defined in %s\n", yamlFile)
	}

	ext := strings.ToLower(filepath.Ext(tracksFile))

	switch ext {
	case ".csv":
		tracks = readCSV(tracksFile)
	case ".xls", ".xlsx":
		tracks = readXLS(tracksFile)
	default:
		log.Fatalf("Unsupported format for tracks file %s: '%s'\n", tracksFile, ext)
	}

	dump("tracks@1=", tracks)

	lastTrack := &Track{}

	for i, track := range tracks {
		preProcessTrack(i+1, track, lastTrack, tracks)
		lastTrack = track
	}

	dump("tracks@2=", tracks)

	lastTrack = &Track{}

	for i, track := range tracks {
		if !track.IsValid() {
			log.Infof("Skipping track %d: %s: %s", i+1, track.Filename, track.Error())
			continue
		}

		processTrack(i+1, track, lastTrack, tracks)
		lastTrack = track
	}

	// pubDate     := updatedDate.AddDate(0, 0, 3)

	dump("tracks@3=", tracks)

	log.Infof("Creating %s\n", defaults.OutputFile)
	xmlFD, err := os.Create(defaults.OutputFile)
	if err != nil {
		log.Fatalf("Cannot create %s: %s", defaults.OutputFile, err)
	}
	defer xmlFD.Close()

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
		addTrack(&p, track, defaults)
		// d := pubDate.AddDate(0, 0, int(i + 1))
	}

	// Podcast.Encode writes to an io.Writer
	err = p.Encode(xmlFD)
	if err != nil {
		log.Fatalf("Cannot write to %s: %s", xmlFD.Name(), err)
	}
	log.Infof("Saved %d of %d tracks to %s", validTracks(tracks), len(tracks), defaults.OutputFile)
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
	logInit()

	if flag.NArg() == 0 {
		processYAML(defaultYAML)
		return
	}

	for _, arg := range flag.Args() {
		processYAML(arg)
	}
}
