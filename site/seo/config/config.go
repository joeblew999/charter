// Package config reads how an app answers search engines from its charter.toml: [seo], the live site and how
// Lighthouse scores it. charter seo finds it by walking up from the working directory. Every setting has a default,
// so the table only says what differs:
//
//	# charter.toml
//	url = "https://app.example.com"      # where the app is deployed: the live site, unless [seo] site names another
//
//	[seo]
//	site = "https://app.example.com"     # the live site; default: url
//	property = "sc-domain:example.com"   # its Search Console property; default the site's Domain property
//
//	[seo.lighthouse]
//	pages = ["/", "/de/board"]           # what charter seo lighthouse scores; default one URL per page of the live sitemap
//	strategy = "mobile"                  # mobile (what Google indexes) or desktop
//	min = 90                             # a category scoring under it fails
//
//	[seo.badges]
//	dir = "docs/badges"                  # where charter seo badges writes its SVG badges
//	readme = "README.md"                 # the file whose <!-- seo scores --> … <!-- /seo scores --> it rewrites
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// File is the file the settings are in, as its [seo].
const File = "charter.toml"

// Config is charter.toml's [seo], with its defaults filled in.
type Config struct {
	Site       string `toml:"site"`
	Property   string `toml:"property"`
	Lighthouse struct {
		Pages    []string `toml:"pages"`
		Strategy string   `toml:"strategy"`
		Min      int      `toml:"min"`
	} `toml:"lighthouse"`
	Badges struct {
		Dir    string `toml:"dir"`
		Readme string `toml:"readme"`
	} `toml:"badges"`
	Path string `toml:"-"` // the file read, "" without one
}

// Find reads [seo] of the nearest charter.toml at or above dir, and fills in the defaults; without one, it is every
// default.
func Find(dir string) (Config, error) {
	var file struct {
		URL string `toml:"url"`
		SEO Config `toml:"seo"`
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return file.SEO, err
	}
	for {
		path := filepath.Join(dir, File)
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, &file); err != nil {
				return file.SEO, err
			}
			file.SEO.Path = path
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	c := file.SEO
	if c.Site == "" {
		c.Site = file.URL
	}
	if c.Lighthouse.Strategy == "" {
		c.Lighthouse.Strategy = "mobile"
	}
	if c.Lighthouse.Min == 0 {
		c.Lighthouse.Min = 90
	}
	if c.Badges.Dir == "" {
		c.Badges.Dir = "docs/badges"
	}
	if c.Badges.Readme == "" {
		c.Badges.Readme = "README.md"
	}
	return c, nil
}
