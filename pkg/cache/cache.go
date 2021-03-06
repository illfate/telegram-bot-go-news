package cache

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/illfate/telegram-bot-go-news/pkg/config"

	"github.com/gocolly/colly"
)

type Cache struct {
	sync.RWMutex
	postsCache map[string][]Post
	userUrls   map[string][]string
	synonym    config.Synonym
}

type Post struct {
	Link    string
	AddedAt time.Time
}

func New(synonym config.Synonym) *Cache {
	return &Cache{
		synonym:    synonym,
		postsCache: make(map[string][]Post),
		userUrls:   make(map[string][]string),
	}
}

func (cache *Cache) ScrapePosts(url string) {
	c := colly.NewCollector()
	c.OnError(func(r *colly.Response, err error) {
		log.Print("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})
	c.OnXML("/rss/channel/item", func(e *colly.XMLElement) {
		link := e.ChildText("/link")
		idx := strings.Index(link, "?")
		if idx != -1 {
			link = link[:idx]
		}
		for _, category := range e.ChildTexts("//category") {
			category := strings.ToLower(category)
			synonymCategory := cache.synonym.GetCategory(category)
			if !postsHasLink(cache.postsCache[category], link) &&
				!postsHasLink(cache.postsCache[synonymCategory], link) {
				cache.postsCache[category] = append(cache.postsCache[category], Post{
					Link:    link,
					AddedAt: time.Now(),
				})
			}
		}
	})
	err := c.Visit(url)
	if err != nil {
		log.Printf("couldn't scrap: %s", err)
	}
	c.Wait()
}

func (cache *Cache) UpdatePosts(lifeTime time.Duration, url string) {
	cache.Lock()
	cache.ScrapePosts(url)
	cache.deleteOldPosts(lifeTime)
	cache.Unlock()
}

func (cache *Cache) GetLink(category string, userName string) string {
	cache.RLock()
	defer cache.RUnlock()
	lowerCategory := strings.ToLower(category)
	synonymCategory := cache.synonym.GetCategory(lowerCategory)
	return cache.getLink(userName, lowerCategory, synonymCategory)
}

func (cache *Cache) AddUserURL(userName, url string) {
	cache.Lock()
	cache.userUrls[userName] = append(cache.userUrls[userName], url)
	cache.Unlock()
}

func (cache *Cache) UpdateConfig(s config.Synonym) {
	cache.Lock()
	cache.synonym = s
	cache.Unlock()
}

func (cache *Cache) deleteOldPosts(lifeTime time.Duration) {
	timeNow := time.Now()
	for category, posts := range cache.postsCache {
		temp := make([]Post, 0, len(posts))
		for _, post := range posts {
			if timeNow.Sub(post.AddedAt) <= lifeTime {
				temp = append(temp, post)
			}
		}
		cache.postsCache[category] = temp
	}
}

func (cache *Cache) getLink(userName string, categories ...string) string {
	for _, category := range categories {
		link := cache.searchLink(cache.postsCache[category], userName)
		if link != "" {
			return link
		}
	}
	return ""
}

func (cache *Cache) searchLink(posts []Post, userName string) string {
	for _, post := range posts {
		if !cache.userHasLink(userName, post.Link) {
			return post.Link
		}
	}
	return ""
}

func (cache *Cache) userHasLink(userName, userURL string) bool {
	for _, url := range cache.userUrls[userName] {
		if url == userURL {
			return true
		}
	}
	return false
}

func postsHasLink(posts []Post, link string) bool {
	for _, post := range posts {
		if post.Link == link {
			return true
		}
	}
	return false
}
