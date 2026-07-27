{
	"a": (string, x == "ok" || x == "failed" || startsWith("success")),
	"b": (int, x > 0 && x < 100),
	"c": (int, 0 < x < 100),
	"d": (int, x > 0 && x != 10)
	// "e": (int, (x > 0 && x != 10) || x == -1) -- not yet
}