package markdown

import "testing"

func TestParse(t *testing.T) {
	post, err := Parse("---\nslug: my-first-post\ntitle: 第一篇\nsummary: 简介\ncategory: 生活\ncover_image: /images/first.jpg\npublished_at: 2024-04-03\n---\n\n正文里的关键词\n")
	if err != nil {
		t.Fatal(err)
	}
	if post.Slug != "my-first-post" || post.Markdown != "正文里的关键词" {
		t.Fatalf("unexpected article: %#v", post)
	}
	if _, err := Parse("---\nslug: Bad Slug\n---\ntext"); err == nil {
		t.Fatal("expected validation error")
	}
}
