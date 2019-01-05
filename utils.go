package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"

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
