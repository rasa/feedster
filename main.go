// Program feedmp3s tags mp3s from csv file, gens xml too!
package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/bogem/id3v2"
	"github.com/eduncan911/podcast"
	"github.com/gocarina/gocsv"
	fpodcast "github.com/rasa/feedster/podcast"
	"github.com/rasa/feedster/version"
	"gopkg.in/yaml.v2"
)

const (
	logDebug       = 1
	logLevel       = 0 // logDebug
	imageWidthMin  = 1400
	imageHeightMin = 1400
	imageWidthMax  = 3000
	imageHeightMax = 3000

	copyrightDescription = "Copyright"
	defaultImage         = "default.jpg"
	defaultImageType     = "jpg"
	defaultConfigYAML    = "config.yaml"
	localYAML            = "local.yaml"
	durationMask         = "%02d:%02d:%02d"
)

var mimeMap = map[string]string{
	"jpg":  "image/jpeg",
	"jepg": "image/jpeg",
	"png":  "image/jpeg",
}

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
	InputFile      string `yaml:"input_file,omitempty"`
	Language       string `yaml:"language,omitempty"`
	ManagingEditor string `yaml:"managingeditor,omitempty"`
	OutputFile     string `yaml:"output_file,omitempty"`
	RenameMask     string `yaml:"rename_mask,omitempty"`
	SettingsFile   string `yaml:"settings_file,omitempty"`
	TotalDiscs     string `yaml:"total_discs,omitempty"`
	TotalTracks    string `yaml:"total_tracks,omitempty"`
	TrackNo        string `yaml:"track_no,omitempty"`
	TTL            string `yaml:"ttl,omitempty"`
	WebMaster      string `yaml:"webmaster,omitempty"`
	totalDiscs     bool
	totalTracks    bool
}

var defaults = &Default{
	Complete:      "yes",
	CopyrightMask: "Copyright (c) & (p) %d, %s",
	DiscNumber:    "1",
	Explicit:      "no",
	Ffprobe:       "ffprobe",
	Image:         defaultImage,
	InputFile:     "default.csv",
	Language:      "en-us",
	OutputFile:    "default.xml",
	SettingsFile:  "settings.yaml",
	TotalDiscs:    "true",
	TotalTracks:   "true",
	TrackNo:       "1",
	TTL:           "1",
}

func logging(level uint) bool {
	return (logLevel & level) == level
}

func dump(s string, x interface{}) {
	if !logging(logDebug) {
		return
	}

	if s != "" {
		log.Println(s)
	}
	if x == nil {
		return
	}

	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		log.Println("error: ", err)
	}
	log.Print(string(b))
}

