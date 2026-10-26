package resolver

// The translation layer between the graph's generated model structs and
// the domain entity — the same discipline the REST handlers'
// newPostResponse performs, one graph-shaped edition. Lives OUTSIDE
// posts.resolvers.go on purpose: gqlgen owns that file's shape (and
// quarantines helpers it finds there), while this file is purely
// hand-written.

import (
	"strconv"

	"guide/internal/domain/posts"
	"guide/internal/graph/model"
)

// argInt renders an optional GraphQL Int argument the way the domain's
// list normalizer expects it: nil (argument not sent) becomes "" (the
// default applies), a sent value becomes its decimal string.
func argInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

// argString does the same for an optional GraphQL String argument.
func argString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// modelPost shapes one domain post for the graph — camelCase keys come
// from the generated model's tags; tags are copied so the wire never
// aliases domain state.
func modelPost(post posts.Post) *model.Post {
	return &model.Post{
		ID:        post.ID,
		Title:     post.Title,
		Body:      post.Body,
		Tags:      append([]string(nil), post.Tags...),
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

// modelPage shapes one domain page for the graph, materializing items as
// an empty list — never nil — so an empty page renders [] on the wire.
func modelPage(page posts.PostPage) *model.PostPage {
	items := make([]*model.Post, 0, len(page.Items))
	for _, post := range page.Items {
		items = append(items, modelPost(post))
	}
	return &model.PostPage{
		Items:       items,
		Page:        page.Page,
		Limit:       page.Limit,
		TotalItems:  page.TotalItems,
		TotalPages:  page.TotalPages,
		HasNextPage: page.HasNext,
		HasPrevPage: page.HasPrev,
	}
}
