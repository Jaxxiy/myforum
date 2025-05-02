package business

type Topic struct {
	ID      int    `json:"id"`
	ForumID int    `json:"forum_id"`
	Title   string `json:"title"`
	Desc    string `json:"description"`
}
