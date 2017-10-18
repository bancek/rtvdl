package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const indexHTML = `
<html>
<head>
	<title>RTV 4D downloader</title>
	<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1.0, user-scalable=no">
</head>
<body style="font-family: Arial; margin: 20px; color: #333">
	<h1 style="margin: 0 0 20px; font-weight: bold; font-size: 24px">RTV 4D downloader</h1>
	<form method="post" action="/">
		<div style="margin-bottom: 4px"><label for="url" style="font-weight: bold; font-size: 13px">URL:</label></div>
		<div style="margin-bottom: 10px"><input type="url" id="url" name="url" style="font-size: 14px; padding: 8px 10px; width: 100%; border: 1px solid #333; box-shadow: none' border-radius: 0"></div>
		<button type="submit" style="display: block; width: 100%; padding: 20px 0; font-size: 15px; font-weight: bold; background-color: #fff; border: 1px solid #333">Download</button>
	</form>
</body>
</html>
`

const resultHTML = `
<html>
<head>
	<title>{{.title}}</title>
	<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1.0, user-scalable=no">
</head>
<body style="font-family: Arial; margin: 20px; color: #333">
	<h1 style="margin: 0 0 20px; font-weight: bold; font-size: 24px">{{.title}}</h1>

	<p><a href="{{.url}}">{{.url}}</a></p>
</body>
</html>
`

var idRegex = regexp.MustCompile("\\/(\\d{5,})")

type GetRecording struct {
	Response *GetRecordingResponse
}

type GetRecordingResponse struct {
	Title      string
	MediaFiles []*GetRecordingMediaFile
}

type GetRecordingMediaFile struct {
	Height    string
	Width     string
	Streamers map[string]string
	Filename  string
	MediaType string
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	resultTemplate, _ := template.New("result").Parse(resultHTML)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		if r.Method == "GET" {
			fmt.Fprint(w, indexHTML)
		} else if r.Method == "POST" {
			url := r.FormValue("url")

			if !strings.Contains(url, "4d.rtvslo.si/") {
				http.Error(w, "Expected URL to contain 4d.rtvslo.si/", http.StatusBadRequest)
				return
			}

			matches := idRegex.FindAllStringSubmatch(url, 1)

			if len(matches) != 1 {
				http.Error(w, "Invalid URL", http.StatusBadRequest)
				return
			}

			id := matches[0][1]

			apiURL := fmt.Sprintf("http://api.rtvslo.si/ava/getRecording/%s?client_id=19cc0556a5ee31d0d52a0e30b0696b26", id)

			req, err := http.NewRequest("GET", apiURL, nil)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				log.Printf("API expected 200 got %d", res.StatusCode)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
			bodyJSON := &GetRecording{}
			err = json.Unmarshal(body, &bodyJSON)
			if err != nil {
				log.Print(err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
			bestURL := ""
			bestHeight := 0
			for _, mediaFile := range bodyJSON.Response.MediaFiles {
				if mediaFile.MediaType != "MP4" {
					continue
				}
				height, err := strconv.Atoi(mediaFile.Height)
				if err != nil {
					continue
				}
				httpStreamer := mediaFile.Streamers["http"]
				if httpStreamer == "" {
					continue
				}
				if height > bestHeight {
					bestURL = httpStreamer + mediaFile.Filename
					bestHeight = height
				}
			}
			if bestURL == "" {
				log.Print(err)
				http.Error(w, "No HTTP URL found", http.StatusInternalServerError)
				return
			}

			resultTemplate.Execute(w, map[string]interface{}{
				"title": bodyJSON.Response.Title,
				"url":   bestURL,
			})
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
