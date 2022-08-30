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
	"strings"

	"github.com/gorilla/websocket"
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

type reddit struct {
	clientID    string
	secret      string
	username    string
	password    string
	userAgent   string
	client      *http.Client
	accessToken string
}

func newReddit(userAgent, clientID, secret, username, password string) *reddit {
	return &reddit{
		userAgent: userAgent,
		clientID:  clientID,
		secret:    secret,
		client:    http.DefaultClient,
		username:  username,
		password:  password,
	}
}

func (c *reddit) setHTTPClient(client *http.Client) {
	c.client = client
}

type asset struct {
	ID        string
	Location  string
	WebSocket string
}

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

func (c *reddit) UploadAsset(ctx context.Context, path string) (asset, error) {
	assetPath := path

	var err error
	var didDownload bool
	if isValidURL(path) {
		assetPath, err = downloadLink(ctx, c.client, path)
		if err != nil {
			return asset{}, fmt.Errorf("downloading %s: %w", path, err)
		}
		didDownload = true
	}

	if didDownload {
		defer os.Remove(assetPath)
	}

	fileName := filepath.Base(path)
	ext := filepath.Ext(fileName)

	var mimeType string
	if v, ok := mimeTypes[ext]; ok {
		mimeType = v
	} else {
		return asset{}, fmt.Errorf("%s not supported", ext)
	}

	assetForm := url.Values{
		"filepath": []string{fileName},
		"mimetype": []string{mimeType},
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/media/asset.json", baseURL), strings.NewReader(assetForm.Encode()))
	if err != nil {
		return asset{}, err
	}
	r.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.accessToken))

	var ar assetLeaseResponse
	_, err = c.doRequest(r, "", &ar)
	if err != nil {
		return asset{}, err
	}

	uploadURL, err := url.Parse(fmt.Sprintf("https:%s", ar.Args.Action))
	if err != nil {
		return asset{}, err
	}

	var formBuff bytes.Buffer
	form := multipart.NewWriter(&formBuff)

	for _, field := range ar.Args.Fields {
		formField, err := form.CreateFormField(field.Name)
		if err != nil {
			return asset{}, err
		}

		_, err = formField.Write([]byte(field.Value))
		if err != nil {
			return asset{}, err
		}
	}

	formFile, err := form.CreateFormFile("file", fileName)
	if err != nil {
		return asset{}, err
	}

	mediaFile, err := os.Open(assetPath)
	if err != nil {
		return asset{}, err
	}
	defer mediaFile.Close()

	_, err = io.Copy(formFile, mediaFile)
	if err != nil {
		return asset{}, err
	}

	err = form.Close()
	if err != nil {
		return asset{}, err
	}

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, uploadURL.String(), &formBuff)
	if err != nil {
		return asset{}, err
	}

	respBody, err := c.doRequest(r, form.FormDataContentType(), nil)
	if err != nil {
		return asset{}, err
	}

	type postResponse struct {
		Location string `xml:"Location"`
	}

	var pr postResponse
	err = xml.Unmarshal(respBody, &pr)
	if err != nil {
		return asset{}, err
	}

	return asset{
		ID:        ar.Asset.AssedID,
		Location:  pr.Location,
		WebSocket: ar.Asset.WebsocketURL,
	}, nil
}

func (c *reddit) SubmitPost(ctx context.Context, websocketURL string, body io.Reader) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/submit", baseURL), body)
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

	return fmt.Sprintf("t3_%s", split[len(split)-3]), nil
}

func (c *reddit) SubmitGalleryPost(ctx context.Context, body io.Reader) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/submit_gallery_post.json", baseURL), body)
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

func (c *reddit) SetToken(ctx context.Context) error {
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

func (c *reddit) doRequest(r *http.Request, contentType string, v interface{}) ([]byte, error) {
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

func downloadLink(ctx context.Context, client *http.Client, link string) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(r)
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
