package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

// downloadMedia downloads a media attachment to destPath and returns the path
// on success.
func downloadMedia(ctx context.Context, tgc *telegram.Client, media app.Media, destPath string) (string, error) {
	loc, ok := media.Location.(tg.InputFileLocationClass)
	if !ok || loc == nil {
		return "", fmt.Errorf("download: unsupported location type %T", media.Location)
	}
	d := downloader.NewDownloader()
	_, err := d.Download(tgc.API(), loc).ToPath(ctx, destPath)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	return destPath, nil
}
