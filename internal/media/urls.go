// Package media serves local video files and images to the webview.
package media

import (
	"fmt"
	"net/url"
	"strings"
)

// RenderURL is the URL of a render's mp4.
func RenderURL(renderID int64) string { return fmt.Sprintf("/media/render/%d", renderID) }

// FileURL is the URL of a file inside a video's folder (rel uses "/").
func FileURL(videoID int64, rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return fmt.Sprintf("/media/file/%d/%s", videoID, strings.Join(parts, "/"))
}

// ThumbURL is the URL of a video's thumbnail.
func ThumbURL(videoID int64) string { return fmt.Sprintf("/media/thumb/%d", videoID) }
