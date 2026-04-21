package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errUsage = errors.New("usage requested")

func extractPayloadBin(filename string) (string, error) {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		return "", fmt.Errorf("not a valid zip archive %s: %w", filename, err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.Reader.File {
		if file.Name == "payload.bin" && file.UncompressedSize64 > 0 {
			zippedFile, err := file.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read zipped file %s: %w", file.Name, err)
			}
			defer zippedFile.Close()

			tempfile, err := os.CreateTemp(os.TempDir(), "payload_*.bin")
			if err != nil {
				return "", fmt.Errorf("failed to create a temp file in %s: %w", os.TempDir(), err)
			}
			defer tempfile.Close()

			_, err = io.Copy(tempfile, zippedFile)
			if err != nil {
				return "", err
			}

			return tempfile.Name(), nil
		}
	}

	return "", nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	setUIOutput(stdout, stderr)
	var (
		list            bool
		partitions      string
		outputDirectory string
		concurrency     int
		dryRun          bool
	)

	fs := flag.NewFlagSet("payload-dumper-go", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&concurrency, "c", 4, "Number of multiple workers to extract (shorthand)")
	fs.IntVar(&concurrency, "concurrency", 4, "Number of multiple workers to extract")
	fs.BoolVar(&list, "l", false, "Show list of partitions in payload.bin (shorthand)")
	fs.BoolVar(&list, "list", false, "Show list of partitions in payload.bin")
	fs.StringVar(&outputDirectory, "o", "", "Set output directory (shorthand)")
	fs.StringVar(&outputDirectory, "output", "", "Set output directory")
	fs.StringVar(&partitions, "p", "", "Dump only selected partitions (comma-separated) (shorthand)")
	fs.StringVar(&partitions, "partitions", "", "Dump only selected partitions (comma-separated)")
	fs.BoolVar(&dryRun, "dry-run", false, "Simulate extraction without writing files")
	if err := fs.Parse(args); err != nil {
		printUsage(stderr, fs)
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	if fs.NArg() == 0 {
		printUsage(stderr, fs)
		return errUsage
	}
	filename := fs.Arg(0)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filename)
	}

	now := time.Now()
	targetDirectory := outputDirectory
	if targetDirectory == "" {
		targetDirectory = fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}

	if dryRun {
		selected := []string{}
		if partitions != "" {
			selected = strings.Split(partitions, ",")
		}
		return DryRun(filepath.Clean(targetDirectory), selected, 20*time.Millisecond)
	}

	payloadBin := filename
	if strings.HasSuffix(filename, ".zip") {
		printWarn("zip input detected, extracting payload.bin first")
		extracted, err := extractPayloadBin(filename)
		if err != nil {
			return err
		}
		payloadBin = extracted
		if payloadBin == "" {
			return errors.New("failed to extract payload.bin from the archive")
		} else {
			defer os.Remove(payloadBin)
		}
	}
	printInfo("input: %s", payloadBin)

	payload := NewPayload(payloadBin)
	if err := payload.Open(); err != nil {
		return err
	}
	if err := payload.Init(); err != nil {
		return err
	}

	if list {
		return nil
	}

	if err := os.MkdirAll(targetDirectory, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDirectory, err)
	}

	if err := payload.SetConcurrency(concurrency); err != nil {
		return err
	}

	printInfo("output dir: %s", targetDirectory)
	printInfo("workers: %d", payload.GetConcurrency())

	if partitions != "" {
		selected := strings.Split(partitions, ",")
		printInfo("extracting selected partitions: %s", partitions)
		if err := payload.ExtractSelected(filepath.Clean(targetDirectory), selected); err != nil {
			return err
		}
	} else {
		printInfo("extracting all partitions")
		if err := payload.ExtractAll(filepath.Clean(targetDirectory)); err != nil {
			return err
		}
	}

	return nil
}

func printUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: %s [options] [inputfile]\n", os.Args[0])
	fs.SetOutput(w)
	fs.PrintDefaults()
}
