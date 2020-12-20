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

func findChannel() string {
	channels := getChannels()

	channel := execCMD("fzf -m", func(in io.WriteCloser) {
		for name := range channels {
			fmt.Fprintln(in, name)
		}
	})

	return channels[channel[0]]
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

func findVideo(rss *feed) string {
	links := make(map[string]string, 15)

	link := execCMD("fzf -m", func(in io.WriteCloser) {
		for _, value := range rss.Entry {
			links[value.Title] = value.Link.Href
			fmt.Fprintln(in, value.Title)
		}
	})

	return links[link[0]]
}

func main() {
	// url := "https://www.youtube.com/feeds/videos.xml?channel_id=UCyzGHKIJjYT-H0x_bzP52SQ"
	// url := "https://www.youtube.com/feeds/videos.xml?channel_id=UCrqM0Ym_NbK1fqeQG2VIohg"
	// getFeed(url)

	// getChannels()

	url := getFeed(findChannel())
	fmt.Println(findVideo(url))

}
