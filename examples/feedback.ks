{
	"id": (string, pk=true),
	"kind": (string, x in [
		"support_ticket",
		"nps",
		"socialmedia_post",
		"complaint",
	]),
	"text": (analyzed, optional=true, len(x) > 0),
	"labels": [(string, x ~ "[a-z0-9:]+")],
	"posted_at": (datetime, parsedate(x, "rfc3339"))
}