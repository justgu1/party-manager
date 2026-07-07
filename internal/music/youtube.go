package music

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ExtractVideoID pulls the 11-char video id out of the common YouTube URL
// shapes: watch?v=, youtu.be/, /embed/, /shorts/, /live/.
func ExtractVideoID(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))

	switch {
	case host == "youtu.be":
		id := strings.Trim(u.Path, "/")
		return validID(id)
	case strings.HasSuffix(host, "youtube.com") || host == "youtube-nocookie.com":
		if v := u.Query().Get("v"); v != "" {
			return validID(v)
		}
		// /embed/ID, /shorts/ID, /live/ID
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 2 {
			switch parts[0] {
			case "embed", "shorts", "live", "v":
				return validID(parts[1])
			}
		}
	}
	return "", false
}

func validID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if len(id) < 8 || len(id) > 20 {
		return "", false
	}
	return id, true
}

// OEmbed is the subset of YouTube's oEmbed response we care about.
type OEmbed struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// FetchOEmbed retrieves title/author/thumbnail for a YouTube video without any
// API key, using the public oEmbed endpoint.
func FetchOEmbed(ctx context.Context, client *http.Client, videoURL string) (OEmbed, error) {
	endpoint := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(videoURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return OEmbed{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return OEmbed{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return OEmbed{}, fmt.Errorf("oembed status %d", resp.StatusCode)
	}
	var oe OEmbed
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return OEmbed{}, err
	}
	return oe, nil
}

// SearchResult is one hit from the YouTube Data API search.
type SearchResult struct {
	VideoID      string `json:"video_id"`
	Title        string `json:"title"`
	Channel      string `json:"channel"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type ytSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title        string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
			Thumbnails   struct {
				Medium struct {
					URL string `json:"url"`
				} `json:"medium"`
			} `json:"thumbnails"`
		} `json:"snippet"`
	} `json:"items"`
}

// Search queries the YouTube Data API v3 for videos matching q. Requires an
// API key.
func Search(ctx context.Context, client *http.Client, apiKey, q string) ([]SearchResult, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("busca indisponível: YOUTUBE_API_KEY não configurada")
	}
	params := url.Values{}
	params.Set("part", "snippet")
	params.Set("type", "video")
	params.Set("maxResults", "12")
	params.Set("q", q)
	params.Set("key", apiKey)
	endpoint := "https://www.googleapis.com/youtube/v3/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube search status %d", resp.StatusCode)
	}
	var body ytSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(body.Items))
	for _, it := range body.Items {
		if it.ID.VideoID == "" {
			continue
		}
		out = append(out, SearchResult{
			VideoID:      it.ID.VideoID,
			Title:        it.Snippet.Title,
			Channel:      it.Snippet.ChannelTitle,
			ThumbnailURL: it.Snippet.Thumbnails.Medium.URL,
		})
	}
	return out, nil
}

// canonicalURL returns a stable watch URL for a video id.
func canonicalURL(id string) string {
	return "https://www.youtube.com/watch?v=" + id
}

var httpClient = &http.Client{Timeout: 10 * time.Second}
