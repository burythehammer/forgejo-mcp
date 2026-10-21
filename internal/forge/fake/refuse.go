// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import "sync"

// refusalState holds the refusals Refuse sets, keyed by port method name.
type refusalState struct {
	mu   sync.Mutex
	byOp map[string]refusal
}

type refusal struct {
	status  int
	message string
}

// Refuse makes the named port method answer as Forgejo does when it declines
// the call: with status and Forgejo's message, the body a real Forgejo sends.
// op is the method's name on the port (for example "DeleteFile"). A status of 0
// clears the refusal. A refused method fails before it touches any state.
func (f *Forgejo) Refuse(op string, status int, message string) {
	f.refusals.mu.Lock()
	defer f.refusals.mu.Unlock()
	if status == 0 {
		delete(f.refusals.byOp, op)
		return
	}
	if f.refusals.byOp == nil {
		f.refusals.byOp = map[string]refusal{}
	}
	f.refusals.byOp[op] = refusal{status: status, message: message}
}

// refused is the failure op is set to answer with, or nil when it is not.
func (f *Forgejo) refused(op string) error {
	f.refusals.mu.Lock()
	defer f.refusals.mu.Unlock()
	r, ok := f.refusals.byOp[op]
	if !ok {
		return nil
	}
	return statusErr(r.status, r.message)
}
