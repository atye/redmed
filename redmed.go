package redmed

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

const (
	tokenURL = "https://www.reddit.com/api/v1/access_token"
	baseURL  = "https://oauth.reddit.com"
)

var (
	mimeTypes = map[string]string{
		".png":  "image/png",
		".mov":  "video/quicktime",
		".mp4":  "video/mp4",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
	}
)

type Client interface {
	PostImage(ctx context.Context, req PostImageRequest) (string, error)
	PostVideo(ctx context.Context, req PostVideoRequest) (string, error)
	PostGallery(ctx context.Context, req PostGalleryRequest) (string, error)
}

type Option func(*client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *client) {
		c.client = httpClient
	}
}

func New(userAgent, clientID, secret, username, password string, options ...Option) Client {
	c := &client{
		userAgent: userAgent,
		clientID:  clientID,
		secret:    secret,
		client:    http.DefaultClient,
		username:  username,
		password:  password,
	}

	for _, o := range options {
		o(c)
	}
	return c
}

type client struct {
	clientID    string
	secret      string
	username    string
	password    string
	userAgent   string
	client      *http.Client
	accessToken string
}

func (c *client) setToken(ctx context.Context) error {
	form := url.Values{
		"grant_type": []string{"password"},
		"username":   []string{c.username},
		"password":   []string{c.password},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.SetBasicAuth(c.clientID, c.secret)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	type token struct {
		AccessToken string `json:"access_token"`
	}

	var t token
	err = json.NewDecoder(resp.Body).Decode(&t)
	if err != nil {
		return err
	}

	if t.AccessToken == "" {
		return errors.New("no token in response")
	}

	c.accessToken = t.AccessToken
	return nil
}

type PostImageRequest struct {
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

	var err error
	err = c.setToken(ctx)
	if err != nil {
		return "", fmt.Errorf("setting oauth token: %w", err)
	}

	mediaPath := req.Path

	var didDownload bool
	if isValidURL(req.Path) {
		mediaPath, err = c.downloadLink(ctx, req.Path)
		if err != nil {
			return "", fmt.Errorf("downloading %s: %w", req.Path, err)
		}
		didDownload = true
	}

	if didDownload {
		defer os.Remove(mediaPath)
	}

	_, mediaURL, websocketURL, err := c.uploadMedia(ctx, mediaPath)
	if err != nil {
		return "", fmt.Errorf("uploading %s: %w", req.Path, err)
	}

	form := url.Values{
		"kind":        []string{"image"},
		"sr":          []string{req.Subreddit},
		"title":       []string{req.Title},
		"url":         []string{mediaURL},
		"nsfw":        []string{strconv.FormatBool(req.NSWF)},
		"resubmit":    []string{strconv.FormatBool(req.Resubmit)},
		"sendreplies": []string{strconv.FormatBool(req.SendReplies)},
		"spoiler":     []string{strconv.FormatBool(req.Spoiler)},
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/submit", baseURL), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating http request: %w", err)
	}
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.accessToken))

	_, err = c.doRequest(r, "", nil)
	if err != nil {
		return "", fmt.Errorf("executing submission request: %w", err)
	}

	redirect, err := waitForPostSuccess(ctx, websocketURL)
	if err != nil {
		return "", fmt.Errorf("waiting for post success: %w", err)
	}

	split := strings.Split(redirect, "/")

	return fmt.Sprintf("t3%s", split[len(split)-3]), nil
}

type PostVideoRequest struct {
	Kind          string
	NSWF          bool
	Path          string
	Resubmit      bool
	SendReplies   bool
	Spoiler       bool
	Subreddit     string
	ThumbnailPath string
	Title         string
}

