package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

func execCMD(command string, input func(in io.WriteCloser)) []string {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}

	cmd := exec.Command(shell, "-c", command)

	cmd.Stderr = os.Stderr
	in, _ := cmd.StdinPipe()
	go func() {
		input(in)
		in.Close()
	}()

	result, _ := cmd.Output()
	return strings.Split(string(result), "\n")
}

// Channels becomes a parsed xml feed
type Channels struct {
	channels map[string]map[int]map[string]string
}

type feed struct {
	Entry []struct {
		Text      string `xml:",chardata"`
		ID        string `xml:"id"`
		VideoID   string `xml:"videoId"`
		ChannelID string `xml:"channelId"`
		Title     string `xml:"title"`
		Link      struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func getChannels() map[string]string {
	// TODO(#1): make this a more dynamically defined size
	channels := make(map[string]string, 100)

	path := os.Getenv("TUBES")
	file, _ := os.Open(path)

	fscanner := bufio.NewScanner(file)
	for fscanner.Scan() {
		channel := strings.Split(fscanner.Text(), ",")
		channels[channel[0]] = channel[1]
	}

	return channels
}

func getFeed(url string) *feed {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	html, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	rss := &feed{}
	_ = xml.Unmarshal([]byte(html), &rss)

	return rss
}

func fetchVideos(rss *feed) map[int]map[string]string {
	links := make(map[int]map[string]string, 15)

	for index, value := range rss.Entry {
		links[index+1] = map[string]string{"title": value.Title, "link": value.Link.Href}
	}

	return links
}

func fetchFeeds() *Channels {
	channels := getChannels()
	parsedFeeds := &Channels{
		channels: make(map[string]map[int]map[string]string),
	}

	var wg sync.WaitGroup
	for name := range channels {
		wg.Add(1)

		go func(name string, wg *sync.WaitGroup) {
			defer wg.Done()
			parsedFeeds.channels[name] = fetchVideos(getFeed(channels[name]))
		}(name, &wg)
	}
	wg.Wait()

	return parsedFeeds
}

func exitOnNull(selection string, output []string) {
	if len(output) == 1 {
		fmt.Printf("Invalid %v selection\n", selection)
		os.Exit(1)
	}
}

func selectChannel(channel *Channels) string {
	selectedChannel := execCMD("fzf", func(in io.WriteCloser) {
		for name := range channel.channels {
			fmt.Fprintln(in, name)
		}
	})

	exitOnNull("channel", selectedChannel)

	return selectedChannel[0]
}

// TODO(#3): enable the option to select multiple videos
func selectVideo(feed map[int]map[string]string) string {
	link := execCMD("fzf", func(in io.WriteCloser) {
		for index := 1; index < len(feed); index++ {
			fmt.Fprintln(in, index, feed[index]["title"])
		}
	})

	exitOnNull("video", link)

	video, _ := strconv.Atoi(string(link[0][0]))
	return feed[video]["link"]
}

func main() {
	feeds := fetchFeeds()
	channel := feeds.channels[selectChannel(feeds)]
	video := selectVideo(channel)
	fmt.Println(video)
}
