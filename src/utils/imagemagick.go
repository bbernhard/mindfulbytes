package utils

import (
	"os/exec"
	"bytes"
	"time"
	"errors"
	"os"
	"image"
)

type ConvertOptions struct {
	Size string
	Caption string
	Grayscale bool
	Format string
}


type ImageMagickWrapper struct {
	imageMagickPath string
	tempDirectory string
}


func runCommand(command string, args []string) (string, error) {
	cmd := exec.Command(command, args...)

	var errBuffer bytes.Buffer
	var outBuffer bytes.Buffer
	cmd.Stderr = &errBuffer
	cmd.Stdout = &outBuffer

	err := cmd.Start()
	if err != nil {
		return "", err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(60 * time.Second):
		err := cmd.Process.Kill()
		if err != nil {
			return "", err
		}
		return "", errors.New("process killed as timeout reached")
	case err := <-done:
		if err != nil {
			return "", errors.New(errBuffer.String())
		}
	}
	return outBuffer.String(), nil

}

func getImageDimension(imagePath string) (int, int, error) {
    file, err := os.Open(imagePath)
    if err != nil {
        return 0, 0, err
    }

	defer file.Close()

    image, _, err := image.DecodeConfig(file)
    if err != nil {
        return 0, 0, err
    }
    return image.Width, image.Height, nil
}

func NewImageMagickWrapper(imageMagickPath string, tempDirectory string) *ImageMagickWrapper {
	return &ImageMagickWrapper {
		imageMagickPath: imageMagickPath,
		tempDirectory: tempDirectory,
	}
}

func (w *ImageMagickWrapper) Convert(inPath string, outFilename string, convertOptions ConvertOptions) (string, error) {
	outPath := w.tempDirectory + "/" + outFilename

	if convertOptions.Format != "" {
		outPath += "." + convertOptions.Format
	}

	args := []string{}
	args = append(args, inPath)

	if convertOptions.Grayscale {
		args = append(args, []string{"+dither", "-depth", "24", "-colorspace", "Gray"}...)
	}

	if convertOptions.Caption != "" {
		//scale pointsize to half of 1/10 of the image height (-pointsize %[fx:h*(1/10)/2])
		//draw black rectangle with 10% height
		//offset y text position by 1/4 of image height
		args = append(args, []string{"-pointsize", "%[fx:h*(1/10)/2]", "-gravity", "South", "-background", "black", "-fill", 
			"white", "-splice", "0x10%", "-annotate", "+0+%[fx:h*(1/10)/4]", convertOptions.Caption}...)
	}

	if convertOptions.Size != "" {
		args = append(args, "-resize")
		args = append(args, convertOptions.Size)
	}

	args = append(args, outPath)

	_, err := runCommand(w.imageMagickPath, args)
	if err != nil {
		return "", err
	}

	return outPath, nil
}
