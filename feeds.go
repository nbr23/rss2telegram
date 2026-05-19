package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/mmcdole/gofeed"
)

const interMessageDelay = 1100 * time.Millisecond

type FeedProcessor struct {
	Parser   *gofeed.Parser
	Telegram *TelegramClient
	State    *State
	MaxSeen  int
}

func NewFeedProcessor(tg *TelegramClient, state *State, maxSeen int) *FeedProcessor {
	return &FeedProcessor{
		Parser:   gofeed.NewParser(),
		Telegram: tg,
		State:    state,
		MaxSeen:  maxSeen,
	}
}

func itemID(it *gofeed.Item) string {
	if it.GUID != "" {
		return it.GUID
	}
	return it.Link
}

func (p *FeedProcessor) Process(ctx context.Context, feed FeedConfig) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	parsed, err := p.Parser.ParseURLWithContext(feed.URL, fetchCtx)
	if err != nil {
		return fmt.Errorf("parse %s: %w", feed.URL, err)
	}

	items := make([]*gofeed.Item, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		if itemID(it) == "" {
			continue
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil
	}

	if !p.State.Has(feed.URL) {
		return p.bootstrap(ctx, feed, items)
	}

	seen := p.State.SeenSet(feed.URL)
	var fresh []*gofeed.Item
	for _, it := range items {
		if !seen[itemID(it)] {
			fresh = append(fresh, it)
		}
	}
	if len(fresh) == 0 {
		return nil
	}

	sort.SliceStable(fresh, func(i, j int) bool {
		return publishedBefore(fresh[i], fresh[j])
	})

	for _, it := range fresh {
		if err := ctx.Err(); err != nil {
			return err
		}
		msg := FormatMessage(feed.Title, it.Title, it.Link)
		if err := p.Telegram.SendMessage(ctx, msg); err != nil {
			slog.Error("send failed", "feed", feed.Title, "item", it.Title, "err", err)
			continue
		}
		p.State.Append(feed.URL, []string{itemID(it)}, p.MaxSeen)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interMessageDelay):
		}
	}

	if err := p.State.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func (p *FeedProcessor) bootstrap(ctx context.Context, feed FeedConfig, items []*gofeed.Item) error {
	sorted := make([]*gofeed.Item, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		return publishedBefore(sorted[j], sorted[i]) // descending: newest first
	})

	newest := sorted[0]
	allIDs := make([]string, 0, len(items))
	for _, it := range items {
		allIDs = append(allIDs, itemID(it))
	}
	p.State.Replace(feed.URL, allIDs, p.MaxSeen)

	msg := FormatMessage(feed.Title, newest.Title, newest.Link)
	if err := p.Telegram.SendMessage(ctx, msg); err != nil {
		slog.Error("bootstrap send failed", "feed", feed.Title, "item", newest.Title, "err", err)
	} else {
		slog.Info("bootstrapped feed", "feed", feed.Title, "newest", newest.Title)
	}

	if err := p.State.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func publishedBefore(a, b *gofeed.Item) bool {
	ta := itemTime(a)
	tb := itemTime(b)
	return ta.Before(tb)
}

func itemTime(it *gofeed.Item) time.Time {
	if it.PublishedParsed != nil {
		return *it.PublishedParsed
	}
	if it.UpdatedParsed != nil {
		return *it.UpdatedParsed
	}
	return time.Time{}
}
