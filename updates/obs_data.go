package updates

import (
	"net/http"
	"net/url"
)

const (
	DownloadIbsLink = "http://download.suse.de/ibs/SUSE:/Maintenance:/"
	baseUrl = "api.suse.de"
)



type Client struct {
	BaseURL    *url.URL
	Username   string
	Password   string
	httpClient *http.Client
}

type Person struct {
	Name string `xml:"name,attr"`
	Role string `xml:"role,attr"` //optional
}
type Grouped struct {
	Id string `xml:"id,attr"`
}
type Acceptinfo struct {
	Rev      string `xml:"rev,attr"`
	Srcmd5   string `xml:"srcmd5,attr"`
	Osrcmd5  string `xml:"osrcmd5,attr"`
	Oproject string `xml:"oproject,attr"` //optional
	Opackage string `xml:"opackage,attr"` //optional
	Xsrcmd5  string `xml:"xsrcmd5,attr"`  //optional
	Oxsrcmd5 string `xml:"oxsrcmd5,attr"` //optional

}
type Options struct {
	Sourceupdate    string `xml:"sourceupdate"`    //optional
	Updatelink      string `xml:"updatelink"`      //optional
	Makeoriginolder string `xml:"makeoriginolder"` //optional
}
type Group struct {
	Name string `xml:"name,attr"`
	Role string `xml:"role,attr"` //optional
}
type Target struct {
	Project        string `xml:"project,attr"`
	Package        string `xml:"package,attr"`        //optional
	Releaseproject string `xml:"releaseproject,attr"` //optional
	Repository     string `xml:"repository,attr"`     //optional
}
type Source struct {
	Project string `xml:"project,attr"`
	Package string `xml:"package,attr"` //optional
	Rev     string `xml:"rev,attr"`     //optional
}
type Action struct {
	Type       string     `xml:"type,attr"`
	Source     Source     `xml:"source"`     //optional
	Target     Target     `xml:"target"`     //optional
	Person     Person     `xml:"person"`     //optional
	Group      Group      `xml:"group"`      //optional
	Grouped    []Grouped  `xml:"grouped"`    //optional-oneOrMore
	Options    Options    `xml:"options"`    //optional
	Acceptinfo Acceptinfo `xml:"acceptinfo"` //optional
}
type State struct {
	Name    string `xml:"name,attr"`
	Who     string `xml:"who,attr"`
	When    string `xml:"when,attr"`
	Comment string `xml:"comment"`
}
type Review struct {
	State      string `xml:"state,attr"`
	By_user    string `xml:"by_user,attr"`
	By_group   string `xml:"by_group,attr"`
	By_project string `xml:"by_project,attr"`
	By_package string `xml:"by_package,attr"`
	Who        string `xml:"who,attr"`
	When       string `xml:"when,attr"`
	Comment    string `xml:"comment"`
}
type History struct {
	Who         string `xml:"who,attr"`
	When        string `xml:"when,attr"`
	Description string `xml:"description"`
	Comment     string `xml:"comment"` //optional
}
type ReleaseRequest struct {
	Id          string    `xml:"id,attr"`      //optional
	Creator     string    `xml:"creator,attr"` //optional
	Actions     []Action  `xml:"action"`       //oneOrMore
	State       State     `xml:"state"`        //optional
	Description string    `xml:"description"`  //optional
	Priority    string    `xml:"priority"`     //optional ref:obs-ratings
	Reviews     []Review  `xml:"review"`       //zeroOrMore
	Histories   []History `xml:"history"`      //zeroOrMore
	Title       string    `xml:"title"`        //optional
	Accept_at   string    `xml:"accept_at"`    //optional
}
type Collection struct {
	Matches         string           `xml:"matches,attr"`
	ReleaseRequests []ReleaseRequest `xml:"request"`
}

type Issue struct {
	Desc    string `xml:"issue"`
	Id      string `xml:"id,attr"`
	Tracker string `xml:"tracker,attr"`
}
type Patchinfo struct {
	Incident    string  `xml:"incident,attr"`
	Issues      []Issue `xml:"issue,attr"`
	Category    string  `xml:"category"`
	Rating      string  `xml:"rating"`
	Packager    string  `xml:"packager"`
	Description string  `xml:"description"`
	Summary     string  `xml:"summary"`
}
