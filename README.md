# rss2telegram

Small Go daemon that polls a list of RSS/Atom feeds and forwards new items to a Telegram chat.

## How it works

On each tick it fetches every feed, compares item GUIDs (or links) against `state.json`, and posts anything new to Telegram. On a feed's very first run it sends only the newest item and marks the rest as seen, so adding a feed doesn't flood the chat.

## Build

```sh
go build
```

## Configure

Copy the example and edit:

```sh
cp config.example.yaml config.yaml
```

```yaml
interval: 15m
state_file: ./state.json
max_seen_per_feed: 500
disable_link_preview: false
feeds:
  - title: "Hacker News"
    url: "https://news.ycombinator.com/rss"
```

Set `disable_link_preview: true` to suppress Telegram's link preview cards.

Messages are formatted as `<feed title> <publish date>` on the first line and the linked article title on the second.

Telegram credentials come from the environment:

```sh
export TELEGRAM_BOT_TOKEN=...
export TELEGRAM_CHAT_ID=...
```

## Run

```sh
./rss2telegram -config config.yaml
```

## Docker

The image expects `config.yaml` and `state.json` under `/data`. Set `state_file: /data/state.json` in your config.

```sh
docker build -t rss2telegram .
docker run -d --name rss2telegram \
  -v "$PWD/data:/data" \
  -e TELEGRAM_BOT_TOKEN=... \
  -e TELEGRAM_CHAT_ID=... \
  rss2telegram
```
