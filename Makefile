all: check/grammar

check/grammar:
	node grammar/check.js grammar.ohm examples/*
