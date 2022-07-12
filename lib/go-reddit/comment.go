package reddit

import (
	"fmt"
)

// Comment is a response to a link or another comment.
type Comment struct {
	ApprovedBy          interface{}   `json:"approved_by"`
	Archived            bool          `json:"archived"`
	Author              string        `json:"author"`
	AuthorFlairCSSClass interface{}   `json:"author_flair_css_class"`
	AuthorFlairText     interface{}   `json:"author_flair_text"`
	BannedBy            interface{}   `json:"banned_by"`
	Body                string        `json:"body"`
	BodyHTML            string        `json:"body_html"`
	Controversiality    int           `json:"controversiality"`
	Created             int           `json:"created"`
	CreatedUtc          int           `json:"created_utc"`
	Distinguished       interface{}   `json:"distinguished"`
	Downs               int           `json:"downs"`
	Edited              bool          `json:"edited"`
	Gilded              int           `json:"gilded"`
	ID                  string        `json:"id"`
	Likes               interface{}   `json:"likes"`
	LinkID              string        `json:"link_id"`
	ModReports          []interface{} `json:"mod_reports"`
	Name                string        `json:"name"`
	NumReports          interface{}   `json:"num_reports"`
	ParentID            string        `json:"parent_id"`
	RemovalReason       interface{}   `json:"removal_reason"`
	ReportReasons       interface{}   `json:"report_reasons"`
	Replies             string        `json:"replies"`
	Saved               bool          `json:"saved"`
	Score               int           `json:"score"`
	ScoreHidden         bool          `json:"score_hidden"`
	Stickied            bool          `json:"stickied"`
	Subreddit           string        `json:"subreddit"`
	SubredditID         string        `json:"subreddit_id"`
	Ups                 int           `json:"ups"`
	UserReports         []interface{} `json:"user_reports"`
}

const commentType = "t1"

// DeleteComment deletes a comment submitted by the currently authenticated user. Requires the 'edit' OAuth scope.
func (c *Client) DeleteComment(commentID string) error {
	return c.deleteThing(fmt.Sprintf("%s_%s", commentType, commentID))
}

// EditCommentText edits the text of a comment by the currently authenticated user. Requires the 'edit' OAuth scope.
func (c *Client) EditCommentText(commentID string, text string) error {
	return c.editThingText(fmt.Sprintf("%s_%s", commentType, commentID), text)
}

// GetLinkComments retrieves a listing of comments for the given link.
func (c *Client) GetLinkComments(linkID string) ([]*Comment, error) {
	url := fmt.Sprintf("%s/comments/%s", baseURL, linkID)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return nil, nil
}

// ReplyToComment creates a reply to the given comment. Requires the 'submit' OAuth scope.
func (c *Client) ReplyToComment(commentID string, text string) error {
	return c.commentOnThing(fmt.Sprintf("%s_%s", commentType, commentID), text)
}
