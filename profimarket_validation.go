package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var profiColorPattern = regexp.MustCompile(`^#[a-fA-F0-9]{3}([a-fA-F0-9]{3})?$`)

// Accept only HTTP(S) or site-relative resources, never executable URL schemes.
func safeContentURL(value string) bool {
	if value == "" {
		return true
	}
	if strings.ContainsAny(value, "\"'<>\\") {
		return false
	}
	for _, r := range value {
		if r <= 32 || r == 127 {
			return false
		}
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil {
		return false
	}
	if u.IsAbs() {
		return (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != ""
	}
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//")
}

func validProfiVideo(value string) bool {
	if value == "" {
		return true
	}
	if !safeContentURL(value) {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" {
		return false
	}
	switch u.Hostname() {
	case "rutube.ru", "youtube.com", "www.youtube.com", "www.youtube-nocookie.com", "youtu.be", "player.vimeo.com":
		return true
	}
	return false
}

func validateProfiResources(in *profiSolutionInput) error {
	urls := []string{in.CoverImage, in.ExternalURL}
	colors := []string{}
	for _, s := range in.Sections {
		urls = append(urls, s.ImageURL, s.IconImageURL)
		colors = append(colors, s.NumberingColor)
	}
	for _, features := range [][]profiFeature{in.AccessFeatures, in.AIFeatures, in.HowItWorks, in.KeyMetrics, in.Bonuses} {
		for _, f := range features {
			colors = append(colors, f.TextColor, f.BackgroundColor)
		}
	}
	for _, m := range in.Media {
		if m.Type != "IMAGE" && m.Type != "VIDEO" {
			return errors.New("некорректный тип медиа")
		}
		if m.Type == "VIDEO" && !validProfiVideo(m.URL) {
			return errors.New("укажите HTTPS-ссылку поддерживаемого видеосервиса")
		}
		urls = append(urls, m.URL)
	}
	if v, ok := in.ProductData["video_url"].(string); ok {
		urls = append(urls, v)
	}
	for _, value := range urls {
		if !safeContentURL(value) {
			return errors.New("некорректная ссылка на ресурс")
		}
	}
	for _, value := range colors {
		if value != "" && !profiColorPattern.MatchString(value) {
			return errors.New("цвет должен быть в формате #RGB или #RRGGBB")
		}
	}
	return nil
}
