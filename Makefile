all: check/grammar

check/grammar:
	node grammar/check.js grammar.ohm grammar/examples/*
