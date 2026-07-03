module github.com/ilhamster/traceviz/causal_tracing

go 1.24.0

require (
	github.com/hashicorp/golang-lru v0.6.0
	github.com/ilhamster/traceviz/server/go v0.0.0
	github.com/ilhamster/tracey v0.0.0-20260113235238-f00f37f166c1
)

require (
	github.com/google/safehtml v0.1.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.3.8 // indirect
)

replace github.com/ilhamster/traceviz/server/go => ../server/go
