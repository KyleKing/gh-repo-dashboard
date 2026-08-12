package models

// PRView is one named pull request search in GitHub's own query syntax. The
// query reaches gh untouched, so anything the search box accepts works, and
// nothing in it is interpreted here.
type PRView struct {
	Name   string
	Search string
}

// defaultPRViews are the views a config file that names none falls back to.
// Each is a question worth asking daily: what is open, what is mine, what is
// waiting on my review, and what is waiting on someone else's.
var defaultPRViews = []PRView{
	{Name: "Open", Search: "is:open sort:updated-desc"},
	{Name: "Mine", Search: "is:open author:@me sort:updated-desc"},
	{Name: "Needs My Review", Search: "is:open review-requested:@me sort:updated-desc"},
	{
		Name:   "Pending Review",
		Search: "is:open draft:false -review:approved -author:app/dependabot sort:updated-desc",
	},
}

var prViews = defaultPRViews

// SetPRViews replaces the named pull request searches. Intended for startup
// config application only; not safe to call concurrently with PRViews.
func SetPRViews(views []PRView) {
	kept := make([]PRView, 0, len(views))
	for _, view := range views {
		if view.Name != "" && view.Search != "" {
			kept = append(kept, view)
		}
	}

	if len(kept) > 0 {
		prViews = kept
	}
}

// PRViews returns the named searches, in the order they are cycled through.
func PRViews() []PRView {
	return prViews
}
