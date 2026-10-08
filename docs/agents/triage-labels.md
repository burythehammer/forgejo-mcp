# Triage Labels

The skills speak in terms of triage roles. This file maps those roles to the label strings used in this repo's Forgejo tracker. Label strings equal the role names.

| Label in mattpocock/skills | Label in our tracker | Meaning                                                  |
| -------------------------- | -------------------- | -------------------------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue                  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information                 |
| `needs-spec`               | `needs-spec`         | Triaged but not yet specced; replaces `needs-triage`     |
| `ready-for-agent`          | `ready-for-agent`    | Specced and ticketed, ready for an AFK agent             |
| `ready-for-human`          | `ready-for-human`    | Specced and ticketed, requires human implementation      |
| `wontfix`                  | `wontfix`            | Will not be actioned                                     |

Plus exactly one priority label per triaged or specced issue: `priority:high`, `priority:medium`, `priority:low`. Judge from impact (data loss/wrong data/crash > degraded behaviour > cleanup) and say why in the triage comment.

Structural labels: `spec` (parent issue owning the branch and PR) and `subtask` (a tracer-bullet slice, landed as commits on the spec's branch).

An issue is only `ready-for-agent` / `ready-for-human` once it has been specced and ticketed. Triage never jumps straight to ready; an unspecced issue gets `needs-spec`.

When a skill mentions a role, use the corresponding label string from this table.
