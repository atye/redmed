package redmed

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/gorilla/websocket"
)

func TestPostImage(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Run("LocalPath", func(t *testing.T) {
			// action server. where the media is actually uploaded to reddit
			actionSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("<PostResponse><Location>https://reddit-uploaded-media.s3-accelerate.amazonaws.com/rte_images%2Fhsklj75xrxk91</Location></PostResponse>"))
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer actionSvr.Close()

			actionServerURL, err := url.Parse(actionSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// websocket server. after the post is submitted, this reddit server tells us when it's ready via websocket
			wsSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				c, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer c.Close()

				resp := wsResponse{}
				resp.Type = "success"
				resp.Payload.Redirect = "https://www.reddit.com/r/subreddit/comments/x1qxro/title/"

				b, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}

				err = c.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer wsSvr.Close()

			wssServerURL, err := url.Parse(wsSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// reddit server. reddit api endpoints
			redditSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/access_token":
					resp := token{AccessToken: "token"}
					b, err := json.Marshal(resp)
					if err != nil {
						t.Fatal(err)
					}
					w.Write(b)
				case "/api/media/asset.json":
					alr := assetLeaseResponse{}
					alr.Args.Fields = []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						{
							"name",
							"value",
						},
					}

					alr.Args.Action = fmt.Sprintf("//127.0.0.1:%s", actionServerURL.Port())
					alr.Asset.AssedID = "123"
					alr.Asset.WebsocketURL = fmt.Sprintf("wss://%s", wssServerURL.Host)

					err = json.NewEncoder(w).Encode(alr)
					if err != nil {
						t.Fatal(err)
					}
				case "/api/submit":
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer redditSvr.Close()

			// save real endpoints
			originalBaseURL, originalTokenURL := baseURL, tokenURL
			defer func() {
				baseURL = originalBaseURL
				tokenURL = originalTokenURL
			}()

			// set endpoints to test servers
			baseURL = redditSvr.URL
			tokenURL = fmt.Sprintf("%s/%s", redditSvr.URL, "api/v1/access_token")

			// Given
			dialer := websocket.DefaultDialer
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}

			reddit := New("userAgent", "clientID", "secret", "username", "password",
				WithHTTPClient(client),
				WithWebsocketDialer(dialer),
			)

			req := PostImageRequest{
				NSWF:        false,
				Path:        "testdata/testimg.jpeg",
				Resubmit:    true,
				SendReplies: true,
				Spoiler:     false,
				Subreddit:   "subreddit",
				Title:       "image test",
			}

			// When
			name, err := reddit.PostImage(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			// Then
			want := "t3_x1qxro"
			if name != want {
				t.Errorf("want %s, got %s", want, name)
			}
		})
		t.Run("Link", func(t *testing.T) {
			// link server. where to download an image to post to reddit
			linkSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/image.jpeg":
					b, err := os.ReadFile("testdata/testimg.jpeg")
					if err != nil {
						t.Fatal(err)
					}
					w.Write(b)
				}
			}))

			// action server. where the media is actually uploaded to reddit
			actionSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("<PostResponse><Location>https://reddit-uploaded-media.s3-accelerate.amazonaws.com/rte_images%2Fhsklj75xrxk91</Location></PostResponse>"))
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer actionSvr.Close()

			actionServerURL, err := url.Parse(actionSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// websocket server. after the post is submitted, this reddit server tells us when it's ready via websocket
			wsSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				c, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer c.Close()

				resp := wsResponse{}
				resp.Type = "success"
				resp.Payload.Redirect = "https://www.reddit.com/r/subreddit/comments/x1qxro/title/"

				b, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}

				err = c.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer wsSvr.Close()

			wssServerURL, err := url.Parse(wsSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// reddit server. reddit api endpoints
			redditSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/access_token":
					resp := token{AccessToken: "token"}
					b, err := json.Marshal(resp)
					if err != nil {
						t.Fatal(err)
					}
					w.Write(b)
				case "/api/media/asset.json":
					alr := assetLeaseResponse{}
					alr.Args.Fields = []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						{
							"name",
							"value",
						},
					}

					alr.Args.Action = fmt.Sprintf("//127.0.0.1:%s", actionServerURL.Port())
					alr.Asset.AssedID = "123"
					alr.Asset.WebsocketURL = fmt.Sprintf("wss://%s", wssServerURL.Host)
					err = json.NewEncoder(w).Encode(alr)
					if err != nil {
						t.Fatal(err)
					}
				case "/api/submit":
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer redditSvr.Close()

			// save real endpoints
			originalBaseURL, originalTokenURL := baseURL, tokenURL
			defer func() {
				baseURL = originalBaseURL
				tokenURL = originalTokenURL
			}()

			// set endpoints to test servers
			baseURL = redditSvr.URL
			tokenURL = fmt.Sprintf("%s/%s", redditSvr.URL, "api/v1/access_token")

			// Given
			dialer := websocket.DefaultDialer
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}}

			reddit := New("userAgent", "clientID", "secret", "username", "password",
				WithHTTPClient(client),
				WithWebsocketDialer(dialer),
			)

			req := PostImageRequest{
				NSWF:        false,
				Path:        fmt.Sprintf("%s/image.jpeg", linkSvr.URL),
				Resubmit:    true,
				SendReplies: true,
				Spoiler:     false,
				Subreddit:   "subreddit",
				Title:       "image test",
			}

			// When
			name, err := reddit.PostImage(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			// Then
			want := "t3_x1qxro"
			if name != want {
				t.Errorf("want %s, got %s", want, name)
			}
		})
	})
}

