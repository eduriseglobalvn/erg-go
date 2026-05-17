package bot

import (
	botadapters "erg.ninja/internal/modules/bot/infrastructure/adapters"
	"erg.ninja/internal/modules/crawler"
	"erg.ninja/internal/modules/trending"
)

type (
	CrawlerAdapter  = botadapters.CrawlerAdapter
	TrendingAdapter = botadapters.TrendingAdapter
)

func NewCrawlerAdapter(svc *crawler.Service) *CrawlerAdapter {
	return botadapters.NewCrawlerAdapter(svc)
}

func NewTrendingAdapter(svc *trending.Service) *TrendingAdapter {
	return botadapters.NewTrendingAdapter(svc)
}