func normalizeTrack(track *Track) (err error) {
	track.OriginalFilename = track.Filename
	newname := normalizeFilename(track.Filename)
	if track.Filename == newname {
		// file doesn't need be renamed
		return nil
	}
	fi, err := os.Stat(track.Filename)
	if err != nil {
		_, err = os.Stat(newname)
		if err == nil {
			// file has already been renamed, and the user deleted the original
			track.Filename = newname
			return nil
		}
		log.Fatalf("Failed to read %s: %s", oldname, err)
	}
	oldTime := fi.ModTime()
	err = os.Rename(track.Filename, newname)
	if err != nil {
		log.Fatalf("Cannot rename %s to %s: %s", track.Filename, newname, err)
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
	track.Filename = newname
	return nil
}

func setTrackDefaults(track *Track, lastTrack *Track, year int) bool {
	if track.Title == "" {
		log.Printf("Skipping track %s: title is empty\n", track.Filename)
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
	if track.Copyright == "" {
		if defaults.Copyright != "" {
			// track.Copyright := html.EscapeString(defaults.Copyright)
			track.Copyright = defaults.Copyright
		} else {
			// track.Copyright = fmt.Sprintf(html.EscapeString(defaults.CopyrightMask), year, track.Artist)
			track.Copyright = fmt.Sprintf(defaults.CopyrightMask, year, track.Artist)
		}
	}
	if track.Genre == "" {
		track.Genre = lastTrack.Genre
	}
	if track.Year == "" {
		track.Year = lastTrack.Year
	}
	if track.Year == "" {
		track.Year = strconv.Itoa(year)
	}

	cmd := defaults.Ffprobe
	out, err := exec.Command(cmd, track.Filename).CombinedOutput()
	if err != nil {
		log.Fatalf("Command failed: %s: %s", cmd, err)
	}
	re := regexp.MustCompile(`Duration:\s*([\d]+):([\d]+):([\d]+)\.([\d]+)`)

	b := re.FindSubmatch(out)
	if len(b) > 4 {
		hours, _ := strconv.Atoi(string(b[1]))
		minutes, _ := strconv.Atoi(string(b[2]))
		seconds, _ := strconv.Atoi(string(b[3]))
		hundredths, _ := strconv.Atoi(string(b[4]))

		track.DurationMilliseconds = int64(1000*((hours*3600)+(minutes*60)+seconds) + (hundredths * 10))

		if hundredths > 0 {
			seconds++
		}
		if seconds > 60 {
			minutes++
			seconds = 0
		}
		if minutes > 60 {
			hours++
			minutes = 0
		}

		track.Duration = fmt.Sprintf(durationMask, hours, minutes, seconds)
	}
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

	if logging(logDebug) {
		fmt.Printf("totalDiscs=%d\n", totalDiscs)
		fmt.Printf("totalTracks=%d\n", totalTracks)
		fmt.Printf("discNumber=%s\n", discNumber)
		fmt.Printf("trackNumber=%s\n", trackNumber)
	}

	tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, track.AlbumArtist)
	tag.SetAlbum(track.AlbumTitle)
	tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, discNumber)
	tag.AddTextFrame(tag.CommonID("Album/Movie/Show title"), id3v2.EncodingUTF8, track.AlbumTitle)
	tag.SetArtist(track.Artist)
	tag.AddTextFrame(tag.CommonID("Copyright message"), id3v2.EncodingUTF8, track.Copyright)
	//panics:
	//tag.AddTextFrame(tag.CommonID("Comments"), id3v2.EncodingUTF8, track.Copyright)
	tag.SetGenre(track.Genre)
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, trackNumber)
	tag.SetTitle(track.Title)
	tag.SetYear(track.Year)

	// Set comment frame.
	comment := id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    bCF47ToISO3(defaults.Language),
		Description: copyrightDescription,
		Text:        track.Copyright,
	}
	tag.AddCommentFrame(comment)

	tag.AddTextFrame(tag.CommonID("Composer"), id3v2.EncodingUTF8, track.Artist)
	if defaults.EncodedBy != "" {
		tag.AddTextFrame(tag.CommonID("Encoded by"), id3v2.EncodingUTF8, defaults.EncodedBy)
	}
	tag.AddTextFrame(tag.CommonID("Language"), id3v2.EncodingUTF8, defaults.Language)
	tag.AddTextFrame(tag.CommonID("Original filename"), id3v2.EncodingUTF8, track.OriginalFilename)
}

