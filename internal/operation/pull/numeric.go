// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

// validate methods: each argument struct checks its index and ID with the
// numeric-argument module once, before its handler runs (see run in handlers.go).

func (a *getArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}

func (a *updateArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}

func (a *pagedArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}

func (a *reviewArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return nil
}

func (a *reviewCommentsArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return nil
}

func (a *getReviewArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return nil
}

func (a *mergeArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}

func (a *diffArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}

func (a *createReviewArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}

func (a *submitReviewArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return nil
}

func (a *dismissArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	a.id, err = numarg.PositiveID("id", a.ID)
	if err != nil {
		return err
	}
	return nil
}

func (a *reviewersArgs) validate() error {
	var err error
	a.idx, err = numarg.PositiveID("index", a.Index)
	if err != nil {
		return err
	}
	return nil
}
