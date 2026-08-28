package main

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/gen2brain/avif"
	"github.com/google/uuid"
)

var allowedTypes = [3]string{
	"image/jpeg",
	"image/png",
	"image/webp",
}

const (
	maxImageWidth  = 10000
	maxImageHeight = 10000
)

func safeCheckContents(inputfile multipart.File) error {

	// first check the actual content is on the whitelist
	// read the first chunk of the request body
	var first512 [512]byte
	n, err := io.ReadFull(inputfile, first512[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return errors.New("Error: file not able to be read")
	}

	// detect and validate the content type
	contentType := http.DetectContentType(first512[:n])
	if !strings.HasPrefix(contentType, "image/") {
		return errors.New("Error: content type not an image")
	}

	// make sure its one of the ones we like
	foundItem := false
	for _, item := range allowedTypes {
		if contentType == item {
			foundItem = true
			break
		}
	}

	if foundItem == false {
		return errors.New("Error: content type not in allowlist")
	}

	// Go to start of file
	if _, err := inputfile.Seek(0, io.SeekStart); err != nil {
		return errors.New("Error: file not able to be rewinded")
	}

	// read only the header for dimensions so we can't explode memory on the server
	cfg, _, err := image.DecodeConfig(inputfile)
	if err != nil {
		return errors.New("Error: image dimensions not readable")
	}
	if cfg.Width > maxImageWidth || cfg.Height > maxImageHeight {
		return errors.New("Error: image dimensions too large")
	}

	return nil

}

func convertToAVIF(data []byte, output *os.File) error {

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("error: failed to decode")
	}

	err = avif.Encode(output, img, avif.Options{
		Quality:           60,
		QualityAlpha:      60,
		Speed:             5,
		ChromaSubsampling: image.YCbCrSubsampleRatio420,
	})

	if err != nil {
		return errors.New("error: failed to encode as AVIF")
	}

	return nil
}
func safeSaveFile(inputfile multipart.File) (error, string) {

	// 20 GB
	// assuming max file size is 2 MB, this is 10,000 files
	// assuming one file every 30 seconds, thats 300,000 seconds (800hrs) needed
	// to exhaust directory. but thats assuming our rate limit code holds
	// but also, that's without any avif compression.
	const maxUploadDirSize int64 = 20 * 1024 * 1024 * 1024

	var fileloc string

	// make dir for uploads if not already
	err := os.Mkdir("../uploads/", 0750)
	if err != nil && !os.IsExist(err) {
		return errors.New("Error: failed to create uploads/"), ""
	}

	// check that upload dir isn't over limit
	var size int64
	// walk the dir, calling a function to count size of sub dirs
	// might not really be needed...? if there are no subdirs? but general purpose i suppose
	err = filepath.Walk("../uploads/", func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})

	if size > maxUploadDirSize {
		return errors.New("Error: uploads over limit, raise limit or clear directory"), ""
	}

	// generate unique string name
	fileloc = "../uploads/" + uuid.NewString() + ".avif"

	if _, err := inputfile.Seek(0, io.SeekStart); err != nil {
		return errors.New("Error: file not able to be rewinded"), ""
	}
	data, err := io.ReadAll(inputfile)
	if err != nil {
		return errors.New("Error: failed to read upload"), ""
	}

	/* encode async so big uploads don't block the request, but write to a
	temp name and rename atomically: the final path only ever exists as a
	complete file, so a mod approving early can never copy a half-written
	image. */
	go func() {
		tmp := fileloc + ".tmp"
		createdFile, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			log.Printf("AVIF: create out: %v", err)
			return
		}
		/* if err, try to remove file */
		if err := convertToAVIF(data, createdFile); err != nil {
			_ = createdFile.Close()
			_ = os.Remove(tmp)
			log.Printf("AVIF: %v", err)
			return
		}
		if err := createdFile.Close(); err != nil {
			_ = os.Remove(tmp)
			log.Printf("AVIF: close: %v", err)
			return
		}
		if err := os.Rename(tmp, fileloc); err != nil {
			_ = os.Remove(tmp)
			log.Printf("AVIF: rename: %v", err)
		}
	}()

	return nil, fileloc
}
