// Simple blog
package blog

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/shurcooL/github_flavored_markdown"
	"github.com/sirupsen/logrus"
)

var (
	fmap  = make(map[string]interface{})
	posts = make([]*Post, 0)
)

type PostMeta struct {
	Author string
	Date   string
	Title  string
	ID     int
}

type Post struct {
	Meta         *PostMeta
	RenderedBody template.HTML
}

func LoadPosts() error {
	files, err := ioutil.ReadDir("posts/")
	if err != nil {
		return errors.WithMessage(err, "readDir")
	}

	loaded := 0
	for _, v := range files {
		name := v.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		split := strings.SplitN(name, "-", 2)
		if len(split) < 2 {
			logrus.Warn("Blog: Failed parsing filename " + name + ", skipping...")
			continue
		}

		parsedID, err := strconv.Atoi(split[0])
		if err != nil {
			logrus.WithError(err).WithField("post", name).Error("Blog: Failed parsing post id")
			continue
		}
		if parsedID >= len(posts) {
			oldPosts := posts
			posts = make([]*Post, parsedID+1)
			copy(posts, oldPosts)
		}

		post, err := readPost("posts/" + name)
		if err != nil {
			logrus.WithError(err).Error("Blog: Failed reading post")
			continue
		}
		post.Meta.ID = parsedID
		posts[parsedID] = post
		loaded++
	}

	logrus.Info("Loaded ", loaded, " blog posts!")

	return nil
}

func readPost(path string) (*Post, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.WithMessage(err, "readPost "+path)
	}

	p, err := parsePost(f)
	return p, errors.WithMessage(err, "readPost "+path)
}

func parsePost(data []byte) (*Post, error) {
	buf := bytes.NewBuffer(data)

	var meta *PostMeta
	dec := json.NewDecoder(buf)
	err := dec.Decode(&meta)
	if err != nil {
		return nil, errors.WithMessage(err, "parsePost")
	}

	rest, err := ioutil.ReadAll(dec.Buffered())
	if err != nil {
		return nil, errors.WithMessage(err, "parsePost")
	}

	if buf.Len() > 0 {
		rest = append(rest, buf.Bytes()...)
	}

	rendered := github_flavored_markdown.Markdown(rest)

	p := &Post{
		Meta:         meta,
		RenderedBody: template.HTML(string(rendered)),
	}

	return p, nil
}

func GetPost(id int) *Post {
	if id >= len(posts) {
		return nil
	}

	return posts[id]
}

func GetPostsNewest(limit, offset int) []*Post {
	result := make([]*Post, 0, limit)

	i := len(posts) - (offset + 1)
	if i >= len(posts) {
		i = len(posts) - 1
	}

	for i >= 0 {
		p := GetPost(i)
		if p != nil {
			result = append(result, p)
			if len(result) >= limit {
				break
			}
		}
		i--
	}

	return result
}
