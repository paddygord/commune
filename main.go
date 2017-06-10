package main

import (
	"encoding/json"
	"github.com/blevesearch/bleve"
	"golang.org/x/crypto/acme/autocert"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"
)

type Post struct {
	Id           uint64
	Title        string
	Snippet      string
	Time         time.Time
	Value        float64
	Username     string
	Html         template.HTML
	CommentCount uint64
	Comments     []Comment
}

type Comment struct {
	Id       uint64
	Time     time.Time
	Value    float64
	Username string
	Html     template.HTML
	Comments []Comment
}

type Page struct {
	Title     template.HTML
	Content   template.HTML
	Freshness uint64
}

type Users struct {
	user_counter uint64
	page_counter uint64
}

type Names struct {
	Animals    []string
	Colours    []string
	Adjectives []string
}

var (
	err   error
	posts []Post
	index [5][]uint64
	users Users
	names Names
)
var text_index bleve.Index

func value(freshness float64, post Post) float64 {
	return float64(post.Value) * math.Pow(0.75, -freshness*float64(post.Time.Unix()))
}

func compare(freshness float64) func(i, j int) bool {
	return func(i, j int) bool {
		return value(freshness, posts[i]) < value(freshness, posts[j])
	}
}

func update_indices() {
	sort.SliceStable(index[0], compare(0.0))
	sort.SliceStable(index[1], compare(0.1))
	sort.SliceStable(index[2], compare(0.2))
	sort.SliceStable(index[3], compare(0.5))
	sort.SliceStable(index[4], compare(1.0))
}

func main() {
	f, err := os.Open("res/posts.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(f).Decode(&posts)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	f, err = os.Open("res/names.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(f).Decode(&names)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	for i := 0; i < len(index); i++ {
		index[i] = make([]uint64, len(posts))
		for j := 0; j < len(posts); j++ {
			index[i][uint64(j)] = uint64(j)
		}
	}
	update_indices()

	text_index, err = bleve.Open("res/search.bleve")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", log_req(hsts(fresh_cookie(home))))
	mux.HandleFunc("/post/", log_req(hsts(fresh_cookie(post))))
	mux.HandleFunc("/search/", log_req(hsts(fresh_cookie(search))))
	mux.HandleFunc("/submit_post", log_req(hsts(user_cookie(submit_post))))
	mux.HandleFunc("/submit_comment", log_req(hsts(user_cookie(submit_comment))))
	mux.Handle("/static/", http.FileServer(http.Dir("./")))

	go func() {
		err = http.Serve(autocert.NewListener("commune.is"), mux)
		if err != nil {
			log.Println(err)
		}
	}()
	close := make(chan os.Signal)
	signal.Notify(close, os.Interrupt, syscall.SIGTERM)
	<-close

	f, err = os.OpenFile("res/posts.json", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(f).Encode(&posts)
	if err != nil {
		log.Println(err)
	}
	f.Close()
}
