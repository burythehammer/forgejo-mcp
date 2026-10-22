// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

// validate methods: each argument struct checks its numbers with the numeric-argument
// module once, before its handler runs (see run in handlers.go). The validated value is
// read through the unexported field, so no handler casts a raw number.

func (a *getIssueArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *createCommentArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *updateIssueArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *replaceBodyLineArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *labelsArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *stateArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *listCommentsArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}

func (a *commentArgs) validate() error {
	var err error
	a.commentID, err = numarg.PositiveID("comment_id", a.CommentID)
	return err
}

func (a *getCommentArgs) validate() error {
	var err error
	a.commentID, err = numarg.PositiveID("comment_id", a.CommentID)
	return err
}

func (a *editCommentArgs) validate() error {
	var err error
	a.commentID, err = numarg.PositiveID("comment_id", a.CommentID)
	return err
}

func (a *editRepoLabelArgs) validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	return err
}

func (a *deleteRepoLabelArgs) validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	return err
}

func (a *getRepoLabelArgs) validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	return err
}

func (a *editOrgLabelArgs) validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	return err
}

func (a *deleteOrgLabelArgs) validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	return err
}

func (a *getOrgLabelArgs) validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	return err
}

func (a *listDependenciesArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	return err
}
