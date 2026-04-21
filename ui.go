package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/term"
)

var (
	uiOut io.Writer = os.Stdout
	uiErr io.Writer = os.Stderr
)

func setUIOutput(stdout, stderr io.Writer) {
	uiOut = stdout
	uiErr = stderr
}

var isTTY = term.IsTerminal(int(os.Stdout.Fd()))

func green(s string) string {
	if !isTTY {
		return s
	}
	return "\x1b[32m" + s + "\x1b[0m"
}

func red(s string) string {
	if !isTTY {
		return s
	}
	return "\x1b[31m" + s + "\x1b[0m"
}

func cyan(s string) string {
	if !isTTY {
		return s
	}
	return "\x1b[36m" + s + "\x1b[0m"
}

func yellow(s string) string {
	if !isTTY {
		return s
	}
	return "\x1b[33m" + s + "\x1b[0m"
}

func printInfo(format string, args ...any) {
	fmt.Fprintf(uiOut, "[INFO] %s\n", fmt.Sprintf(format, args...))
}

func printWarn(format string, args ...any) {
	fmt.Fprintf(uiOut, "[!] %s\n", fmt.Sprintf(format, args...))
}

func printError(format string, args ...any) {
	fmt.Fprintf(uiErr, "[ERR] %s\n", fmt.Sprintf(format, args...))
}

func renderPartition(name string, size uint64) string {
	return fmt.Sprintf("%s (%s)", name, humanizeBytes(size))
}

func humanizeBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func makeBarStyle() mpb.BarFillerBuilder {
	return mpb.BarStyle().
		Lbound("[").Filler("=").Tip(">").Padding("-").Rbound("]")
}

type emptyFillerBuilder struct{}

func (emptyFillerBuilder) Build() mpb.BarFiller {
	return mpb.BarFillerFunc(func(w io.Writer, stat decor.Statistics) error {
		return nil
	})
}

func emptyBarStyle() mpb.BarFillerBuilder {
	return emptyFillerBuilder{}
}

func stateDecorator(state *string) decor.Decorator {
	return decor.Any(func(s decor.Statistics) string {
		st := "RUN"
		if state != nil && *state != "" {
			st = *state
		}
		switch st {
		case "RUN":
			return cyan(st)
		case "DONE":
			return green(st)
		case "FAIL":
			return red(st)
		default:
			return st
		}
	}, decor.WCSyncWidth)
}

func makeBarOptions(name string, total int64, state *string) []mpb.BarOption {
	return []mpb.BarOption{
		mpb.BarWidth(32),
		mpb.PrependDecorators(
			stateDecorator(state),
			decor.Name("  "+name+"  ", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncWidth),
			decor.CountersNoUnit(" (%d/%d)", decor.WCSyncWidth),
		),
	}
}

func summaryDecorator(p *Payload) decor.Decorator {
	return decor.Any(func(s decor.Statistics) string {
		p.stateMu.Lock()
		active := p.activeCount
		done := p.doneCount
		fail := p.failCount
		total := len(p.partitionStates)
		p.stateMu.Unlock()
		pending := total - done - fail
		if pending < 0 {
			pending = 0
		}

		return fmt.Sprintf("ACTIVE %s  FAIL %s  DONE %s  PEND %s  TOTAL",
			green(fmt.Sprintf("%d", active)),
			red(fmt.Sprintf("%d", fail)),
			green(fmt.Sprintf("%d/%d", done, total)),
			yellow(fmt.Sprintf("%d", pending)),
		)
	})
}

// DryRun simulates extraction with animated progress bars, zero IO.
func DryRun(targetDirectory string, partitions []string, speed time.Duration) error {
	printInfo("output dir: %s", targetDirectory)
	printInfo("workers: 4")
	if len(partitions) > 0 {
		printInfo("extracting selected partitions: %s", strings.Join(partitions, ","))
	} else {
		printInfo("extracting all partitions")
	}

	progress := mpb.New(mpb.WithAutoRefresh(), mpb.WithOutput(uiOut))
	var wg sync.WaitGroup

	selected := partitions
	if len(selected) == 0 {
		selected = []string{"abl", "bl1", "bl2", "bl31", "boot", "dtbo", "gsa", "init_boot", "ldfw", "modem", "pbl", "product", "pvmfw", "system", "system_dlkm", "system_ext", "tzsw", "vbmeta", "vbmeta_system", "vbmeta_vendor", "vendor", "vendor_boot", "vendor_dlkm", "vendor_kernel_boot"}
	}

	for _, name := range selected {
		totalOps := 50
		bar := progress.New(int64(totalOps), makeBarStyle(), makeBarOptions(name, int64(totalOps), nil)...)
		wg.Add(1)
		go func(bar *mpb.Bar, ops int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				time.Sleep(speed + time.Duration(i%3)*speed/2)
				bar.Increment()
			}
		}(bar, totalOps)
	}

	wg.Wait()
	progress.Wait()

	printInfo("dry-run complete")
	return nil
}
