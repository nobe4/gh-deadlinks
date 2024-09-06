package main

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	gh "github.com/cli/go-gh/v2/pkg/api"
)

type Link struct {
	URL  string
	Text string
	Line int
}

var linkRegex = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
var githubMarkdownURL = regexp.MustCompile(`https://github.com/(.*?)/(.*?)/blob/(.*?)/(.*)`)
var client *gh.RESTClient
var titleCache = map[string][]string{}

func main() {
	var err error
	client, err = gh.DefaultRESTClient()
	if err != nil {
		panic(err)
	}

	filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".md" {
			processFile(path)
		}

		return nil
	})
}

func getGithubFile(owner, repo, branch, path string) (bool, []string) {
	if titles, ok := titleCache[path]; ok {
		return true, titles
	}

	type File struct {
		Content string `json:"content"`
	}
	file := File{}

	restPath := fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path)
	err := client.Get(restPath, &file)

	if err != nil {
		fmt.Printf("error getting file %s: %s\n", restPath, err)
		return false, []string{}
	}

	content, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		panic(err)
	}

	titles := parseTitles(string(content))
	titleCache[restPath] = titles
	return true, titles
}

func parseTitles(content string) []string {
	titles := []string{}

	for _, line := range strings.Split(content, "\n") {
		for _, match := range regexp.MustCompile(`^#+ (.*)$`).FindAllStringSubmatch(line, -1) {
			title := match[1]
			title = strings.ReplaceAll(title, " ", "-")
			title = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(title, "")
			title = strings.ToLower(title)
			titles = append(titles, title)
		}
	}

	return titles
}

func processFile(path string) {
	fmt.Printf("found markdown file: %s\n", path)
	content, err := os.ReadFile(path)

	if err != nil {
		fmt.Printf("error reading file: %s", path)
		return
	}

	links := parseLinks(string(content))
	for _, link := range links {
		for _, match := range githubMarkdownURL.FindAllStringSubmatch(link.URL, -1) {
			owner := match[1]
			repo := match[2]
			branch := match[3]
			path := match[4]
			title := ""

			if strings.Contains(path, "#") {
				parts := strings.Split(path, "#")
				path = parts[0]
				title = parts[1]
			}

			exists, titles := getGithubFile(owner, repo, branch, path)
			if !exists {
				fmt.Printf("file not found: %s\n", path)
			} else {
				if title != "" && !slices.Contains(titles, title) {
					fmt.Printf("title not found: %s\n", title)
				}
			}
		}
	}
}

func parseLinks(content string) []Link {
	links := []Link{}

	for i, line := range strings.Split(content, "\n") {
		for _, match := range linkRegex.FindAllStringSubmatch(line, -1) {
			links = append(links, Link{
				Line: i,
				URL:  match[2],
				Text: match[1],
			})
		}
	}

	return links
}
