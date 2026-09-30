package model

import "time"

type FeedItem struct {
	Title       string
	URL         string
	PublishedAt time.Time
	Summary     string
	Author      string
	FeedTitle   string
}

type Config struct {
	URLs    []string
	Format  string
	Output  string
	Timeout time.Duration
	Limit   int
}
