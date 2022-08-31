# redmed

`redmed` is a **red**it API wrapper for posting **med**ia submissions.

## Why?
Existing Reddit API clients, such as [go-reddit](https://github.com/vartanbeno/go-reddit), only support link and self posts. 

`redmed` used alongside existing tools gives you a more complete Reddit API wrapper.

## Usage (see [examples)](https://github.com/atye/redmed/blob/main/examples/main.go)

### Create a client
```
reddit := redmed.New(userAgent, clientID, secret, username, password)
```

With HTTP Client

```
c := &http.Client{Timeout: time.Second * 30}
reddit := redmed.New(userAgent, clientID, secret, username, password, redmed.WithHTTPClient(c))
```

With [gorilla](https://github.com/gorilla/websocket) Websocket Dialer

```
d := websocket.DefaultDialer
d.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
reddit := redmed.New(userAgent, clientID, secret, username, password, redmed.WithWebsocketDialer(d))
```
### Post an image

Supported image types:

 - .png 
 - .jpg 
 - .jpeg 
 - .gif

<details>
    <summary>Post image from local machine</summary>

```go
req := redmed.PostImageRequest{
    NSWF: false,
    Path: "/path/to/image.jpeg",
    Resubmit: true,
    SendReplies: true,
    Spoiler: false,
    Subreddit: "subreddit",
    Title: "image from local path",
}

name, err := reddit.PostImage(context.Background(), req)
if err != nil {
    fmt.Println(err)
}
```
</details>

<details>
    <summary>Post image from link</summary>

```go
req := redmed.PostImageRequest{
    NSWF: false,
    Path: "https://host.com/image.jpeg",
    Resubmit: true,
    SendReplies: true,
    Spoiler: false,
    Subreddit: "subreddit",
    Title: "image from local path",
}

name, err := reddit.PostImage(context.Background(), req)
if err != nil {
    fmt.Println(err)
}
```
</details>

<details>
    <summary>Post image gallery</summary>

```go
req := redmed.PostGalleryRequest{
    NSWF: false,
	Paths: []string{"/path/to/image.jpeg", "https://host.com/image.jpeg"},
	SendReplies: true,
	Spoiler: false,
	Subreddit: "subreddit",
	Title: "gallery from local path and link",
}

name, err := reddit.PostGallery(context.Background(), req)
if err != nil {
    fmt.Println(err)
}
```
</details>

### Post a Video

Supported image types:

 - .mp4
 - .mov
 
<details>
    <summary>Post video from local machine with thumbnail image from link</summary>

```go
req := redmed.PostVideoRequest{
	Kind: "video", // or videogif for silent video
	NSWF: false,
	VideoPath: "/path/to/video.mp4",
	Resubmit: true,
	SendReplies: true,
	Spoiler: false,
	Subreddit: "subreddit",
	Title: "video from local path",
	ThumbnailPath: "https://host.com/image.jpeg",
}

name, err := reddit.PostVideo(context.Background(), req)
if err != nil {
    fmt.Println(err)
}
```
</details>

<details>
    <summary>Post video from link with thumbnail image from local path</summary>

```go
req := redmed.PostVideoRequest{
	Kind: "video", // or videogif for silent video
	NSWF: false,
	VideoPath: "https://host.com/video.mp4",
	Resubmit: true,
	SendReplies: true,
	Spoiler: false,
	Subreddit: "subreddit",
	Title: "video from link",
	ThumbnailPath: "/path/to/image.jpeg",
}

name, err := reddit.PostVideo(context.Background(), req)
if err != nil {
    fmt.Println(err)
}
```
</details>

The `name` returned is the *fullname* of the post, such as `t3_x2dx7f`. 