func TestPostVideo(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Run("LocalPath", func(t *testing.T) {
			// action server. where the media is actually uploaded to reddit
			actionSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("<PostResponse><Location>https://reddit-uploaded-video.s3-accelerate.amazonaws.com/ttcn2fy0nyk91</Location></PostResponse>"))
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer actionSvr.Close()

			actionServerURL, err := url.Parse(actionSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// websocket server. after the post is submitted, this reddit server tells us when it's ready via websocket
			wsSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				c, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer c.Close()

				resp := wsResponse{}
				resp.Type = "success"
				resp.Payload.Redirect = "https://www.reddit.com/r/subreddit/comments/x1qxro/title/"

				b, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}

				err = c.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer wsSvr.Close()

			wssServerURL, err := url.Parse(wsSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// reddit server. reddit api endpoints
			redditSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/access_token":
					resp := token{AccessToken: "token"}
					b, err := json.Marshal(resp)
					if err != nil {
						t.Fatal(err)
					}
					w.Write(b)
				case "/api/media/asset.json":
					alr := assetLeaseResponse{}
					alr.Args.Fields = []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						{
							"name",
							"value",
						},
					}

					alr.Args.Action = fmt.Sprintf("//127.0.0.1:%s", actionServerURL.Port())
					alr.Asset.AssedID = "123"
					alr.Asset.WebsocketURL = fmt.Sprintf("wss://%s", wssServerURL.Host)

					err = json.NewEncoder(w).Encode(alr)
					if err != nil {
						t.Fatal(err)
					}
				case "/api/submit":
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer redditSvr.Close()

			// save real endpoints
			originalBaseURL, originalTokenURL := baseURL, tokenURL
			defer func() {
				baseURL = originalBaseURL
				tokenURL = originalTokenURL
			}()

			// set endpoints to test servers
			baseURL = redditSvr.URL
			tokenURL = fmt.Sprintf("%s/%s", redditSvr.URL, "api/v1/access_token")

			// Given
			dialer := websocket.DefaultDialer
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}

			reddit := New("userAgent", "clientID", "secret", "username", "password",
				WithHTTPClient(client),
				WithWebsocketDialer(dialer),
			)

			req := PostVideoRequest{
				Kind:          "video",
				NSWF:          false,
				VideoPath:     "testdata/video.mp4",
				ThumbnailPath: "testdata/testimg.jpeg",
				Resubmit:      true,
				SendReplies:   true,
				Spoiler:       false,
				Subreddit:     "subreddit",
				Title:         "video test",
			}

			// When
			name, err := reddit.PostVideo(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			// Then
			want := "t3_x1qxro"
			if name != want {
				t.Errorf("want %s, got %s", want, name)
			}
		})
		t.Run("Link", func(t *testing.T) {
			// link server. where to download a video to post to reddit
			linkSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/video.mp4":
					b, err := os.ReadFile("testdata/video.mp4")
					if err != nil {
						t.Fatal(err)
					}
					w.Write(b)
				}
			}))

			// action server. where the media is actually uploaded to reddit
			actionSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("<PostResponse><Location>https://reddit-uploaded-video.s3-accelerate.amazonaws.com/ttcn2fy0nyk91</Location></PostResponse>"))
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer actionSvr.Close()

			actionServerURL, err := url.Parse(actionSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// websocket server. after the post is submitted, this reddit server tells us when it's ready via websocket
			wsSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				c, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer c.Close()

				resp := wsResponse{}
				resp.Type = "success"
				resp.Payload.Redirect = "https://www.reddit.com/r/subreddit/comments/x1qxro/title/"

				b, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}

				err = c.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer wsSvr.Close()

			wssServerURL, err := url.Parse(wsSvr.URL)
			if err != nil {
				t.Fatal(err)
			}

			// reddit server. reddit api endpoints
			redditSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/access_token":
					resp := token{AccessToken: "token"}
					b, err := json.Marshal(resp)
					if err != nil {
						t.Fatal(err)
					}
					w.Write(b)
				case "/api/media/asset.json":
					alr := assetLeaseResponse{}
					alr.Args.Fields = []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					}{
						{
							"name",
							"value",
						},
					}

					alr.Args.Action = fmt.Sprintf("//127.0.0.1:%s", actionServerURL.Port())
					alr.Asset.AssedID = "123"
					alr.Asset.WebsocketURL = fmt.Sprintf("wss://%s", wssServerURL.Host)
					err = json.NewEncoder(w).Encode(alr)
					if err != nil {
						t.Fatal(err)
					}
				case "/api/submit":
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("%s not supported", r.URL.Path)
				}
			}))
			defer redditSvr.Close()

			// save real endpoints
			originalBaseURL, originalTokenURL := baseURL, tokenURL
			defer func() {
				baseURL = originalBaseURL
				tokenURL = originalTokenURL
			}()

			// set endpoints to test servers
			baseURL = redditSvr.URL
			tokenURL = fmt.Sprintf("%s/%s", redditSvr.URL, "api/v1/access_token")

			// Given
			dialer := websocket.DefaultDialer
			dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}}

			reddit := New("userAgent", "clientID", "secret", "username", "password",
				WithHTTPClient(client),
				WithWebsocketDialer(dialer),
			)

			req := PostVideoRequest{
				Kind:          "video",
				NSWF:          false,
				VideoPath:     fmt.Sprintf("%s/video.mp4", linkSvr.URL),
				ThumbnailPath: "testdata/testimg.jpeg",
				Resubmit:      true,
				SendReplies:   true,
				Spoiler:       false,
				Subreddit:     "subreddit",
				Title:         "image test",
			}

			// When
			name, err := reddit.PostVideo(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			// Then
			want := "t3_x1qxro"
			if name != want {
				t.Errorf("want %s, got %s", want, name)
			}
		})
	})
}

