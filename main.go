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
	// TODO: make this a more dynamically defined size
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

func fetchVideos(rss *feed) map[string]string {
	links := make(map[string]string, 15)

	for _, value := range rss.Entry {
		links[value.Title] = value.Link.Href
	}

	return links
}

func fetchFeeds() map[string]map[string]string {
	channels := getChannels()
	parsedFeeds := make(map[string]map[string]string, len(channels))

	var wg sync.WaitGroup
	for name := range channels {
		parsedFeeds[name] = fetchVideos(getFeed(channels[name]))

		wg.Add(1)

		go func(name string, wg *sync.WaitGroup) {
			defer wg.Done()
			parsedFeeds[name] = fetchVideos(getFeed(channels[name]))
		}(name, &wg)
	}
	wg.Wait()

	return parsedFeeds
}

func selectChannel(channels map[string]map[string]string) string {
	channel := execCMD("fzf -m", func(in io.WriteCloser) {
		for name := range channels {
			fmt.Fprintln(in, name)
		}
	})

	return channel[0]
}

func selectVideo(feed map[string]string) string {
	link := execCMD("fzf -m", func(in io.WriteCloser) {
		for video := range feed {
			fmt.Fprintln(in, video)
		}
	})

	return feed[link[0]]
}

func main() {
	feeds := fetchFeeds()
	channel := feeds[selectChannel(feeds)]
	video := selectVideo(channel)
	fmt.Println(video)
}
