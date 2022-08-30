package redmed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

type Client interface {
	PostImage(ctx context.Context, req PostImageRequest) (string, error)
	PostVideo(ctx context.Context, req PostVideoRequest) (string, error)
	PostGallery(ctx context.Context, req PostGalleryRequest) (string, error)
}

type Option func(*client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *client) {
		c.reddit.setHTTPClient(httpClient)
	}
}

type client struct {
	reddit *reddit
}

func New(userAgent, clientID, secret, username, password string, options ...Option) Client {
	c := &client{
		reddit: newReddit(userAgent, clientID, secret, username, password),
	}

	for _, o := range options {
		o(c)
	}
	return c
}

type PostImageRequest struct {
	FlairID     string
	FlairText   string
	NSWF        bool
	Path        string
	Resubmit    bool
	SendReplies bool
	Spoiler     bool
	Subreddit   string
	Title       string
}

func (c *client) PostImage(ctx context.Context, req PostImageRequest) (string, error) {
	if req.Path == "" {
		return "", fmt.Errorf("must proivde a local path or link to image")
	}

	err := c.reddit.SetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("setting oauth token: %w", err)
	}

	asset, err := c.reddit.UploadAsset(ctx, req.Path)
	if err != nil {
		return "", fmt.Errorf("uploading asset: %w", err)
	}

	form := url.Values{}
	form.Add("kind", "image")
	form.Add("sr", req.Subreddit)
	form.Add("title", req.Title)
	form.Add("url", asset.Location)
	form.Add("nsfw", strconv.FormatBool(req.NSWF))
	form.Add("resubmit", strconv.FormatBool(req.Resubmit))
	form.Add("sendreplies", strconv.FormatBool(req.SendReplies))
	form.Add("spoiler", strconv.FormatBool(req.Spoiler))

	if req.FlairID != "" {
		form.Add("flair_id", req.FlairID)
	}

	if req.FlairText != "" {
		form.Add("flair_text", req.FlairText)
	}

	name, err := c.reddit.SubmitPost(ctx, asset.WebSocket, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("submitting post: %w", err)
	}

	return name, nil
}

type PostVideoRequest struct {
	FlairID       string
	FlairText     string
	Kind          string
	NSWF          bool
	VideoPath     string
	Resubmit      bool
	SendReplies   bool
	Spoiler       bool
	Subreddit     string
	ThumbnailPath string
	Title         string
}

func (c *client) PostVideo(ctx context.Context, req PostVideoRequest) (string, error) {
	if req.VideoPath == "" {
		return "", fmt.Errorf("must proivde a local path or link to a video")
	}

	if req.ThumbnailPath == "" {
		return "", fmt.Errorf("must provide a local path or link to thumbnail")
	}

	if req.Kind != "video" && req.Kind != "videogif" {
		return "", fmt.Errorf("kind must be video or videogif")
	}

	err := c.reddit.SetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("setting oauth token: %w", err)
	}

	videoAsset, err := c.reddit.UploadAsset(ctx, req.VideoPath)
	if err != nil {
		return "", fmt.Errorf("uploading video asset: %w", err)
	}

	thumbnailAsset, err := c.reddit.UploadAsset(ctx, req.ThumbnailPath)
	if err != nil {
		return "", fmt.Errorf("uploading thumbnail asset: %w", err)
	}

	form := url.Values{}
	form.Add("kind", req.Kind)
	form.Add("sr", req.Subreddit)
	form.Add("title", req.Title)
	form.Add("url", videoAsset.Location)
	form.Add("video_poster_url", thumbnailAsset.Location)
	form.Add("nsfw", strconv.FormatBool(req.NSWF))
	form.Add("resubmit", strconv.FormatBool(req.Resubmit))
	form.Add("sendreplies", strconv.FormatBool(req.SendReplies))
	form.Add("spoiler", strconv.FormatBool(req.Spoiler))

	if req.FlairID != "" {
		form.Add("flair_id", req.FlairID)
	}

	if req.FlairText != "" {
		form.Add("flair_text", req.FlairText)
	}

	name, err := c.reddit.SubmitPost(ctx, videoAsset.WebSocket, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("submitting post: %w", err)
	}

	return name, nil
}

type PostGalleryRequest struct {
	FlairID     string
	FlairText   string
	NSWF        bool
	Paths       []string
	SendReplies bool
	Spoiler     bool
	Subreddit   string
	Title       string
}

func (c *client) PostGallery(ctx context.Context, req PostGalleryRequest) (string, error) {
	if len(req.Paths) == 0 {
		return "", fmt.Errorf("must proivde local paths or links to images")
	}

	err := c.reddit.SetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("setting oauth token: %w", err)
	}

	items := make([]map[string]string, len(req.Paths))

	var eg errgroup.Group
	for i, path := range req.Paths {
		path := path
		index := i
		eg.Go(func() error {
			asset, err := c.reddit.UploadAsset(ctx, path)
			if err != nil {
				return err
			}

			items[index] = map[string]string{
				"caption":      "",
				"outbound_url": "",
				"media_id":     asset.ID,
			}
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return "", fmt.Errorf("uploading asset: %w", err)
	}

	payload := map[string]interface{}{
		"sr":                 req.Subreddit,
		"title":              req.Title,
		"items":              items,
		"nsfw":               strconv.FormatBool(req.NSWF),
		"sendreplies":        strconv.FormatBool(req.SendReplies),
		"spoiler":            strconv.FormatBool(req.Spoiler),
		"api_type":           "json",
		"show_error_list":    true,
		"validate_on_submit": true,
	}

	if req.FlairID != "" {
		payload["flair_id"] = req.FlairID
	}

	if req.FlairText != "" {
		payload["flair_text"] = req.FlairText
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling payload: %w", err)
	}

	name, err := c.reddit.SubmitGalleryPost(ctx, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("submitting post: %w", err)
	}

	return name, nil
}