func TestPostGallery(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// link server. where to download an image to post to reddit
		linkSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/image.jpeg":
				b, err := os.ReadFile("testdata/image.jpeg")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(b)
			}
		}))

		// action server. where the media is actually uploaded to reddit
		actionSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("<PostResponse><Location>https://reddit-uploaded-media.s3-accelerate.amazonaws.com/rte_images%2Fhsklj75xrxk91</Location></PostResponse>"))
			default:
				t.Fatalf("%s not supported", r.URL.Path)
			}
		}))
		defer actionSvr.Close()

		actionServerURL, err := url.Parse(actionSvr.URL)
		if err != nil {
			t.Fatal(err)
		}

		// reddit server. reddit api endpoints
		redditSvr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/access_token":
				resp := token{AccessToken: "token"}
				b, err := json.Marshal(resp)
				if err != nil {
					t.Fatal(err)
				}
				w.Write(b)
			case "/api/media/asset.json":
				alr := assetLeaseResponse{}
				alr.Args.Fields = []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{
					{
						"name",
						"value",
					},
				}

				alr.Args.Action = fmt.Sprintf("//127.0.0.1:%s", actionServerURL.Port())
				alr.Asset.AssedID = "123"
				err = json.NewEncoder(w).Encode(alr)
				if err != nil {
					t.Fatal(err)
				}
			case "/api/submit_gallery_post.json":
				pgr := postGalleryResponse{}
				pgr.JSON.Data.ID = "t3_x1qxro"
				err = json.NewEncoder(w).Encode(pgr)
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("%s not supported", r.URL.Path)
			}
		}))
		defer redditSvr.Close()

		// save real endpoints
		originalBaseURL, originalTokenURL := baseURL, tokenURL
		defer func() {
			baseURL = originalBaseURL
			tokenURL = originalTokenURL
		}()

		// set endpoints to test servers
		baseURL = redditSvr.URL
		tokenURL = fmt.Sprintf("%s/%s", redditSvr.URL, "api/v1/access_token")

		// Given
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}}

		reddit := New("userAgent", "clientID", "secret", "username", "password",
			WithHTTPClient(client),
		)

		req := PostGalleryRequest{
			NSWF:        false,
			Paths:       []string{fmt.Sprintf("%s/video.mp4", linkSvr.URL), "testdata/testimg.jpeg"},
			SendReplies: true,
			Spoiler:     false,
			Subreddit:   "subreddit",
			Title:       "image test",
		}

		// When
		name, err := reddit.PostGallery(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		// Then
		want := "t3_x1qxro"
		if name != want {
			t.Errorf("want %s, got %s", want, name)
		}
	})
}
