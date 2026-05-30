// cSpell: words charmbracelet lipgloss wrapcheck
package image

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kaweezle/iknite/pkg/utils"
)

type ProgressReader struct {
	startTime time.Time
	lastPrint time.Time
	reader    io.Reader
	out       io.Writer
	name      string
	total     int64
	current   int64
}

func NewProgressReader(reader io.Reader, total int64, name string, out io.Writer) *ProgressReader {
	return &ProgressReader{
		reader:    reader,
		total:     total,
		current:   0,
		startTime: time.Now(),
		out:       out,
		name:      name,
		lastPrint: time.Now(),
	}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	now := time.Now()
	if now.Sub(pr.lastPrint) >= 100*time.Millisecond || err == io.EOF {
		pr.updateProgress()
		pr.lastPrint = now
	}

	return n, err //nolint:wrapcheck // We want to preserve io.EOF for progress updates
}

func (pr *ProgressReader) updateProgress() {
	if pr.total <= 0 {
		return
	}

	elapsed := time.Since(pr.startTime)
	percentage := float64(pr.current) / float64(pr.total) * 100

	var speed float64
	if elapsed.Seconds() > 0 {
		speed = float64(pr.current) / elapsed.Seconds()
	}

	currentSize := utils.FormatBytes(pr.current)
	totalSize := utils.FormatBytes(pr.total)
	speedStr := utils.FormatBytes(int64(speed)) + "/s"

	barWidth := 20
	completed := int(float64(barWidth) * percentage / 100)
	var bar strings.Builder
	for i := range barWidth {
		if i < completed {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}

	bold := lipgloss.NewStyle().Bold(true)
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	progressLine := fmt.Sprintf("\r%s %s %*s/%s (%.1f%%) [%s]",
		bold.Render("Downloading "+pr.name+"..."),
		bar.String(),
		len(totalSize),
		currentSize,
		totalSize,
		percentage,
		cyan.Render(speedStr),
	)

	fmt.Fprint(pr.out, progressLine) //nolint:errcheck // Ignore write errors to progress output

	if pr.current >= pr.total {
		fmt.Fprint(pr.out, "\n") //nolint:errcheck // Ignore write errors to progress output
	}
}
