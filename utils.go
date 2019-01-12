package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

func basename(filename string) string {
	ext := path.Ext(filename)
	filename = strings.TrimSuffix(filename, ext)
	return filename
}

func bCF47ToISO3(BCF47 string) string {
	lang, err := language.Parse(BCF47)
	switch e := err.(type) {
	case language.ValueError:
		log.Fatalf("Unknown language: %q: culprit %q", lang, e.Subtag())
	case nil:
		// No error.
	default:
		// A syntax error.
		log.Fatalf("Unknown language: %q: ill-formed", lang)
	}
	base, _ := lang.Base()
	return base.ISO3()
}

// From: https://stackoverflow.com/a/21067803

func copyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %q (%s)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %q (%s)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	// if err = os.Link(src, dst); err == nil {
	//    return
	// }
	err = copyFileContents(src, dst)

	if err != nil {
		return
	}

	err = os.Chtimes(dst, sfi.ModTime(), sfi.ModTime())
	if err != nil {
		log.Warnf("Cannot set time for %q: %s", dst, err)
		err = nil
	}

	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func normalizeDirectory(dir string) string {
	if runtime.GOOS != "windows" {
		return dir
	}
	return strings.Replace(dir, `\`, "/", -1)
}

func normalizeFilename(filename string) string {
	// invalid characters in a filename (Windows, etc)
	re := regexp.MustCompile(`[<>"|?*/\\:%]+`)
	return re.ReplaceAllString(filename, "_")
}

func getDurationViaExiftool(filename string, exiftool string) (durationMilliseconds int64, err error) {
	if exiftool == "" {
		return 0, fmt.Errorf("exiftool is not set")
	}

	args := []string{
		"-s",
		"-s",
		"-s",
		"-Duration",
		filename,
	}

	cmd := exec.Command(exiftool, args...)
	cmdline := fmt.Sprintf("%q %s", exiftool, args)
	log.Debugf("Executing: %s", cmdline)
	var bout bytes.Buffer
	var berr bytes.Buffer
	cmd.Stdout = &bout
	cmd.Stderr = &berr
	err = cmd.Run()
	sout := strings.TrimSpace(bout.String())
	serr := strings.TrimSpace(berr.String())
	if sout != "" {
		log.Debugf("stdout=%v", sout)
	}
	if serr != "" {
		log.Debugf("stderr=%v", serr)
	}
	if err != nil {
		return 0, fmt.Errorf("Command failed: %s: %s: %s", cmdline, err, serr)
	}

	re := regexp.MustCompile(`([\d]+):([\d]+):([\d]+)`)
	b := re.FindStringSubmatch(sout)

	if len(b) <= 3 {
		return 0, fmt.Errorf("Command failed: %s: %s: %s", cmdline, "dd:dd:dd not found", sout)
	}
	hours, _ := strconv.Atoi(b[1])
	minutes, _ := strconv.Atoi(b[2])
	seconds, _ := strconv.Atoi(b[3])

	return int64(1000 * ((hours * 3600) + (minutes * 60) + seconds)), nil
}

// see https://superuser.com/questions/650291/how-to-get-video-duration-in-seconds
func getDurationViaFfmpeg(filename string, ffmpeg string) (durationMilliseconds int64, err error) {
	if ffmpeg == "" {
		return 0, fmt.Errorf("ffmpeg is not set")
	}

	args := []string{
		"-i",
		filename,
		"-f",
		"null",
		"-",
		"-y",
	}

	cmd := exec.Command(ffmpeg, args...)
	cmdline := fmt.Sprintf("%q %s", ffmpeg, args)
	log.Debugf("Executing: %s", cmdline)
	var bout bytes.Buffer
	var berr bytes.Buffer
	cmd.Stdout = &bout
	cmd.Stderr = &berr
	err = cmd.Run()
	sout := strings.TrimSpace(bout.String())
	serr := strings.TrimSpace(berr.String())
	if sout != "" {
		log.Debugf("stdout=%v", sout)
	}
	if serr != "" {
		log.Debugf("stderr=%v", serr)
	}
	if err != nil {
		return 0, fmt.Errorf("Command failed: %s: %s: %s", cmdline, err, serr)
	}

	// ffmpeg outputs on stderr
	sout = serr
	re := regexp.MustCompile(` time=([\d]+):([\d]+):([\d]+)`)
	b := re.FindStringSubmatch(sout)

	if len(b) <= 3 {
		return 0, fmt.Errorf("Command failed: %s: %s: %s", cmdline, "time= not found", sout)
	}
	hours, _ := strconv.Atoi(b[1])
	minutes, _ := strconv.Atoi(b[2])
	seconds, _ := strconv.Atoi(b[3])
	hundredths := int(0)

	re = regexp.MustCompile(` time=[\d]+:[\d]+:[\d]+\.([\d]+)`)
	b = re.FindStringSubmatch(sout)

	if len(b) == 2 {
		hundredths, _ = strconv.Atoi(b[1])
	}

	return int64(1000*((hours*3600)+(minutes*60)+seconds) + (hundredths * 10)), nil
}

func getDurationViaFfprobe(filename string, ffprobe string) (durationMilliseconds int64, err error) {
	if ffprobe == "" {
		return 0, fmt.Errorf("ffprobe is not set")
	}

	args := []string{
		"-v",
		"error",
		"-show_entries",
		"format=duration",
		"-of",
		"default=noprint_wrappers=1:nokey=1",
		filename,
	}

	cmd := exec.Command(ffprobe, args...)
	cmdline := fmt.Sprintf("%q %s", ffprobe, args)
	log.Debugf("Executing: %s", cmdline)
	var bout bytes.Buffer
	var berr bytes.Buffer
	cmd.Stdout = &bout
	cmd.Stderr = &berr
	err = cmd.Run()
	sout := strings.TrimSpace(bout.String())
	serr := strings.TrimSpace(berr.String())
	if sout != "" {
		log.Debugf("stdout=%v", sout)
	}
	if serr != "" {
		log.Debugf("stderr=%v", serr)
	}
	if err != nil {
		return 0, fmt.Errorf("Command failed: %s: %s: %s", cmdline, err, serr)
	}

	re := regexp.MustCompile("[^0-9:.]+")

	seconds, err := strconv.ParseFloat(re.ReplaceAllString(sout, ""), 64)
	if err != nil {
		return 0, fmt.Errorf("Command failed: %s: %s: %s", cmdline, "duration not found", sout)
	}
	// round up to the next millisecond
	seconds += 0.000999

	return int64(seconds * 1000), nil
}

/*
func findNewestFile(dir string, mask string) (name string, err error) {
	// inspiration: https://stackoverflow.com/a/45579190
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatalf("Cannot read directory %q: %s", dir, err)
	}
	var modTime time.Time
	var names []string
	for _, fi := range files {
		if mask != "" {
			matched, err := path.Match(mask, fi.Name())
			if err != nil {
				log.Debugf("Match failed on %q for %q", mask, fi.Name())
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
*/

/*
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
*/
