// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

// Validate methods: each argument struct checks its required owner, repo or org,
// then its numbers with the numeric-argument module, before the credential lookup
// (see pipeline.Handle). The validated value is read through the unexported field,
// so no handler casts a raw number.

func (a *getIssueArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *createCommentArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *updateIssueArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *replaceBodyLineArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *labelsArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *stateArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *listCommentsArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *commentArgs) Validate() error {
	var err error
	a.commentID, err = numarg.PositiveID("comment_id", a.CommentID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *getCommentArgs) Validate() error {
	var err error
	a.commentID, err = numarg.PositiveID("comment_id", a.CommentID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *editCommentArgs) Validate() error {
	var err error
	a.commentID, err = numarg.PositiveID("comment_id", a.CommentID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *editRepoLabelArgs) Validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *deleteRepoLabelArgs) Validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *getRepoLabelArgs) Validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *editOrgLabelArgs) Validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return required("org", a.Org)
}

func (a *deleteOrgLabelArgs) Validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return required("org", a.Org)
}

func (a *getOrgLabelArgs) Validate() error {
	var err error
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return required("org", a.Org)
}

func (a *listDependenciesArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}
