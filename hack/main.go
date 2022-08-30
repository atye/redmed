package main

import (
	"context"
	"fmt"

	"github.com/atye/redmed"
)

// "/mnt/c/Users/aty3/video.mp4"
// "/mnt/c/Users/aty3/testimg.jpeg"
// "https://i.imgur.com/E1pzamp.jpeg"

func main() {
	reddit := redmed.New("script:mmafakenews:v0.0.1 (by /u/mmafakenews)", "KR2RECD7RXYLxbwUnKIkWQ", "G2yr6R8ior3LefkwrnX5wGFuolgjfA", "mmafakenews", "4wX_c8ANF@8/!z2")

	req := redmed.PostVideoRequest{
		Kind:          "video",
		NSWF:          false,
		Path:          "https://i.imgur.com/DjkIbsM.mp4",
		Resubmit:      true,
		SendReplies:   true,
		Spoiler:       false,
		Subreddit:     "mmafakenews",
		Title:         "video from link",
		ThumbnailPath: "https://i.imgur.com/E1pzamp.jpeg",
	}

	name, err := reddit.PostVideo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	req = redmed.PostVideoRequest{
		Kind:          "video",
		NSWF:          false,
		Path:          "/mnt/c/Users/aty3/video.mp4",
		Resubmit:      true,
		SendReplies:   true,
		Spoiler:       false,
		Subreddit:     "mmafakenews",
		Title:         "video from local path",
		ThumbnailPath: "https://i.imgur.com/E1pzamp.jpeg",
	}

	name, err = reddit.PostVideo(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	imgReq := redmed.PostImageRequest{
		NSWF:        false,
		Path:        "/mnt/c/Users/aty3/testimg.jpeg",
		Resubmit:    true,
		SendReplies: true,
		Spoiler:     false,
		Subreddit:   "mmafakenews",
		Title:       "image from local path",
	}
	name, err = reddit.PostImage(context.Background(), imgReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	imgReq = redmed.PostImageRequest{
		NSWF:        false,
		Path:        "https://i.imgur.com/E1pzamp.jpeg",
		Resubmit:    true,
		SendReplies: true,
		Spoiler:     false,
		Subreddit:   "mmafakenews",
		Title:       "image from link",
	}
	name, err = reddit.PostImage(context.Background(), imgReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)

	galReq := redmed.PostGalleryRequest{
		NSWF:        false,
		Paths:       []string{"/mnt/c/Users/aty3/testimg.jpeg", "https://i.imgur.com/E1pzamp.jpeg"},
		SendReplies: true,
		Spoiler:     false,
		Subreddit:   "mmafakenews",
		Title:       "galler from local path and link",
	}
	name, err = reddit.PostGallery(context.Background(), galReq)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(name)
}
