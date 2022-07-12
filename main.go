package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
)

type announcer struct{}

type CliArgs struct {
	DiscordWebhookURL string
	Subreddit         string
}

type WebhookData struct {
	Embeds          []Embed `json:"embeds"`
	DiscordUsername string  `json:"username"`
	DiscordAvatar   string  `json:"avatar_url"`
}
type Embed struct {
	Title       string      `json:"title"`
	URL         string      `json:"url"`
	Color       string      `json:"color"`
	Description string      `json:"description,omitempty"`
	Author      EmbedAuthor `json:"author"`
	Timestamp   string      `json:"timestamp"`
	Image       EmbedImage  `json:"image,omitempty"`
}
type EmbedAuthor struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Avatar string `json:"icon_url"`
}
type EmbedImage struct {
	ImageURL string `json:"url,omitempty"`
}

func grabRemoteJson(url string) ([]byte, error) {
	httpClient := &http.Client{}
	var jsonByte []byte

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not fetch author's about.json: %s", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error in HTTP client: %s", err)
	}

	defer resp.Body.Close()

	jsonByte, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not byterize author's about.json: %s", err)
	}
	return jsonByte, nil
}

func getAuthorAvatar(username string) string {
	defaultAvatarUrl := fmt.Sprintf("https://www.redditstatic.com/avatars/defaults/v2/avatar_default_%d.png", rand.Intn(7))
	authorAboutJson, err := grabRemoteJson(fmt.Sprintf("https://reddit.com/user/%s/about.json", username))
	if err != nil {
		log.Printf("Could not fetch JSON: %s", err)
		return defaultAvatarUrl
	}

	authorAvatar, err := jsonparser.GetString(authorAboutJson, "data", "icon_img")
	if err != nil {
		log.Printf("Failed to parse json: %s", err)
		return defaultAvatarUrl
	}

	authorAvatar = strings.Replace(authorAvatar, "&amp;", "&", -1)

	if authorAvatar != "" {
		return authorAvatar
	} else {
		log.Print("Using default avatar")
		return defaultAvatarUrl
	}
}

func populateWebhook(post *reddit.Post) WebhookData {
	var WebhookContents WebhookData
	var authorAvatar = getAuthorAvatar(post.Author)
	if post.IsSelf {
		WebhookContents = WebhookData{Embeds: []Embed{
			{
				Title:       post.Title,
				URL:         post.URL,
				Color:       "16729344",
				Description: post.SelfText,
				Timestamp:   time.Unix(int64(post.CreatedUTC), 0).Format(time.RFC3339),
				Author: EmbedAuthor{
					Name:   post.Author,
					URL:    fmt.Sprintf("https://reddit.com/user/%s/", post.Author),
					Avatar: authorAvatar,
				},
			}}}
	} else {
		WebhookContents = WebhookData{Embeds: []Embed{
			{
				Title:     post.Title,
				URL:       post.URL,
				Color:     "16729344",
				Timestamp: time.Unix(int64(post.CreatedUTC), 0).Format(time.RFC3339),
				Author: EmbedAuthor{
					Name:   post.Author,
					URL:    fmt.Sprintf("https://reddit.com/user/%s/", post.Author),
					Avatar: authorAvatar,
				},
				Image: EmbedImage{
					ImageURL: post.Thumbnail,
				},
			}}}
	}
	return WebhookContents
}

func parseArgs() CliArgs {
	var args CliArgs

	// Build args and parse them
	flag.StringVar(&args.DiscordWebhookURL, "webhook", "", "Discord Webhook URL")
	flag.StringVar(&args.Subreddit, "subreddit", "", "Subreddit to monitor")
	flag.Parse()

	// Set defaults
	if args.DiscordWebhookURL == "" {
		if os.Getenv("WEBHOOK") == "" {
			log.Fatalf("Webhook not supplied, use --help for info")
		} else {
			args.DiscordWebhookURL = os.Getenv("FOO")
		}
	}

	if args.Subreddit == "" {
		if os.Getenv("SUBREDDIT") == "" {
			log.Fatalf("Subreddit not supplied, use --help for info")
		} else {
			args.Subreddit = os.Getenv("SUBREDDIT")
		}
	}

	return args
}

var args = parseArgs()
var userAgent = "linux:reddit-discord-bridge:0.1.0 (by u/SirTerryW)"

func (a *announcer) Post(post *reddit.Post) error {
	log.Printf("New FullID is: %v, Thumbnail URL is: %s", post.URL, post.Thumbnail)

	// Create json byte data for body
	WebhookJSON, err := json.Marshal(populateWebhook(post))
	if err != nil {
		log.Printf("Error marshalling JSON data: %v", err)
	}

	// POST to Discord
	response, err := http.Post(args.DiscordWebhookURL, "application/json", bytes.NewBuffer(WebhookJSON))
	if err != nil {
		log.Printf("Error in HTTP POST to Discord: %v", err)
	}
	body_byte, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error from Discord after HTTP POST: %v", err)
	}

	// Print POST response
	log.Printf(`%s posted "%s"`, post.Author, post.Title)
	log.Printf("Webhook response: %s", body_byte)
	return nil
}

func main() {
	// Get an api handle to reddit for a logged out (script) program,
	// which forwards this user agent on all requests and issues a request at
	// most every 5 seconds.
	apiHandle, err := reddit.NewScript(userAgent, time.Second*20)

	if err != nil {
		log.Fatalf("Failed to create Script: %v", err)
	}

	// Create a configuration specifying what event sources on Reddit graw
	// should connect to the bot.
	cfg := graw.Config{Subreddits: []string{args.Subreddit}}

	log.Printf("Listening to /r/%s", args.Subreddit)

	// launch a graw scan in a goroutine using the bot, handle, and config. The
	// returned "stop" and "wait" are functions. "stop" will stop the graw run
	// at any time, and "wait" will block until it finishes.
	_, wait, err := graw.Scan(&announcer{}, apiHandle, cfg)

	if err != nil {
		log.Printf("Graw error: %v", err)
	}

	// This time, let's block so the bot will announce (ideally) forever.
	if err := wait(); err != nil {
		log.Printf("graw run encountered an error: %v", err)
	}

}
