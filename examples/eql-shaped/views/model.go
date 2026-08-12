// SPDX-License-Identifier: 0BSD

package views

import "gamertan.com/sandwich-hime/sando"

type LayoutView struct {
	SiteName string
	Title    string
	Body     sando.Component
}

type HomeView struct {
	Heading string
	Intro   string
	Browse  BrowseView
}

type BrowseView struct {
	Query   string
	Records []RecordView
}

type RecordView struct {
	URL      string
	Title    string
	Kind     string
	Featured bool
}
