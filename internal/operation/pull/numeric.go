// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

// Validate methods: each argument struct checks its required owner and repo, then
// its index and ID with the numeric-argument module, before the credential lookup
// (see pipeline.Handle).

func (a *getArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *updateArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *pagedArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *reviewArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *reviewCommentsArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *getReviewArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *mergeArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *diffArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *createReviewArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *submitReviewArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *dismissArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}

func (a *reviewersArgs) Validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return requiredRepo(a.Owner, a.Repo)
}