func (c *client) PostVideo(ctx context.Context, req PostVideoRequest) (string, error) {
	if req.Kind != "video" && req.Kind != "videogif" {
		return "", fmt.Errorf("kind must be video or videogif")
	}

	if req.ThumbnailPath == "" {
		return "", fmt.Errorf("must provide a local path or link to thumbnail")
	}

	var err error
	err = c.setToken(ctx)
	if err != nil {
		return "", fmt.Errorf("setting oauth token: %w", err)
	}

	mediaPath := req.Path

	var didDownload bool
	if isValidURL(req.Path) {
		mediaPath, err = c.downloadLink(ctx, req.Path)
		if err != nil {
			return "", fmt.Errorf("downloading %s: %w", req.Path, err)
		}
		didDownload = true
	}

	thumbnailPath := req.ThumbnailPath

	var didThumbnailDownload bool
	if isValidURL(req.ThumbnailPath) {
		thumbnailPath, err = c.downloadLink(ctx, req.ThumbnailPath)
		if err != nil {
			return "", fmt.Errorf("downloading %s: %w", req.Path, err)
		}
		didThumbnailDownload = true
	}

	if didDownload {
		defer os.Remove(mediaPath)
	}

	if didThumbnailDownload {
		defer os.Remove(thumbnailPath)
	}

	_, mediaURL, websocketURL, err := c.uploadMedia(ctx, mediaPath)
	if err != nil {
		return "", fmt.Errorf("uploading %s: %w", req.Path, err)
	}

	// verify thumbnail upload?
	_, thumbnailURL, _, err := c.uploadMedia(ctx, thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("uploading %s: %w", req.ThumbnailPath, err)
	}

	form := url.Values{
		"kind":             []string{req.Kind},
		"sr":               []string{req.Subreddit},
		"title":            []string{req.Title},
		"url":              []string{mediaURL},
		"video_poster_url": []string{thumbnailURL},
		"nsfw":             []string{strconv.FormatBool(req.NSWF)},
		"resubmit":         []string{strconv.FormatBool(req.Resubmit)},
		"sendreplies":      []string{strconv.FormatBool(req.SendReplies)},
		"spoiler":          []string{strconv.FormatBool(req.Spoiler)},
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/submit", baseURL), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating http request: %w", err)
	}
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.accessToken))

	_, err = c.doRequest(r, "", nil)
	if err != nil {
		return "", fmt.Errorf("executing submission request: %w", err)
	}

	redirect, err := waitForPostSuccess(ctx, websocketURL)
	if err != nil {
		return "", fmt.Errorf("waiting for post success: %w", err)
	}

	split := strings.Split(redirect, "/")

	return fmt.Sprintf("t3%s", split[len(split)-3]), nil
}

type PostGalleryRequest struct {
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

	var err error
	err = c.setToken(ctx)
	if err != nil {
		return "", fmt.Errorf("setting oauth token: %w", err)
	}

	items := make([]map[string]string, len(req.Paths))

	var eg errgroup.Group
	for i, path := range req.Paths {
		path := path
		index := i
		eg.Go(func() error {
			mediaPath := path
			var didDownload bool
			if isValidURL(path) {
				mediaPath, err = c.downloadLink(ctx, path)
				if err != nil {
					return fmt.Errorf("downloading %s: %w", path, err)
				}
				didDownload = true
			}
			if didDownload {
				defer os.Remove(mediaPath)
			}

			assetID, _, _, err := c.uploadMedia(ctx, mediaPath)
			if err != nil {
				return fmt.Errorf("uploading %s: %w", mediaPath, err)
			}

			items[index] = map[string]string{
				"caption":      "",
				"outbound_url": "",
				"media_id":     assetID,
			}
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return "", err
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

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling payload: %w", err)
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/submit_gallery_post.json", baseURL), bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("creating http request: %w", err)
	}
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.accessToken))

	type postGalleryResponse struct {
		JSON struct {
			Errors []interface{} `json:"errors"`
			Data   struct {
				URL string `json:"url"`
				ID  string `json:"id"`
			} `json:"data"`
		} `json:"json"`
	}

	var pgr postGalleryResponse
	respBody, err := c.doRequest(r, "application/json", &pgr)
	if err != nil {
		return "", fmt.Errorf("executing submission request: %w", err)
	}

	if pgr.JSON.Data.ID == "" {
		return "", fmt.Errorf("executing submission request: %w", fmt.Errorf(string(respBody)))
	}

	return pgr.JSON.Data.ID, nil
}