func addFrontCover(filename string) (pic *id3v2.PictureFrame, err error) {
	_, err = os.Stat(filename)
	if err != nil {
		if filename != defaultImage {
			log.Printf("Cannot read %s: %s", filename, err)
		}
		return nil, nil
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	mimeType, ok := mimeMap[ext]
	if !ok {
		log.Printf("Unknown image format %s: %s", filename, err)
		mimeType = mimeMap[defaultImageType]
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
	if track.Filename == "" {
		log.Printf("Skipping track %d: %s\n", trackIndex, "Filename is empty")
		return
	}

	log.Printf("Preprocessing track %d: %s\n", trackIndex, track.Filename)

	normalizeTrack(track)

	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}
	track.ModTime = fi.ModTime().UnixNano()
	track.FileSize = fi.Size()
	year := fi.ModTime().Year()

	setTrackDefaults(track, lastTrack, year)
}

func processTrack(trackIndex int, track *Track, lastTrack *Track, tracks []*Track) {
	log.Printf("Processing track %d: %s\n", trackIndex, track.Filename)

	normalizeTrack(track)

	fi, err := os.Stat(track.Filename)
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}
	modTime := fi.ModTime()
	track.ModTime = modTime.UnixNano()
	tag, err := id3v2.Open(track.Filename, id3v2.Options{Parse: true})
	if err != nil {
		log.Fatalf("Cannot open %s: %s", track.Filename, err)
	}

	setTags(tag, track, defaults, tracks)

	pic, err := addFrontCover(defaults.Image)
	if err == nil && pic != nil {
		tag.AddAttachedPicture(*pic)
	}
	// Write it to file.
	err = tag.Save()
	if err != nil {
		log.Fatalf("Cannot save tags for %s: %s", track.Filename, err)
	}
	tag.Close()
	err = os.Chtimes(track.Filename, modTime, modTime)
	if err != nil {
		log.Fatalf("Cannot set time for %s: %s", track.Filename, err)
	}
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
	p.LastBuildDate = fp.LastBuildDate
	p.ManagingEditor = fp.ManagingEditor
	p.PubDate = fp.PubDate
	p.Rating = fp.Rating
	p.SkipHours = fp.SkipHours
	p.SkipDays = fp.SkipDays
	p.TTL = fp.TTL
	p.WebMaster = fp.WebMaster
	p.IAuthor = fp.IAuthor
	// p.ISubtitle = fp.ISubtitle
	//if fp.ISubtitle != "" {
	p.AddSubTitle(fp.ISubtitle)
	//}
	p.IBlock = fp.IBlock
	p.IDuration = fp.IDuration
	p.IExplicit = fp.IExplicit
	p.IComplete = fp.IComplete
	p.INewFeedURL = fp.INewFeedURL

	if fp.Image.URL != "" {
		p.AddImage(fp.Image.URL)
	}
	// image := podcast.Image(*fp.Image)
	// p.Image = &image
	/*
		if fp.TextInput.Name != "" {
			textInput := podcast.TextInput(*fp.TextInput)
			p.TextInput = &textInput
		}
	*/
	if fp.AtomLink.HREF != "" {
		p.AddAtomLink(fp.AtomLink.HREF)
	}
	// atomLink := podcast.AtomLink(*fp.AtomLink)
	// p.AtomLink = &atomLink
	//summary := podcast.ISummary(*fp.ISummary)
	// p.ISummary = &summary
	p.AddSummary(fp.ISummary.Text)
	//iimage := podcast.IImage(*fp.IImage)
	//p.IImage = &iimage
	iowner := podcast.Author(*fp.IOwner)
	p.IOwner = &iowner
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
		if logging(logDebug) {
			//log.Printf("k=%v\n", k)
			//log.Printf("v=%v\n", v)
			//log.Printf("regex=%v\n", regex)
			//log.Printf("name=%s\n", name)
			//log.Printf("b=%v\n", b)
		}
		format := b[2]
		if format == "" {
			format = "%s"
		}
		if logging(logDebug) {
			//log.Printf("format=%v\n", format)
		}
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
				log.Println(err)
				return "", err
			}
			s = fmt.Sprintf(format, b)
		case "b", "c", "d", "o", "q", "x", "X", "U":
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				log.Println(err)
				return "", err
			}
			s = fmt.Sprintf(format, i)
		case "e", "E", "f", "F", "g", "G":
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				log.Println(err)
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
		if logging(logDebug) {
			//log.Printf("s=%v\n", s)
		}
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
	item.AddImage(defaults.BaseURL + defaults.Image)
	if track.Duration != "" {
		item.IDuration = track.Duration
	}
	item.AddSummary(track.Summary)

	newName, err := newName(track, defaults)
	if err == nil {
		if newName != "" {
			if !strings.EqualFold(track.Filename, newName) {
				log.Printf("Copying %s to %s\n", track.Filename, newName)
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
		log.Printf("Cannot add track %s: %s", track.Filename, err)
	}
}

func processImage(fp *fpodcast.Podcast) (err error) {
	err = nil

	if fp.Image.URL == "" {
		return err
	}

	basename := path.Base(fp.Image.URL)
	reader, err := os.Open(basename)
	if err != nil {
		log.Printf("Cannot open %s: %s", basename, err)
		return err
	}
	defer reader.Close()

	im, _, err := image.DecodeConfig(reader)
	if err != nil {
		log.Printf("Cannot read %s: %v\n", basename, err)
		return err
	}

	fp.Image.Width = im.Width
	fp.Image.Height = im.Height

	if fp.Image.Width < imageWidthMin {
		err = fmt.Errorf("image %s's width (%d) needs to be %d or greater", basename, fp.Image.Width, imageWidthMin)
		log.Println(err)
	}
	if fp.Image.Width > imageWidthMax {
		err = fmt.Errorf("image %s's width (%d) needs to be %d or less", basename, fp.Image.Width, imageWidthMax)
		log.Println(err)
	}
	if fp.Image.Height < imageHeightMin {
		err = fmt.Errorf("image %s's height (%d) needs to be %d or greater", basename, fp.Image.Height, imageHeightMin)
		log.Println(err)
	}
	if fp.Image.Height > imageHeightMax {
		err = fmt.Errorf("image %s's height (%d) needs to be %d or less", basename, fp.Image.Height, imageHeightMax)
		log.Println(err)
	}

	return err
}

func readCSV(csvFile string) (tracks []*Track) {
	log.Printf("Reading %s\n", csvFile)
	csvFD, err := os.Open(csvFile)
	if err != nil {
		log.Panicf("Cannot read %s: %s", csvFile, err)
	}
	defer csvFD.Close()

	err = gocsv.Unmarshal(csvFD, &tracks)
	if err != nil { // Load track from file
		log.Panicf("Cannot process %s: %s", csvFile, err)
	}

	return tracks
}

func readXLS(xlsFile string) (tracks []*Track) {
	log.Printf("Reading %s\n", xlsFile)
	xlsx, err := excelize.OpenFile(xlsFile)
	if err != nil {
		log.Panicf("Cannot read %s: %s", xlsFile, err)
	}

	var sheetName string
	for _, sheetName = range xlsx.GetSheetMap() {
		// use the first sheet in the workbook
		break
	}
	if sheetName == "" {
		log.Panicf("Cannot find any sheets in %s", xlsFile)
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
			// fmt.Print(colCell, "\t")
			track.Set(nameToColMap[j], colCell)
		}
		// fmt.Println()
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

func loadDefaults(configFile string) {
	log.Printf("Reading %s\n", configFile)
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Panicf("Cannot read %s: %s", configFile, err)
	}

	err = yaml.Unmarshal(configData, defaults)
	if err != nil {
		log.Panicf("Cannot process %s: %s", configFile, err)
	}

	if len(defaults.BaseURL) > 0 {
		if defaults.BaseURL[len(defaults.BaseURL)-1:] != "/" {
			defaults.BaseURL += "/"
		}
	}
	defaults.totalDiscs, err = strconv.ParseBool(defaults.TotalDiscs)
	if err != nil {
		log.Fatalf("Parse error parsing total_discs in %s: %s", defaults.SettingsFile, err)
	}
	defaults.totalTracks, err = strconv.ParseBool(defaults.TotalTracks)
	if err != nil {
		log.Fatalf("Parse error parsing total_tracks in %s: %s", defaults.SettingsFile, err)
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

	loadDefaults(defaultConfigYAML)

	dump("defaults@1=", defaults)

	_, err := os.Stat(localYAML)
	if err == nil {
		loadDefaults(localYAML)
		dump("defaults@2=", defaults)
	}

	inputFile := defaults.InputFile
	if len(os.Args) >= 2 {
		inputFile = os.Args[1]
	}

	log.Printf("Reading %s\n", defaults.SettingsFile)
	yamlData, err := ioutil.ReadFile(defaults.SettingsFile)
	if err != nil {
		log.Panicf("Cannot read %s: %s", defaults.SettingsFile, err)
	}

	var fp fpodcast.Podcast

	setDefaults(&fp, defaults)

	dump("fp@1=", fp)

	err = yaml.Unmarshal(yamlData, &fp)
	if err != nil {
		log.Panicf("Cannot process %s: %s", defaults.SettingsFile, err)
	}

	// don't exit on image errors
	_ = processImage(&fp)

	dump("fp@2=", fp)

	var tracks []*Track

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(inputFile)), ".")

	switch ext {
	case "csv":
		tracks = readCSV(inputFile)
	case "xls", "xlsx":
		tracks = readXLS(inputFile)
	default:
		log.Fatalf("Unknown file format: '%s'\n", ext)
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
			log.Printf("Skipping track %d: %s (%s)", i+1, track.Filename, track.Error())
			continue
		}

		processTrack(i+1, track, lastTrack, tracks)
		lastTrack = track
	}

	// pubDate     := updatedDate.AddDate(0, 0, 3)

	dump("tracks@3=", tracks)

	log.Printf("Creating %s\n", defaults.OutputFile)
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
		&lastBuildDate
	)

	setPodcast(&p, &fp)

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
		log.Printf("Cannot write to %s: %s", xmlFD.Name(), err)
	}
	log.Printf("Saved %d of %d tracks to %s", validTracks(tracks), len(tracks), defaults.OutputFile)
}
