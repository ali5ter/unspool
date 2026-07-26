package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ali5ter/unspool/config"
)

func runExport(cfg *config.Config) error {
	switch flagExport {
	case "json", "csv", "markdown":
	default:
		return fmt.Errorf("unknown --export format %q (want json, csv, or markdown)", flagExport)
	}

	result, err := loadFeedResult(cfg)
	if err != nil {
		return err
	}
	videos := toPipelineVideos(result.Items)

	w := io.Writer(os.Stdout)
	if flagOutput != "" {
		f, err := os.Create(flagOutput)
		if err != nil {
			return fmt.Errorf("create export destination %q: %w", flagOutput, err)
		}
		defer f.Close()
		w = f
	}

	switch flagExport {
	case "json":
		return writeJSON(w, videos)
	case "csv":
		return writeCSV(w, videos)
	case "markdown":
		return writeMarkdown(w, videos)
	}
	return nil
}

// writeCSV encodes videos as CSV: a header row followed by one row per video.
func writeCSV(w io.Writer, videos []pipelineVideo) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"video_id", "title", "channel", "published_at", "duration_seconds", "seen"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}
	for _, v := range videos {
		row := []string{v.VideoID, v.Title, v.Channel, v.Published, strconv.Itoa(v.Duration), strconv.FormatBool(v.Seen)}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeMarkdown encodes videos as a Markdown pipe-table. Titles/channels are
// escaped so an embedded "|" can't corrupt the table structure.
func writeMarkdown(w io.Writer, videos []pipelineVideo) error {
	if _, err := fmt.Fprint(w, "| Title | Channel | Published | Duration (s) | Seen |\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "| --- | --- | --- | --- | --- |\n"); err != nil {
		return err
	}
	for _, v := range videos {
		_, err := fmt.Fprintf(w, "| %s | %s | %s | %d | %s |\n",
			escapeMarkdownCell(v.Title), escapeMarkdownCell(v.Channel), v.Published, v.Duration, strconv.FormatBool(v.Seen))
		if err != nil {
			return fmt.Errorf("write markdown row: %w", err)
		}
	}
	return nil
}

func escapeMarkdownCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
