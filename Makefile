all:

test:
	go test -race -shuffle on ./...

test/bench:
	go test -bench=.  -benchmem -memprofile memprofile.out -cpuprofile profile.out ./...

check/grammar:
	node grammar/check.js grammar.ohm grammar/examples/*