func waitForPostSuccess(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", nil
	}

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return "", fmt.Errorf("dialing websocket connection: %w", err)
	}
	defer ws.Close()

	type msg struct {
		value string
		err   error
	}

	msgCh := make(chan msg)
	go func(ctx context.Context, msgCh chan msg) {
		defer close(msgCh)

		for {
			if ctx.Err() != nil {
				return
			}
			_, message, err := ws.ReadMessage()
			if err != nil {
				msgCh <- msg{err: fmt.Errorf("reading websocket message: %w", err)}
				return
			}

			type wsResponse struct {
				Type    string `json:"type"`
				Payload struct {
					Redirect string `json:"redirect"`
				} `json:"payload"`
			}

			var wr wsResponse
			err = json.Unmarshal(message, &wr)
			if err != nil {
				msgCh <- msg{err: fmt.Errorf("unmarshalling websocket message: %w", err)}
				return
			}

			if wr.Type == "failed" || wr.Payload.Redirect == "" {
				msgCh <- msg{err: fmt.Errorf("waiting for media upload success: %w", fmt.Errorf(string(message)))}
				return
			}

			msgCh <- msg{value: wr.Payload.Redirect, err: nil}
		}
	}(ctx, msgCh)

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case msg := <-msgCh:
		if msg.err != nil {
			return "", msg.err
		}
		return msg.value, nil
	}
}

func (c *client) uploadMedia(ctx context.Context, path string) (assetID string, mediaURL string, websocketURL string, err error) {
	fileName := filepath.Base(path)
	ext := filepath.Ext(fileName)

	var mimeType string
	if v, ok := mimeTypes[ext]; ok {
		mimeType = v
	} else {
		return "", "", "", fmt.Errorf("%s not supported", ext)
	}

	assetForm := url.Values{
		"filepath": []string{fileName},
		"mimetype": []string{mimeType},
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/media/asset.json", baseURL), strings.NewReader(assetForm.Encode()))
	if err != nil {
		return "", "", "", err
	}
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.accessToken))

	type assetLeaseResponse struct {
		Args struct {
			Action string `json:"action"`
			Fields []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"fields"`
		} `json:"args"`
		Asset struct {
			AssedID      string `json:"asset_id"`
			WebsocketURL string `json:"websocket_url"`
		} `json:"asset"`
	}

	var ar assetLeaseResponse
	_, err = c.doRequest(r, "", &ar)
	if err != nil {
		return "", "", "", err
	}

	uploadURL, err := url.Parse(fmt.Sprintf("https:%s", ar.Args.Action))
	if err != nil {
		return "", "", "", err
	}

	var formBuff bytes.Buffer
	form := multipart.NewWriter(&formBuff)

	for _, field := range ar.Args.Fields {
		formField, err := form.CreateFormField(field.Name)
		if err != nil {
			return "", "", "", err
		}

		_, err = formField.Write([]byte(field.Value))
		if err != nil {
			return "", "", "", err
		}
	}

	formFile, err := form.CreateFormFile("file", fileName)
	if err != nil {
		return "", "", "", err
	}

	mediaFile, err := os.Open(path)
	if err != nil {
		return "", "", "", err
	}
	defer mediaFile.Close()

	_, err = io.Copy(formFile, mediaFile)
	if err != nil {
		return "", "", "", err
	}

	err = form.Close()
	if err != nil {
		return "", "", "", err
	}

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, uploadURL.String(), &formBuff)
	if err != nil {
		return "", "", "", err
	}

	respBody, err := c.doRequest(r, form.FormDataContentType(), nil)
	if err != nil {
		return "", "", "", err
	}

	type postResponse struct {
		Location string `xml:"Location"`
	}

	var pr postResponse
	err = xml.Unmarshal(respBody, &pr)
	if err != nil {
		return "", "", "", err
	}

	return ar.Asset.AssedID, pr.Location, ar.Asset.WebsocketURL, nil
}

func (c *client) downloadLink(ctx context.Context, link string) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("expectes status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	file, err := os.CreateTemp("", fmt.Sprintf("redmed*%s", filepath.Ext(link)))
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return file.Name(), nil
}

func (c *client) doRequest(r *http.Request, contentType string, v interface{}) ([]byte, error) {
	r.Header.Set("User-Agent", c.userAgent)

	cType := "application/x-www-form-urlencoded"
	if contentType != "" {
		cType = contentType
	}

	r.Header.Set("Content-Type", cType)

	resp, err := c.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("status code %d: %s", resp.StatusCode, string(respBytes))
	}

	if v != nil {
		err = json.Unmarshal(respBytes, &v)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling %s: %v", string(respBytes), err)
		}
	}

	return respBytes, nil
}

func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}
