package main

import (
	"context"
	"fmt"

	"github.com/atye/redmed"
)

func main() {
	var (
		userAgent = "ChangeMeClient/0.1 by YourUsername"
		clientID  = "ChangeMeClientID"
		secret    = "ChangeMeSecret"
		username  = "ChangeMeUsername"
		password  = "ChangeMEPassword"

		subreddit = "changeme"
	)

	reddit := redmed.New(userAgent, clientID, secret, username, password)

	// post .png, .jpg, .jpeg, or .gif image from local path
	imgReq := redmed.PostImageRequest{
		NSWF:        false,
		Path:        "/path/to/image.jpeg", // change me
		Resubmit:    true,
		SendReplies: true,
		Spoiler:     false,
		Subreddit:   subreddit,
		Title:       "image from local path",
	}
	name, err := reddit.PostImage(context.Background(), imgReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	// post .png, .jpg, .jpeg, or .gif image from link
	imgReq = redmed.PostImageRequest{
		NSWF:        false,
		Path:        "https://host.com/image.jpeg", // change me
		Resubmit:    true,
		SendReplies: true,
		Spoiler:     false,
		Subreddit:   subreddit,
		Title:       "image from link",
	}

	name, err = reddit.PostImage(context.Background(), imgReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	// post gallery of .png, .jpg, .jpeg, or .gif images from local paths and/or links
	galReq := redmed.PostGalleryRequest{
		NSWF:        false,
		Paths:       []string{"/path/to/image.jpeg", "https://host.com/image.jpeg"}, // change me
		SendReplies: true,
		Spoiler:     false,
		Subreddit:   subreddit,
		Title:       "gallery from local path and link",
	}
	name, err = reddit.PostGallery(context.Background(), galReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	// post .mp4 or .mov video from link
	// must provide image ThumbnailPath (local path or link)
	req := redmed.PostVideoRequest{
		Kind:          "video", // or videogif
		NSWF:          false,
		VideoPath:     "https://host.com/somevideo.mp4", // change me
		Resubmit:      true,
		SendReplies:   true,
		Spoiler:       false,
		Subreddit:     subreddit,
		Title:         "video from link",
		ThumbnailPath: "https://host.com/image.jpeg", // change me
	}

	name, err = reddit.PostVideo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	// post .mp4 or .mov video from local path
	// must provide image ThumbnailPath (local path or link)
	req = redmed.PostVideoRequest{
		Kind:          "video",
		NSWF:          false,
		VideoPath:     "/path/to/video.mp4", // change me
		Resubmit:      true,
		SendReplies:   true,
		Spoiler:       false,
		Subreddit:     subreddit,
		Title:         "video from local path",
		ThumbnailPath: "https://host.com/image.jpeg", // change me
	}

	name, err = reddit.PostVideo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)
}
