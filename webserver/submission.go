package main

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
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

func convertToAVIF(inputfile multipart.File, output *os.File) error {
	if _, err := inputfile.Seek(0, io.SeekStart); err != nil {
		return errors.New("error: failed to rewind")
	}

	img, _, err := image.Decode(inputfile)
	if err != nil {
		return errors.New("error: failed to decode")
	}

	err = avif.Encode(output, img, avif.Options{
		Quality:           60,
		QualityAlpha:      60,
		Speed:             8,
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
		return errors.New("Error: failed to create uploads/"), fileloc
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
		return errors.New("Error: uploads over limit, raise limit or clear directory"), fileloc
	}

	// generate unique string name
	// get file type again, now that we trust it, if it doesn't split okay something went wrong (so throw err)

	fileloc = "../uploads/" + uuid.NewString() + ".avif"

	// save to disk, use safe open flags
	dst, err := os.OpenFile(fileloc, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("Error: failed to create file at disk filepath"), fileloc
	}

	// need to rewind the input file (we read the first 512 bytes to get image type)
	err = convertToAVIF(inputfile, dst)
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(fileloc)
		return err, fileloc
	}

	if err := dst.Close(); err != nil {
		_ = os.Remove(fileloc)
		return errors.New("Error: failed closing AVIF file"), fileloc
	}

	return nil, fileloc
}
